// control serves a phone-sized page for running the home setup: for now one
// Shutdown button, which powers off the hosts ticked on the page, in their
// CONTROL_HOSTS order, over SSH (as root, with a key it makes itself). The
// server's own host goes last in the list, as powering it off ends this
// service too.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

//go:embed index.html
var indexHTML []byte

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	addr := envOr("HTTP_ADDR", ":8093")
	sshDir := envOr("SSH_DIR", "/ssh")
	hosts := parseHosts(os.Getenv("CONTROL_HOSTS"))
	if len(hosts) == 0 {
		log.Printf("CONTROL_HOSTS is empty: Shutdown will have nothing to do")
	}

	key, err := ensureKey(sshDir)
	if err != nil {
		log.Fatalf("ssh key: %v", err)
	}
	c := &controller{key: key, knownHosts: filepath.Join(sshDir, "known_hosts"), dryRun: os.Getenv("DRY_RUN") == "1"}
	c.reset(hosts)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	mux.HandleFunc("GET /rpc/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, c.status())
	})
	mux.HandleFunc("POST /rpc/shutdown", func(w http.ResponseWriter, r *http.Request) {
		// The body lists the ticked hosts by their place in CONTROL_HOSTS.
		var req struct {
			Hosts []int `json:"hosts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("shutdown of hosts %v requested from %s", req.Hosts, r.RemoteAddr)
		c.shutdown(hosts, req.Hosts)
		writeJSON(w, c.status())
	})

	log.Printf("control %s listening on %s, hosts %v, dry run %v", version, addr, hosts, c.dryRun)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// host is one machine the page can power off: Name is what the page calls
// it, Target the user@address ssh logs in to.
type host struct{ Name, Target string }

// parseHosts reads CONTROL_HOSTS: comma-separated user@address, each
// optionally named as "Name=user@address". An unnamed host is called by its
// target.
func parseHosts(s string) []host {
	var hosts []host
	for _, h := range strings.Split(s, ",") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		name, target, named := strings.Cut(h, "=")
		if !named {
			name, target = h, h
		}
		hosts = append(hosts, host{Name: strings.TrimSpace(name), Target: strings.TrimSpace(target)})
	}
	return hosts
}

// ensureKey makes the SSH key on first start and logs its public half, to go
// in /root/.ssh/authorized_keys on every host in CONTROL_HOSTS.
func ensureKey(dir string) (string, error) {
	key := filepath.Join(dir, "id_ed25519")
	if _, err := os.Stat(key); os.IsNotExist(err) {
		out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "control@homeserver", "-f", key).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("ssh-keygen: %v: %s", err, out)
		}
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		return "", err
	}
	log.Printf("public key, for /root/.ssh/authorized_keys on each host: %s", strings.TrimSpace(string(pub)))
	return key, nil
}

// Host states, as the page shows them.
const (
	stateWaiting  = "waiting"
	stateStopping = "shutting down"
	stateOff      = "off"
	stateFailed   = "failed"
	stateSkipped  = "skipped" // not ticked
)

type hostStatus struct {
	Name  string `json:"name"`
	Host  string `json:"host"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

type status struct {
	Running bool         `json:"running"`
	DryRun  bool         `json:"dryRun"`
	Hosts   []hostStatus `json:"hosts"`
}

type controller struct {
	key, knownHosts string
	dryRun          bool

	mu      sync.Mutex
	running bool
	hosts   []hostStatus
}

func (c *controller) reset(hosts []host) {
	c.hosts = make([]hostStatus, len(hosts))
	for i, h := range hosts {
		c.hosts[i] = hostStatus{Name: h.Name, Host: h.Target, State: stateWaiting}
	}
}

func (c *controller) status() status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return status{Running: c.running, DryRun: c.dryRun, Hosts: append([]hostStatus(nil), c.hosts...)}
}

func (c *controller) set(i int, state, errText string) {
	c.mu.Lock()
	c.hosts[i].State, c.hosts[i].Error = state, errText
	c.mu.Unlock()
	log.Printf("%s (%s): %s %s", c.hosts[i].Name, c.hosts[i].Host, state, errText)
}

// shutdown starts powering off the selected hosts (by index) in their
// order, unless that's already under way; the rest are marked skipped. A
// host that fails is passed over, so one dead display box can't keep the
// server up.
func (c *controller) shutdown(hosts []host, selected []int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return
	}
	c.running = true
	c.reset(hosts)
	chosen := make([]bool, len(hosts))
	for _, i := range selected {
		if i >= 0 && i < len(hosts) {
			chosen[i] = true
		}
	}
	for i := range hosts {
		if !chosen[i] {
			c.hosts[i].State = stateSkipped
		}
	}
	go func() {
		for i, h := range hosts {
			if !chosen[i] {
				continue
			}
			c.set(i, stateStopping, "")
			if err := c.powerOff(h.Target); err != nil {
				c.set(i, stateFailed, err.Error())
			} else {
				c.set(i, stateOff, "")
			}
		}
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()
}

// powerOff runs poweroff on host over SSH, then waits for its SSH port to
// stop answering. poweroff often drops the connection before ssh can report
// success, so ssh's own result is only used if the host stays up.
func (c *controller) powerOff(host string) error {
	cmd := "poweroff"
	if c.dryRun {
		cmd = "true"
	}
	out, sshErr := exec.Command("ssh",
		"-i", c.key,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile="+c.knownHosts,
		host, cmd).CombinedOutput()
	if c.dryRun {
		if sshErr != nil {
			return fmt.Errorf("%v: %s", sshErr, strings.TrimSpace(string(out)))
		}
		return nil
	}

	addr := host[strings.LastIndex(host, "@")+1:]
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(addr, "22"), 2*time.Second)
		if err != nil {
			return nil // gone
		}
		conn.Close()
	}
	if sshErr != nil {
		return fmt.Errorf("still up: %v: %s", sshErr, strings.TrimSpace(string(out)))
	}
	return fmt.Errorf("still up after 90s")
}

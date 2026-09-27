package main

import (
	"log"
	"net/url"
	"sync"
	"time"

	"homeserver/internal/dlna"
)

// nameRetry is how long to wait before asking a device for its name again
// after failing to find one.
const nameRetry = 5 * time.Minute

type nameEntry struct {
	name    string
	checked time.Time
}

// deviceNames caches UPnP friendly names by "scheme://host:port", so the
// renderer's and each media server's description is fetched once rather
// than on every poll.
type deviceNames struct {
	mu    sync.Mutex
	names map[string]nameEntry
}

func newDeviceNames() *deviceNames {
	return &deviceNames{names: make(map[string]nameEntry)}
}

// lookup returns the friendly name of the device serving rawURL, or its
// host if it has no UPnP description (e.g. a plain internet radio stream),
// or "" if rawURL isn't an http(s) URL.
func (d *deviceNames) lookup(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	base := u.Scheme + "://" + u.Host

	d.mu.Lock()
	e, ok := d.names[base]
	d.mu.Unlock()
	if ok && (e.name != "" || time.Since(e.checked) < nameRetry) {
		return orHost(e.name, u)
	}

	name, err := dlna.FriendlyName(base)
	if err != nil {
		log.Printf("names: %v", err)
	}
	d.mu.Lock()
	d.names[base] = nameEntry{name: name, checked: time.Now()}
	d.mu.Unlock()
	return orHost(name, u)
}

func orHost(name string, u *url.URL) string {
	if name != "" {
		return name
	}
	return u.Hostname()
}

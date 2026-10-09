package main

import (
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"homeserver/internal/dlna"
)

// nameRetry is how long to wait before asking a device for its name again
// after failing to find one.
const nameRetry = 5 * time.Minute

type nameEntry struct {
	device  dlna.Device
	checked time.Time
}

// deviceNames caches UPnP device descriptions by "scheme://host:port", so
// the renderer's and each media server's description is fetched once rather
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
// or "" if rawURL isn't an http(s) URL. detail is its model and version,
// e.g. "Plex Media Server v1.43.4.10903", or "" if it has no description.
func (d *deviceNames) lookup(rawURL string) (name, detail string) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", ""
	}
	base := u.Scheme + "://" + u.Host

	d.mu.Lock()
	e, ok := d.names[base]
	d.mu.Unlock()
	if !ok || (e.device.FriendlyName == "" && time.Since(e.checked) >= nameRetry) {
		dev, err := dlna.Describe(base)
		if err != nil {
			log.Printf("names: %v", err)
		}
		e = nameEntry{device: dev, checked: time.Now()}
		d.mu.Lock()
		d.names[base] = e
		d.mu.Unlock()
	}
	return orDefault(e.device.FriendlyName, u.Hostname()), modelDetail(e.device)
}

// modelDetail describes a device by model, adding the model number as a
// version when it looks like one ("1.43.4.10903", not a model code "N602").
func modelDetail(dev dlna.Device) string {
	if dev.ModelName == "" {
		return ""
	}
	if strings.Contains(dev.ModelNumber, ".") {
		return dev.ModelName + " v" + dev.ModelNumber
	}
	return dev.ModelName
}

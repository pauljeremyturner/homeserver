package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// wttr.in locates us by IP but doesn't say what time zone that is, so the
// time zone for its coordinates comes from timeapi.io (free, no key). It's
// only looked up again when the coordinates change.
var tzCache struct {
	sync.Mutex
	coords, name string
}

// timezoneAt returns the IANA time zone name at lat/lon, or "" if the
// lookup fails.
func timezoneAt(lat, lon string) string {
	if lat == "" || lon == "" {
		return ""
	}
	coords := lat + "," + lon
	tzCache.Lock()
	defer tzCache.Unlock()
	if tzCache.coords == coords {
		return tzCache.name
	}
	name, err := lookupTimezone(lat, lon)
	if err != nil {
		log.Printf("timezone lookup for %s: %v", coords, err)
		return ""
	}
	tzCache.coords, tzCache.name = coords, name
	return name
}

func lookupTimezone(lat, lon string) (string, error) {
	client := http.Client{Timeout: 15 * time.Second}
	q := url.Values{"latitude": {lat}, "longitude": {lon}}
	resp, err := client.Get("https://timeapi.io/api/timezone/coordinate?" + q.Encode())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("timeapi.io: %s", resp.Status)
	}
	var r struct {
		TimeZone string `json:"timeZone"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if _, err := time.LoadLocation(r.TimeZone); err != nil {
		return "", fmt.Errorf("timeapi.io: unknown time zone %q", r.TimeZone)
	}
	return r.TimeZone, nil
}

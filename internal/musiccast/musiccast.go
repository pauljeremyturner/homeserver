// Package musiccast implements a minimal client for Yamaha's MusicCast
// ("Yamaha Extended Control") HTTP API: enough to read what a receiver's
// network inputs are playing and to pause or resume it.
//
// A MusicCast receiver playing from its own inputs (Server, Net Radio,
// Spotify, ...) leaves its UPnP AVTransport at NO_MEDIA_PRESENT, so this API
// is the only place that playback shows up.
package musiccast

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to one receiver, at base ("http://host").
type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

// get calls a /YamahaExtendedControl/v1/ endpoint and decodes its JSON into v,
// failing on a non-zero response_code (the API answers errors with HTTP 200).
func (c *Client) get(path string, v any) error {
	resp, err := c.http.Get(c.base + "/YamahaExtendedControl/v1/" + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("musiccast %s: unexpected status %d", path, resp.StatusCode)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("musiccast %s: %w", path, err)
	}
	var rc struct {
		ResponseCode int `json:"response_code"`
	}
	if err := json.Unmarshal(raw, &rc); err != nil {
		return fmt.Errorf("musiccast %s: %w", path, err)
	}
	if rc.ResponseCode != 0 {
		return fmt.Errorf("musiccast %s: response_code %d", path, rc.ResponseCode)
	}
	if v == nil {
		return nil
	}
	return json.Unmarshal(raw, v)
}

// Status is the main zone's power and selected input.
type Status struct {
	Power string `json:"power"` // "on" or "standby"
	Input string `json:"input"` // e.g. "server", "net_radio", "tuner"
}

func (c *Client) Status() (Status, error) {
	var s Status
	err := c.get("main/getStatus", &s)
	return s, err
}

// PlayInfo is what the network/USB inputs are playing.
type PlayInfo struct {
	Input       string `json:"input"`
	Playback    string `json:"playback"` // "play", "pause", "stop", "fast_forward", "fast_reverse"
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	Track       string `json:"track"`
	AlbumArtURL string `json:"albumart_url"` // absolute once returned by PlayInfo
	PlayTime    int    `json:"play_time"`    // seconds
	TotalTime   int    `json:"total_time"`   // seconds, 0 if unknown
}

func (c *Client) PlayInfo() (PlayInfo, error) {
	var p PlayInfo
	if err := c.get("netusb/getPlayInfo", &p); err != nil {
		return PlayInfo{}, err
	}
	// The receiver gives art as a path on itself.
	if strings.HasPrefix(p.AlbumArtURL, "/") {
		p.AlbumArtURL = c.base + p.AlbumArtURL
	}
	return p, nil
}

// Names holds the receiver's display names: the main zone's (which is the
// receiver's network name, e.g. "Salon") and each input's, by input id.
type Names struct {
	Zone   string
	Inputs map[string]string
}

func (c *Client) Names() (Names, error) {
	var r struct {
		ZoneList []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"zone_list"`
		InputList []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"input_list"`
	}
	if err := c.get("system/getNameText", &r); err != nil {
		return Names{}, err
	}
	n := Names{Inputs: make(map[string]string)}
	for _, z := range r.ZoneList {
		if z.ID == "main" {
			n.Zone = z.Text
		}
	}
	for _, in := range r.InputList {
		n.Inputs[in.ID] = in.Text
	}
	return n, nil
}

// SetPlayback sends a playback command to the network/USB inputs: "play",
// "pause" or "stop" (among others).
func (c *Client) SetPlayback(playback string) error {
	return c.get("netusb/setPlayback?playback="+url.QueryEscape(playback), nil)
}

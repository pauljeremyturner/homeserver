package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"homeserver/internal/dlna"
	"homeserver/internal/musiccast"
)

// renderer is the device nowplaying shows and controls: a UPnP AVTransport
// that something else sends tracks to, or a Yamaha MusicCast receiver playing
// from its own inputs.
type renderer interface {
	// poll reads what's playing. State uses AVTransport's values
	// ("PLAYING", "PAUSED_PLAYBACK", "STOPPED", ...) whichever the device.
	poll() (nowPlaying, error)
	// toggle pauses the device if it's playing, otherwise resumes it, and
	// returns the new state.
	toggle() (string, error)
	describe() string
}

type dlnaRenderer struct {
	controlURL  string
	serviceType string
	art         *artCache
	names       *deviceNames
}

func (d *dlnaRenderer) describe() string { return d.controlURL }

func (d *dlnaRenderer) poll() (nowPlaying, error) {
	state, err := dlna.GetTransportState(d.controlURL, d.serviceType)
	if err != nil {
		return nowPlaying{}, fmt.Errorf("GetTransportInfo failed: %w", err)
	}

	np := nowPlaying{State: state}
	if state == "PLAYING" || state == "PAUSED_PLAYBACK" {
		pos, err := dlna.GetPositionInfo(d.controlURL, d.serviceType)
		if err != nil {
			return nowPlaying{}, fmt.Errorf("GetPositionInfo failed: %w", err)
		}
		np.Title = pos.Title
		np.Artist = pos.Artist
		np.Album = pos.Album
		np.TrackNumber = pos.TrackNumber
		np.AlbumArtURL = d.art.resolve(pos.AlbumArtURL, pos.Album, pos.Artist)
		np.Duration = pos.Duration
		np.RelTime = pos.RelTime
		np.Source = d.names.lookup(pos.TrackURI)
		np.Renderer = d.names.lookup(d.controlURL)
	}
	return np, nil
}

// toggle decides from the renderer's live state rather than the last poll.
func (d *dlnaRenderer) toggle() (string, error) {
	state, err := dlna.GetTransportState(d.controlURL, d.serviceType)
	if err != nil {
		return "", err
	}
	if state == "PLAYING" {
		if err := dlna.Pause(d.controlURL, d.serviceType); err != nil {
			return "", err
		}
		return "PAUSED_PLAYBACK", nil
	}
	if err := dlna.Play(d.controlURL, d.serviceType); err != nil {
		return "", err
	}
	return "PLAYING", nil
}

type musiccastRenderer struct {
	baseURL string
	client  *musiccast.Client
	art     *artCache

	mu           sync.Mutex
	names        musiccast.Names
	namesChecked time.Time
}

func newMusiccastRenderer(baseURL string, art *artCache) *musiccastRenderer {
	return &musiccastRenderer{baseURL: baseURL, client: musiccast.New(baseURL), art: art}
}

func (m *musiccastRenderer) describe() string { return m.baseURL + " (MusicCast)" }

// playbackStates maps MusicCast playback values to AVTransport states.
var playbackStates = map[string]string{
	"play":         "PLAYING",
	"pause":        "PAUSED_PLAYBACK",
	"stop":         "STOPPED",
	"fast_forward": "PLAYING",
	"fast_reverse": "PLAYING",
}

func (m *musiccastRenderer) poll() (nowPlaying, error) {
	status, err := m.client.Status()
	if err != nil {
		return nowPlaying{}, err
	}
	// In standby, or on an input that isn't network/USB (tuner, phono,
	// optical, ...), getPlayInfo describes nothing that's audible.
	if status.Power != "on" {
		return nowPlaying{State: "STOPPED"}, nil
	}
	info, err := m.client.PlayInfo()
	if err != nil {
		return nowPlaying{}, err
	}
	if info.Input != status.Input {
		return nowPlaying{State: "STOPPED"}, nil
	}

	state, ok := playbackStates[info.Playback]
	if !ok {
		state = "STOPPED"
	}
	np := nowPlaying{State: state}
	if state == "PLAYING" || state == "PAUSED_PLAYBACK" {
		names := m.lookupNames()
		np.Title = info.Track
		np.Artist = info.Artist
		np.Album = info.Album
		np.AlbumArtURL = m.art.resolve(info.AlbumArtURL, info.Album, info.Artist)
		if info.TotalTime > 0 {
			np.Duration = hms(info.TotalTime)
		}
		np.RelTime = hms(info.PlayTime)
		np.Source = orDefault(names.Inputs[info.Input], info.Input)
		np.Renderer = names.Zone
	}
	return np, nil
}

// lookupNames returns the receiver's display names, fetched once and retried
// after nameRetry if that failed.
func (m *musiccastRenderer) lookupNames() musiccast.Names {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.names.Inputs != nil || time.Since(m.namesChecked) < nameRetry {
		return m.names
	}
	names, err := m.client.Names()
	m.namesChecked = time.Now()
	if err != nil {
		log.Printf("names: %v", err)
		return m.names
	}
	m.names = names
	return names
}

// toggle decides from the receiver's live state rather than the last poll.
func (m *musiccastRenderer) toggle() (string, error) {
	info, err := m.client.PlayInfo()
	if err != nil {
		return "", err
	}
	if info.Playback == "play" {
		if err := m.client.SetPlayback("pause"); err != nil {
			return "", err
		}
		return "PAUSED_PLAYBACK", nil
	}
	if err := m.client.SetPlayback("play"); err != nil {
		return "", err
	}
	return "PLAYING", nil
}

// hms formats seconds the way AVTransport does, e.g. "0:05:22".
func hms(seconds int) string {
	return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
}

func orDefault(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

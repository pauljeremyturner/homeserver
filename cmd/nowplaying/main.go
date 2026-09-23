package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"homeserver/internal/dlna"
)

//go:embed web
var webFiles embed.FS

type nowPlaying struct {
	State       string `json:"state"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	TrackNumber string `json:"track_number"`
	AlbumArtURL string `json:"album_art_url"`
	Duration    string `json:"duration"`
	RelTime     string `json:"rel_time"`
	UpdatedAt   string `json:"updated_at"`
	Stale       bool   `json:"stale"`
}

type store struct {
	mu    sync.RWMutex
	state nowPlaying
}

func (s *store) get() nowPlaying {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *store) set(np nowPlaying) {
	s.mu.Lock()
	defer s.mu.Unlock()
	np.UpdatedAt = time.Now().Format(time.RFC3339)
	s.state = np
}

// setState updates just the transport state, so a play/pause shows on the
// display straight away instead of on the next poll.
func (s *store) setState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.State = state
}

func (s *store) markStale() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Stale = true
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	controlURL := os.Getenv("RENDERER_CONTROL_URL")
	if controlURL == "" {
		log.Fatal("RENDERER_CONTROL_URL is required")
	}
	serviceType := envOr("RENDERER_SERVICE_TYPE", "urn:schemas-upnp-org:service:AVTransport:1")
	pollSeconds, err := strconv.Atoi(envOr("POLL_INTERVAL_SECONDS", "3"))
	if err != nil || pollSeconds <= 0 {
		pollSeconds = 3
	}
	addr := envOr("LISTEN_ADDR", ":8090")

	s := &store{}
	art := newArtCache(os.Getenv("PLEX_URL"), os.Getenv("PLEX_TOKEN"))

	go pollLoop(s, art, controlURL, serviceType, time.Duration(pollSeconds)*time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/now-playing", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.get())
	})
	mux.HandleFunc("/api/art", art.serveHTTP)
	mux.HandleFunc("/api/toggle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		state, err := togglePlayback(controlURL, serviceType)
		if err != nil {
			log.Printf("toggle: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		s.setState(state)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"state": state})
	})

	webRoot, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatalf("setting up embedded web root: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webRoot)))

	log.Printf("nowplaying listening on %s, polling %s every %ds", addr, controlURL, pollSeconds)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// togglePlayback pauses the renderer if it's playing, otherwise resumes it,
// deciding from the renderer's live state rather than the last poll.
func togglePlayback(controlURL, serviceType string) (string, error) {
	state, err := dlna.GetTransportState(controlURL, serviceType)
	if err != nil {
		return "", err
	}
	if state == "PLAYING" {
		if err := dlna.Pause(controlURL, serviceType); err != nil {
			return "", err
		}
		return "PAUSED_PLAYBACK", nil
	}
	if err := dlna.Play(controlURL, serviceType); err != nil {
		return "", err
	}
	return "PLAYING", nil
}

func pollLoop(s *store, art *artCache, controlURL, serviceType string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	poll := func() {
		state, err := dlna.GetTransportState(controlURL, serviceType)
		if err != nil {
			log.Printf("poll: GetTransportInfo failed: %v", err)
			s.markStale()
			return
		}

		np := nowPlaying{State: state}
		if state == "PLAYING" || state == "PAUSED_PLAYBACK" {
			pos, err := dlna.GetPositionInfo(controlURL, serviceType)
			if err != nil {
				log.Printf("poll: GetPositionInfo failed: %v", err)
				s.markStale()
				return
			}
			np.Title = pos.Title
			np.Artist = pos.Artist
			np.Album = pos.Album
			np.TrackNumber = pos.TrackNumber
			np.AlbumArtURL = art.resolve(pos.AlbumArtURL, pos.Album, pos.Artist)
			np.Duration = pos.Duration
			np.RelTime = pos.RelTime
		}
		s.set(np)
	}

	poll()
	for range ticker.C {
		poll()
	}
}

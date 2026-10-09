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
	Source      string `json:"source"`   // media server the track comes from
	Renderer    string `json:"renderer"` // device playing it
	// Model and version of each, e.g. "Plex Media Server v1.43.4.10903".
	SourceDetail   string `json:"source_detail"`
	RendererDetail string `json:"renderer_detail"`
	UpdatedAt      string `json:"updated_at"`
	Stale          bool   `json:"stale"`
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
	pollSeconds, err := strconv.Atoi(envOr("POLL_INTERVAL_SECONDS", "3"))
	if err != nil || pollSeconds <= 0 {
		pollSeconds = 3
	}
	addr := envOr("LISTEN_ADDR", ":8090")

	s := &store{}
	art := newArtCache(os.Getenv("PLEX_URL"), os.Getenv("PLEX_TOKEN"))

	// A Yamaha MusicCast receiver playing from its own inputs never shows
	// it on AVTransport, so when its address is given, poll its own API.
	var r renderer
	if musiccastURL := os.Getenv("RENDERER_MUSICCAST_URL"); musiccastURL != "" {
		r = newMusiccastRenderer(musiccastURL, art, newDeviceNames())
	} else {
		controlURL := os.Getenv("RENDERER_CONTROL_URL")
		if controlURL == "" {
			log.Fatal("RENDERER_MUSICCAST_URL or RENDERER_CONTROL_URL is required")
		}
		r = &dlnaRenderer{
			controlURL:  controlURL,
			serviceType: envOr("RENDERER_SERVICE_TYPE", "urn:schemas-upnp-org:service:AVTransport:1"),
			art:         art,
			names:       newDeviceNames(),
		}
	}

	go pollLoop(s, r, time.Duration(pollSeconds)*time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/now-playing", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.get())
	})
	mux.HandleFunc("/api/art", art.serveHTTP)
	mux.HandleFunc("/api/toggle", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		state, err := r.toggle()
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

	log.Printf("nowplaying listening on %s, polling %s every %ds", addr, r.describe(), pollSeconds)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func pollLoop(s *store, r renderer, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	poll := func() {
		np, err := r.poll()
		if err != nil {
			log.Printf("poll: %v", err)
			s.markStale()
			return
		}
		s.set(np)
	}

	poll()
	for range ticker.C {
		poll()
	}
}

package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// artCache fetches album art server-side and serves it from /api/art, so the
// display never loads art straight from the renderer's URL. Plex's DLNA art
// links (/proxy/<id>/albumart.jpg) embed a token that goes stale across Plex
// restarts, and a renderer keeps handing out the stale link until the album is
// re-browsed — so when that URL fails, fall back to looking the album up via
// the Plex API with our own token.
type artCache struct {
	plexURL   string // e.g. http://192.168.1.148:32400; empty disables the fallback
	plexToken string
	client    *http.Client

	mu          sync.RWMutex
	key         string // album art URL + album the cached image belongs to
	image       []byte
	contentType string
	failedAt    time.Time
}

const artRetryInterval = 30 * time.Second

func newArtCache(plexURL, plexToken string) *artCache {
	return &artCache{
		plexURL:   strings.TrimRight(plexURL, "/"),
		plexToken: plexToken,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// resolve makes sure art for the given track is cached and returns the URL
// the display should load it from, or "" if no art could be found.
func (c *artCache) resolve(artURL, album, artist string) string {
	if artURL == "" && album == "" {
		return ""
	}
	key := artURL + "\x00" + album

	c.mu.RLock()
	cached := c.key == key
	hasImage := len(c.image) > 0
	failedAt := c.failedAt
	c.mu.RUnlock()
	if cached && (hasImage || time.Since(failedAt) < artRetryInterval) {
		return c.localURL(key, hasImage)
	}

	img, ctype, err := c.fetchImage(artURL, "")
	if err != nil && c.plexURL != "" && album != "" {
		log.Printf("art: renderer art URL failed (%v), falling back to Plex lookup for %q", err, album)
		img, ctype, err = c.fetchFromPlex(album, artist)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.key = key
	if err != nil {
		log.Printf("art: no art for %q: %v", album, err)
		c.image, c.contentType, c.failedAt = nil, "", time.Now()
		return ""
	}
	c.image, c.contentType, c.failedAt = img, ctype, time.Time{}
	return c.localURL(key, true)
}

func (c *artCache) localURL(key string, hasImage bool) string {
	if !hasImage {
		return ""
	}
	sum := sha1.Sum([]byte(key))
	// Query string changes with the track's art, so the browser refetches.
	return "/api/art?v=" + hex.EncodeToString(sum[:6])
}

// fetchImage GETs an image, sending plexToken as a header (not in the URL, so
// it never ends up in logged errors) when non-empty.
func (c *artCache) fetchImage(u, plexToken string) ([]byte, string, error) {
	if u == "" {
		return nil, "", fmt.Errorf("no art URL")
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, "", err
	}
	if plexToken != "" {
		req.Header.Set("X-Plex-Token", plexToken)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	ctype := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if !strings.HasPrefix(ctype, "image/") {
		return nil, "", fmt.Errorf("unexpected content type %q", ctype)
	}
	img, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, "", err
	}
	return img, ctype, nil
}

func (c *artCache) fetchFromPlex(album, artist string) ([]byte, string, error) {
	q := url.Values{"type": {"9"}, "query": {album}}
	req, err := http.NewRequest("GET", c.plexURL+"/search?"+q.Encode(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Token", c.plexToken)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Plex search: HTTP %d", resp.StatusCode)
	}

	var result struct {
		MediaContainer struct {
			Metadata []struct {
				Title       string `json:"title"`
				ParentTitle string `json:"parentTitle"`
				Thumb       string `json:"thumb"`
			} `json:"Metadata"`
		} `json:"MediaContainer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("Plex search: %v", err)
	}

	// Prefer an exact album title whose album artist appears in the
	// renderer's artist string (which can be e.g. "Pink Floyd-The Division
	// Bell"), then any exact title match.
	thumb := ""
	for _, m := range result.MediaContainer.Metadata {
		if !strings.EqualFold(m.Title, album) || m.Thumb == "" {
			continue
		}
		if m.ParentTitle != "" && strings.Contains(strings.ToLower(artist), strings.ToLower(m.ParentTitle)) {
			thumb = m.Thumb
			break
		}
		if thumb == "" {
			thumb = m.Thumb
		}
	}
	if thumb == "" {
		return nil, "", fmt.Errorf("Plex search: no album titled %q with art", album)
	}

	tq := url.Values{
		"width": {"512"}, "height": {"512"}, "format": {"jpg"},
		"url": {thumb},
	}
	return c.fetchImage(c.plexURL+"/photo/:/transcode?"+tq.Encode(), c.plexToken)
}

func (c *artCache) serveHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	img, ctype := c.image, c.contentType
	c.mu.RUnlock()
	if len(img) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(img)
}

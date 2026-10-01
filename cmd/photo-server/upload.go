package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The upload page: phones on the LAN pick photos from their gallery, which
// are saved to Uploads/ in the photos folder and dealt next (see library).
// The folder only ever holds copies, so photo-server may write to it.

//go:embed upload.html
var uploadHTML []byte

// serveUploads serves the upload page and its POST /upload on addr.
func serveUploads(addr string, u *uploader) error {
	if err := os.MkdirAll(u.dir, 0o755); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(uploadHTML)
	})
	mux.HandleFunc("POST /upload", u.handle)
	log.Printf("upload page on %s, saving to %s", addr, u.dir)
	return http.ListenAndServe(addr, mux)
}

type uploader struct {
	dir      string
	maxBytes int64 // per photo
}

type result struct {
	Name  string `json:"name"`            // as sent by the phone
	Saved string `json:"saved,omitempty"` // file name written
	Error string `json:"error,omitempty"`
}

// handle saves each photo in a multipart POST (field "photo", repeatable),
// streaming it to disk, and replies with a result per file.
func (u *uploader) handle(w http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "expected a multipart form", http.StatusBadRequest)
		return
	}
	var results []result
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "reading upload: "+err.Error(), http.StatusBadRequest)
			return
		}
		if part.FormName() != "photo" || part.FileName() == "" {
			part.Close()
			continue
		}
		res := result{Name: part.FileName()}
		if saved, err := u.save(part); err != nil {
			res.Error = err.Error()
			log.Printf("upload %q from %s: %v", res.Name, r.RemoteAddr, err)
		} else {
			res.Saved = saved
			log.Printf("saved %s from %s", saved, r.RemoteAddr)
		}
		part.Close()
		results = append(results, res)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

var errTooBig = errors.New("too big")

// save writes the photo to a hidden temporary file (which the library
// skips), checks it decodes, then renames it into place, so the library
// never sees half a photo. It returns the name it was saved as.
func (u *uploader) save(part *multipart.Part) (string, error) {
	ext := strings.ToLower(filepath.Ext(part.FileName()))
	if !photoExts[ext] {
		if ext == ".heic" || ext == ".heif" {
			return "", errors.New("HEIC can't be shown; set the camera to \"Most Compatible\" (JPEG)")
		}
		return "", fmt.Errorf("not a photo type that can be shown (%s)", ext)
	}

	tmp, err := os.CreateTemp(u.dir, ".upload-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	n, err := io.Copy(tmp, io.LimitReader(part, u.maxBytes+1))
	if err == nil && n > u.maxBytes {
		err = fmt.Errorf("%w: over %d MB", errTooBig, u.maxBytes>>20)
	}
	if err == nil {
		_, err = tmp.Seek(0, io.SeekStart)
	}
	if err == nil {
		if _, _, derr := image.DecodeConfig(tmp); derr != nil {
			err = fmt.Errorf("not a readable photo: %v", derr)
		}
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return "", err
	}

	// Phones often send every photo as "image.jpg", so names are prefixed
	// with the upload time and never overwrite one another.
	base := safeName(strings.TrimSuffix(filepath.Base(part.FileName()), filepath.Ext(part.FileName())))
	stamp := time.Now().Format("20060102-150405")
	for i := 0; ; i++ {
		name := fmt.Sprintf("%s-%s%s", stamp, base, ext)
		if i > 0 {
			name = fmt.Sprintf("%s-%s-%d%s", stamp, base, i, ext)
		}
		dst := filepath.Join(u.dir, name)
		// Link fails if dst exists, unlike rename, so nothing is clobbered.
		if err := os.Link(tmp.Name(), dst); err == nil {
			return name, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// safeName keeps a file name to plain characters, without leading dots (so
// it isn't hidden) and not too long.
func safeName(s string) string {
	s = strings.TrimLeft(unsafeChars.ReplaceAllString(s, "_"), "._")
	if len(s) > 60 {
		s = s[:60]
	}
	if s == "" {
		s = "photo"
	}
	return s
}

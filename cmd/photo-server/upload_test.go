package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func post(t *testing.T, u *uploader, files map[string][]byte) []result {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, data := range files {
		fw, _ := mw.CreateFormFile("photo", name)
		fw.Write(data)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	u.handle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var res []result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestUpload(t *testing.T) {
	dir := t.TempDir()
	u := &uploader{dir: dir, maxBytes: 1 << 20}
	good := pngBytes(t)

	// Same name twice: both kept.
	for range 2 {
		res := post(t, u, map[string][]byte{"image.png": good})
		if len(res) != 1 || res[0].Error != "" || !strings.Contains(res[0].Saved, "-image") {
			t.Fatalf("got %+v", res)
		}
	}
	for name, data := range map[string][]byte{
		"fake.jpg":  []byte("not a jpeg"),
		"IMG.HEIC":  good,
		"notes.txt": []byte("hi"),
		"big.png":   append(good, make([]byte, 2<<20)...),
	} {
		if res := post(t, u, map[string][]byte{name: data}); len(res) != 1 || res[0].Error == "" {
			t.Errorf("%s: got %+v, want an error", name, res)
		}
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("folder has %v, want just the two photos", names)
	}
}

func TestSafeName(t *testing.T) {
	for in, want := range map[string]string{
		"IMG_0001":       "IMG_0001",
		"../../etc/pass": "etc_pass",
		".hidden":        "hidden",
		"café photo":     "caf_photo",
		"":               "photo",
	} {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %q, want %q", in, got, want)
		}
	}
}

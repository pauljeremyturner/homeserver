package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryDealsEachPhotoOnce(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "a.png"), 4, 4)
	writePNG(t, filepath.Join(dir, "trip", "b.PNG"), 4, 4)
	writePNG(t, filepath.Join(dir, ".thumbnails", "c.png"), 4, 4)
	writePNG(t, filepath.Join(dir, "Screenshots", "d.png"), 4, 4)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644)

	lib := newLibrary(dir)
	seen := map[string]int{}
	for range 4 {
		p, err := lib.next()
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(dir, p)
		seen[rel]++
	}
	if len(seen) != 2 || seen["a.png"] != 2 || seen[filepath.Join("trip", "b.PNG")] != 2 {
		t.Errorf("dealt %v, want a.png and trip/b.PNG twice each", seen)
	}

	if _, err := newLibrary(t.TempDir()).next(); err != errNoPhotos {
		t.Errorf("empty dir: got %v, want errNoPhotos", err)
	}
}

func TestPickOneSkipsBrokenPhotos(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "broken.jpg"), []byte("not a jpeg"), 0o644)
	writePNG(t, filepath.Join(dir, "good.png"), 3000, 2000)
	for range 3 {
		p, err := pickOne(newLibrary(dir))
		if err != nil {
			t.Fatal(err)
		}
		if p.name != "good.png" {
			t.Errorf("picked %s", p.name)
		}
		// Kept no bigger than keepSize.
		if b := p.img.Bounds(); b.Dx() != keepSize || b.Dy() != 1280 {
			t.Errorf("kept at %v", b.Size())
		}
	}
}

func TestEncodeFit(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1920, 1080))
	for _, c := range []struct{ w, h, wantW, wantH int }{
		{800, 480, 800, 450},     // wide screen: width-limited
		{480, 800, 480, 270},     // portrait screen
		{0, 0, 1920, 1080},       // no limit
		{4000, 4000, 1920, 1080}, // never enlarged
	} {
		data, err := encodeFit(img, c.w, c.h)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != c.wantW || cfg.Height != c.wantH {
			t.Errorf("fit %dx%d: got %dx%d, want %dx%d", c.w, c.h, cfg.Width, cfg.Height, c.wantW, c.wantH)
		}
	}
}

func TestLibraryDealsAddedPhotosNext(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.png", "b.png", "c.png"} {
		writePNG(t, filepath.Join(dir, n), 4, 4)
	}
	lib := newLibrary(dir)
	first, err := lib.next()
	if err != nil {
		t.Fatal(err)
	}

	// An upload part-way through the deck is dealt next; one still arriving
	// (hidden) isn't.
	writePNG(t, filepath.Join(dir, "Uploads", "new.png"), 4, 4)
	writePNG(t, filepath.Join(dir, "Uploads", ".upload-123"), 4, 4)
	if p, _ := lib.next(); filepath.Base(p) != "new.png" {
		t.Errorf("after an upload, dealt %s, want new.png", p)
	}

	// A removed photo is dropped from the deck.
	var gone string
	for _, n := range []string{"a.png", "b.png", "c.png"} {
		if filepath.Join(dir, n) != first {
			gone = filepath.Join(dir, n)
			break
		}
	}
	os.Remove(gone)
	// Only the third of a, b, c was left in this deck.
	if p, _ := lib.next(); p == gone || p == first {
		t.Errorf("dealt %s, want the photo left in the deck", p)
	}
	// Then a fresh deck of the three still there.
	seen := map[string]bool{}
	for range 3 {
		p, err := lib.next()
		if err != nil {
			t.Fatal(err)
		}
		seen[p] = true
	}
	if seen[gone] || len(seen) != 3 {
		t.Errorf("dealt %v, want the three photos left, not %s", seen, gone)
	}
}

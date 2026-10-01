package main

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"io/fs"
	"log"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	// Decoders beyond imaging's own (JPEG, PNG, GIF, TIFF, BMP).
	_ "golang.org/x/image/webp"

	"homeserver/internal/broadcast"
)

// photoExts are the files taken as photos. HEIC isn't among them: Go has no
// decoder for it.
var photoExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".tif": true, ".tiff": true, ".bmp": true}

// keepSize is the most a picked photo is kept at (longest side, pixels): a
// full-size 50MP photo would otherwise sit in memory at ~200MB, and no
// display needs more than this.
const keepSize = 1920

// library deals out the photos under dir in a random order, reshuffling
// once every one has been dealt. The folder is looked at again on every
// deal, so a photo added since the last one (an upload) is dealt next, and
// a removed one is dropped.
type library struct {
	dir   string
	deck  []string
	known map[string]bool // what the last look found; nil before the first
}

func newLibrary(dir string) *library { return &library{dir: dir} }

var errNoPhotos = errors.New("no photos")

func (l *library) next() (string, error) {
	found, err := l.scan()
	if err != nil {
		return "", err
	}
	there := make(map[string]bool, len(found))
	for _, p := range found {
		there[p] = true
	}
	if l.known != nil {
		deck := l.deck[:0]
		for _, p := range l.deck {
			if there[p] {
				deck = append(deck, p)
			}
		}
		// Dealt from the end, so these go next.
		for _, p := range found {
			if !l.known[p] {
				deck = append(deck, p)
			}
		}
		l.deck = deck
	}
	l.known = there
	if len(l.deck) == 0 {
		if len(found) == 0 {
			return "", errNoPhotos
		}
		l.deck = found
		rand.Shuffle(len(l.deck), func(i, j int) { l.deck[i], l.deck[j] = l.deck[j], l.deck[i] })
	}
	p := l.deck[len(l.deck)-1]
	l.deck = l.deck[:len(l.deck)-1]
	return p, nil
}

// scan lists the photos under dir.
func (l *library) scan() ([]string, error) {
	var found []string
	err := filepath.WalkDir(l.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden files and folders (.thumbnails, .trashed-..., and
		// uploads' .upload-* files still arriving), and screenshots,
		// which the desktop saves under ~/Pictures too.
		if (strings.HasPrefix(d.Name(), ".") || d.IsDir() && d.Name() == "Screenshots") && path != l.dir {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && photoExts[strings.ToLower(filepath.Ext(path))] {
			found = append(found, path)
		}
		return nil
	})
	return found, err
}

// pick is the photo showing: decoded, upright and at most keepSize.
type pick struct {
	img         image.Image
	name        string
	since, next time.Time
}

// load decodes the photo at path, turned upright by its EXIF orientation
// and shrunk to keepSize.
func load(path string) (image.Image, error) {
	img, err := imaging.Open(path, imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	return imaging.Fit(img, keepSize, keepSize, imaging.Lanczos), nil
}

// pickPhotos publishes a new photo every period, on the clock (anchored to
// the Unix epoch), skipping any that won't decode.
func pickPhotos(lib *library, period time.Duration, bc *broadcast.Broadcaster[*pick]) {
	for {
		now := time.Now()
		since := now.Truncate(period)
		next := since.Add(period)
		if p, err := pickOne(lib); err != nil {
			log.Printf("photos: %v", err)
		} else {
			p.since, p.next = since, next
			bc.Publish(p)
		}
		time.Sleep(time.Until(next))
	}
}

// pickOne loads the library's next photo that decodes, giving up after a
// whole deck's worth of failures.
func pickOne(lib *library) (*pick, error) {
	for tries := 0; ; tries++ {
		path, err := lib.next()
		if err != nil {
			return nil, err
		}
		img, err := load(path)
		if err == nil {
			name, _ := filepath.Rel(lib.dir, path)
			return &pick{img: img, name: name}, nil
		}
		log.Printf("photos: skipping %s: %v", path, err)
		if tries > len(lib.deck) {
			return nil, errors.New("no photo would decode")
		}
	}
}

// encodeFit shrinks img to fit w x h (either 0 for no limit) and encodes it
// as a JPEG.
func encodeFit(img image.Image, w, h int) ([]byte, error) {
	b := img.Bounds()
	if w <= 0 {
		w = b.Dx()
	}
	if h <= 0 {
		h = b.Dy()
	}
	img = imaging.Fit(img, w, h, imaging.Lanczos)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

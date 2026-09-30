// photo-gui shows the photos photo-server streams, full screen under the
// cage Wayland kiosk compositor, fading from one to the next.
package main

import (
	"bytes"
	"context"
	"flag"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	photopb "homeserver/gen/photo"
	"homeserver/internal/countdown"
)

// version is set at build time by build.sh (-ldflags "-X main.version=...").
var version = "dev"

// fadeTime is how long one photo takes to fade into the next.
const fadeTime = time.Second

func main() {
	server := flag.String("server", envOr("PHOTO_SERVER_ADDR", "localhost:9092"), "photo-server gRPC address")
	kiosk := flag.Bool("kiosk", false, "full screen with no window decorations (for cage)")
	shot := flag.String("screenshot", "", "render one 800x480 frame of the current photo to this PNG and exit")
	flag.Parse()
	log.Printf("photo-gui %s, server %s", version, *server)

	conn, err := grpc.NewClient(*server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connecting to photo-server: %v", err)
	}
	s := &state{}

	if *shot != "" {
		const w, h = 800, 480
		go stream(context.Background(), photopb.NewPhotoServiceClient(conn), w, h, s, func() {})
		for deadline := time.Now().Add(10 * time.Second); s.current() == nil && time.Now().Before(deadline); {
			time.Sleep(100 * time.Millisecond)
		}
		if err := screenshot(*shot, s, w, h); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		return
	}

	w := new(app.Window)
	go func() {
		w.Option(app.Title("Photos"), app.Size(unit.Dp(800), unit.Dp(480)))
		if *kiosk {
			// cage has no server-side decorations, so without this Gio
			// draws its own title bar.
			w.Option(app.Fullscreen.Option(), app.Decorated(true))
		}
		if err := run(w, photopb.NewPhotoServiceClient(conn), s); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

// shown is a decoded photo, when it arrived, for the fade in, and when the
// server picked it and will move on, for the countdown line.
type shown struct {
	img         paint.ImageOp
	arrived     time.Time
	since, next time.Time
}

// state holds the current photo and the one it's fading from; the stream
// writes it, the window reads it each frame.
type state struct {
	mu        sync.Mutex
	cur, prev *shown
}

func (s *state) set(img image.Image, since, next time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prev, s.cur = s.cur, &shown{img: paint.NewImageOp(img), arrived: time.Now(), since: since, next: next}
}

func (s *state) current() *shown {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

func (s *state) both() (cur, prev *shown) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur, s.prev
}

// stream receives photos sized for a w x h screen, reconnecting after a
// pause whenever the stream drops (the server resends the current photo on
// subscribe).
func stream(ctx context.Context, c photopb.PhotoServiceClient, w, h int, s *state, changed func()) {
	for {
		st, err := c.StreamPhotos(ctx, &photopb.StreamPhotosRequest{Width: int32(w), Height: int32(h)},
			// A photo is bigger than gRPC's usual messages, but well under
			// this even at 4K.
			grpc.MaxCallRecvMsgSize(32<<20))
		if err == nil {
			for {
				p, err := st.Recv()
				if err != nil {
					log.Printf("photo stream: %v", err)
					break
				}
				img, err := jpeg.Decode(bytes.NewReader(p.Jpeg))
				if err != nil {
					log.Printf("photo %s: %v", p.Name, err)
					continue
				}
				s.set(img, time.Unix(p.SinceUnix, 0), time.Unix(p.NextUnix, 0))
				changed()
			}
		} else {
			log.Printf("photo stream: %v", err)
		}
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func run(w *app.Window, c photopb.PhotoServiceClient, s *state) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	started := false
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			// Ask for photos once the screen's size is known, so the server
			// sends them no bigger than it.
			if !started {
				started = true
				go stream(context.Background(), c, e.Size.X, e.Size.Y, s, w.Invalidate)
			}
			gtx := app.NewContext(&ops, e)
			draw(gtx, th, s, true)
			e.Frame(gtx.Ops)
		}
	}
}

// draw shows the current photo as large as fits, centred on black, fading
// in over the previous one, with a line along the bottom counting down to
// the next (redrawn as it moves when live).
func draw(gtx layout.Context, th *material.Theme, s *state, live bool) {
	paint.Fill(gtx.Ops, rgbBlack)
	cur, prev := s.both()
	if cur == nil {
		l := material.Label(th, unit.Sp(16), "Waiting for photos...")
		l.Color = rgbFaint
		layout.Center.Layout(gtx, l.Layout)
		return
	}
	alpha := float32(gtx.Now.Sub(cur.arrived)) / float32(fadeTime)
	if alpha < 1 {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(33 * time.Millisecond)})
		if prev != nil {
			photo(gtx, prev.img, 1)
		}
	}
	photo(gtx, cur.img, min(max(alpha, 0), 1))
	if cur.next.After(cur.since) {
		defer op.Offset(image.Pt(0, gtx.Constraints.Max.Y-gtx.Dp(countdown.Height))).Push(gtx.Ops).Pop()
		countdown.Layout(gtx, gtx.Now.Sub(cur.since), cur.next.Sub(cur.since), live)
	}
}

func photo(gtx layout.Context, img paint.ImageOp, alpha float32) {
	defer paint.PushOpacity(gtx.Ops, alpha).Pop()
	gtx.Constraints.Min = gtx.Constraints.Max
	widget.Image{Src: img, Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
}

var (
	rgbBlack = rgb(0x000000)
	rgbFaint = rgb(0x5a616b)
)

func rgb(c uint32) color.NRGBA {
	return color.NRGBA{R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c), A: 0xff}
}

func screenshot(path string, s *state, width, height int) error {
	win, err := headless.NewWindow(width, height)
	if err != nil {
		return err
	}
	defer win.Release()
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(width, height)),
		// Past the fade in.
		Now: time.Now().Add(fadeTime),
	}
	draw(gtx, th, s, false)
	if err := win.Frame(&ops); err != nil {
		return err
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := win.Screenshot(img); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

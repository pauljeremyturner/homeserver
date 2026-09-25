// info-gui is the graphical info dashboard for the Tinker Board's 800x480
// screen, run full-screen under the cage Wayland kiosk compositor. It shows
// the local clock and board temperature, and weather/moon, crypto/wallet and
// news streamed from info-server over gRPC.
package main

import (
	"context"
	"flag"
	"image"
	"image/png"
	"log"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func main() {
	server := flag.String("server", envOr("INFO_SERVER_ADDR", "localhost:9090"), "info-server gRPC address")
	demo := flag.Bool("demo", false, "use built-in sample data instead of info-server")
	shot := flag.String("screenshot", "", "render one 800x480 frame to this PNG and exit")
	kiosk := flag.Bool("kiosk", false, "full screen with no window decorations (for cage)")
	wait := flag.Duration("wait", 4*time.Second, "with -screenshot and live data: how long to wait for the streams first")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newState()
	w := new(app.Window)
	changed := func() {}
	if *shot == "" {
		changed = w.Invalidate
	}
	if *demo {
		fillDemo(s)
	} else if err := startStreams(ctx, *server, s, changed); err != nil {
		log.Fatalf("connecting to info-server: %v", err)
	}
	go pollBoardTemp(ctx, s, changed)

	if *shot != "" {
		if !*demo {
			time.Sleep(*wait)
		}
		if err := screenshot(*shot, s); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		return
	}

	go func() {
		w.Option(app.Title("Info"), app.Size(unit.Dp(designWidth), unit.Dp(designHeight)))
		if *kiosk {
			// cage has no server-side decorations, so without this Gio
			// draws its own title bar.
			w.Option(app.Fullscreen.Option(), app.Decorated(true))
		}
		if err := run(w, s); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window, s *state) error {
	u := newUI()
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			gtx.Metric = fitMetric(e.Size)
			u.layout(gtx, gtx.Now, s.snapshot())
			// Redraw on the next second for the clock (the ticker asks for
			// its own faster frames).
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Truncate(time.Second).Add(time.Second)})
			e.Frame(gtx.Ops)
		}
	}
}

// The layout is designed at 800x480dp; fitMetric scales dp so that fills
// whatever size the screen really is (the Tinker's HDMI panel has reported
// both 800x480 and 1024x768 modes).
const designWidth, designHeight = 800, 480

func fitMetric(size image.Point) unit.Metric {
	scale := min(float32(size.X)/designWidth, float32(size.Y)/designHeight)
	if scale <= 0 {
		scale = 1
	}
	return unit.Metric{PxPerDp: scale, PxPerSp: scale}
}

func screenshot(path string, s *state) error {
	const width, height = designWidth, designHeight
	win, err := headless.NewWindow(width, height)
	if err != nil {
		return err
	}
	defer win.Release()
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(width, height)),
		Now:         time.Now(),
	}
	newUI().layout(gtx, gtx.Now, s.snapshot())
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

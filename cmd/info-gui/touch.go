package main

import (
	"math"
	"math/bits"
	"os"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"

	displaypb "homeserver/gen/display"
	"homeserver/internal/pagecycle"
)

// On a touch screen the rotation doesn't run by itself: weather is home and
// stays up, and swiping left or right steps through the other pages. Each
// stays up for its slot in the rotation, then weather comes back.

// touchOn says whether to run that way, from -touch: "on", "off", or
// "auto" for whenever there's a touch screen.
func touchOn(mode string) bool {
	switch mode {
	case "on":
		return true
	case "off":
		return false
	}
	return hasTouchscreen()
}

// hasTouchscreen reports whether the kernel knows of a touch screen. Its
// device list isn't namespaced, so this works from inside a container too.
func hasTouchscreen() bool {
	b, err := os.ReadFile("/proc/bus/input/devices")
	return err == nil && listsTouchscreen(string(b))
}

// Input properties and axes, from linux/input-event-codes.h.
const (
	inputPropDirect = 0x01 // on the screen itself, not a touchpad
	absMTPositionX  = 0x35 // tracks each finger's position
)

// listsTouchscreen reports whether a /proc/bus/input/devices listing has a
// direct multi-touch device: a touch screen, not a touchpad or a mouse.
func listsTouchscreen(devices string) bool {
	for _, dev := range strings.Split(devices, "\n\n") {
		var prop, abs string
		for _, line := range strings.Split(dev, "\n") {
			if v, ok := strings.CutPrefix(line, "B: PROP="); ok {
				prop = v
			} else if v, ok := strings.CutPrefix(line, "B: ABS="); ok {
				abs = v
			}
		}
		if bitSet(prop, inputPropDirect) && bitSet(abs, absMTPositionX) {
			return true
		}
	}
	return false
}

// bitSet reads bit n of a kernel bitmap as /proc prints it: hex words the
// size of the kernel's long, most significant first. The binary is built
// for the board's own word size, so that's bits.UintSize.
func bitSet(bitmap string, n int) bool {
	words := strings.Fields(bitmap)
	i := len(words) - 1 - n/bits.UintSize
	if i < 0 {
		return false
	}
	w, err := strconv.ParseUint(words[i], 16, bits.UintSize)
	return err == nil && w&(1<<(n%bits.UintSize)) != 0
}

// touchPages is the rotation the pages are swiped through, weather first as
// home: info-server's when it's sent one, else the default.
func touchPages(u *displaypb.PageUpdate) []*displaypb.PageSlot {
	cycle := defaultCycle
	if u != nil && pagecycle.Valid(u.Cycle) {
		cycle = u.Cycle
	}
	pages := []*displaypb.PageSlot{{Page: displaypb.Page_PAGE_WEATHER}}
	for _, s := range cycle {
		if s.Page != displaypb.Page_PAGE_WEATHER {
			pages = append(pages, s)
		}
	}
	return pages
}

// touchNav is which page has been swiped to and when.
type touchNav struct {
	at    int // into touchPages; 0 is weather
	since time.Time
	// Where the finger went down, while it's down.
	press   f32.Point
	pressed bool
}

// showing returns the page up at now, when it came up and when it goes back
// to weather (the zero time for weather, which stays).
func (n *touchNav) showing(now time.Time, pages []*displaypb.PageSlot) (page displaypb.Page, since, next time.Time) {
	if n.at >= len(pages) { // the rotation got shorter
		n.at = 0
	}
	if n.at > 0 {
		next = n.since.Add(time.Duration(pages[n.at].Seconds) * time.Second)
		if !now.Before(next) {
			n.at, n.since = 0, next
		}
	}
	if n.at == 0 {
		return pages[0].Page, n.since, time.Time{}
	}
	return pages[n.at].Page, n.since, next
}

// swipe moves step pages along, wrapping round, and starts the new one's
// time at now.
func (n *touchNav) swipe(now time.Time, step, count int) {
	n.at = ((n.at+step)%count + count) % count
	n.since = now
}

// swipes takes the touches (or mouse drags) across the whole window since
// the last frame and turns a sideways one into a swipe: left for the next
// page, right for the one before.
func (u *ui) swipes(gtx layout.Context, count int) {
	n := &u.nav
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: n, Kinds: pointer.Press | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			n.press, n.pressed = e.Position, true
		case pointer.Release:
			d := e.Position.Sub(n.press)
			if n.pressed && math.Abs(float64(d.X)) > float64(gtx.Dp(80)) && math.Abs(float64(d.X)) > 2*math.Abs(float64(d.Y)) {
				step := 1
				if d.X > 0 {
					step = -1
				}
				n.swipe(gtx.Now, step, count)
			}
			n.pressed = false
		case pointer.Cancel:
			n.pressed = false
		}
	}
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, n)
}

// pageDots draws a dot for each page, the one up filled in, centred across
// a header of the given size (between the clock and the date).
func pageDots(gtx layout.Context, header f32.Point, count, at int) {
	r, gap := float32(gtx.Dp(4)), float32(gtx.Dp(16))
	x := header.X/2 - gap*float32(count-1)/2
	for i := range count {
		c := colFaint
		if i == at {
			c = colText
		}
		fillCircle(gtx.Ops, c, x+gap*float32(i), header.Y/2, r)
	}
}

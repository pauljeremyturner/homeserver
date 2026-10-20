package main

import (
	"testing"
	"time"

	displaypb "homeserver/gen/display"
)

// From the XPS 13 9365's /proc/bus/input/devices, trimmed to the lines read.
const (
	xpsTouchpad = `I: Bus=0018 Vendor=06cb Product=76af Version=0100
N: Name="DLL077A:01 06CB:76AF Touchpad"
B: PROP=5
B: EV=1b
B: ABS=2e0800000000003`
	xpsPen = `I: Bus=0018 Vendor=056a Product=4831 Version=0100
N: Name="Wacom HID 4831 Pen"
B: PROP=2
B: EV=1b
B: ABS=10001000003`
	xpsFinger = `I: Bus=0018 Vendor=056a Product=4831 Version=0100
N: Name="Wacom HID 4831 Finger"
B: PROP=2
B: EV=1b
B: ABS=260800000000003`
)

func TestListsTouchscreen(t *testing.T) {
	if !listsTouchscreen(xpsTouchpad + "\n\n" + xpsPen + "\n\n" + xpsFinger + "\n") {
		t.Error("missed the XPS touch screen")
	}
	if listsTouchscreen(xpsTouchpad + "\n\n" + xpsPen + "\n") {
		t.Error("took a touchpad or pen for a touch screen")
	}
	if listsTouchscreen("") {
		t.Error("touch screen in an empty listing")
	}
}

func TestBitSet(t *testing.T) {
	// A bitmap longer than one word: bit 0 is in the last word.
	if !bitSet("1 0 1", 0) || bitSet("1 0 1", 1) {
		t.Error("bit 0 of a multi-word bitmap")
	}
	if bitSet("1", 200) || bitSet("", 0) || bitSet("zz", 0) {
		t.Error("bit outside or in a bad bitmap reads as set")
	}
}

func TestTouchPages(t *testing.T) {
	weather, markets, planets := displaypb.Page_PAGE_WEATHER, displaypb.Page_PAGE_MARKETS, displaypb.Page_PAGE_PLANETS
	u := &displaypb.PageUpdate{Cycle: []*displaypb.PageSlot{{Page: markets, Seconds: 30}, {Page: weather, Seconds: 60}, {Page: planets, Seconds: 20}}}
	got := touchPages(u)
	if len(got) != 3 || got[0].Page != weather || got[1].Page != markets || got[2].Page != planets {
		t.Errorf("weather should lead, the rest in order: %v", got)
	}
	if got := touchPages(nil); len(got) != len(defaultCycle) || got[0].Page != weather {
		t.Errorf("no server: %v", got)
	}
}

func TestTouchNav(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	pages := touchPages(nil) // weather, markets (30s), markets 6m (30s), planets (30s)
	var n touchNav

	// Weather stays up, with nowhere to go back to.
	if p, _, next := n.showing(t0.Add(time.Hour), pages); p != displaypb.Page_PAGE_WEATHER || !next.IsZero() {
		t.Errorf("home: got %v until %v", p, next)
	}
	// Left goes on to the next page, for its slot.
	n.swipe(t0, 1, len(pages))
	if p, since, next := n.showing(t0.Add(10*time.Second), pages); p != displaypb.Page_PAGE_MARKETS || !since.Equal(t0) || !next.Equal(t0.Add(30*time.Second)) {
		t.Errorf("swiped left: got %v %v-%v", p, since, next)
	}
	// Then weather comes back, from when that ran out.
	if p, since, _ := n.showing(t0.Add(31*time.Second), pages); p != displaypb.Page_PAGE_WEATHER || !since.Equal(t0.Add(30*time.Second)) {
		t.Errorf("after the slot: got %v since %v", p, since)
	}
	// Right from weather wraps round to the last page.
	n.swipe(t0.Add(40*time.Second), -1, len(pages))
	if p, _, _ := n.showing(t0.Add(41*time.Second), pages); p != displaypb.Page_PAGE_PLANETS {
		t.Errorf("swiped right from home: got %v", p)
	}
	// A rotation that's shrunk under it falls back to weather.
	if p, _, _ := n.showing(t0.Add(42*time.Second), pages[:2]); p != displaypb.Page_PAGE_WEATHER {
		t.Errorf("shorter rotation: got %v", p)
	}
}

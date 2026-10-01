package main

import (
	"time"

	displaypb "homeserver/gen/display"
	"homeserver/internal/pagecycle"
)

// fadeTime is how long a page takes to fade out, and the next to fade in.
const fadeTime = 350 * time.Millisecond

// defaultCycle is the page rotation until info-server sends its own (the
// server's defaults, so a display that never reaches it still agrees).
var defaultCycle = []*displaypb.PageSlot{
	{Page: displaypb.Page_PAGE_WEATHER, Seconds: 60},
	{Page: displaypb.Page_PAGE_MARKETS, Seconds: 30},
	{Page: displaypb.Page_PAGE_MARKETS_6M, Seconds: 30},
	{Page: displaypb.Page_PAGE_PLANETS, Seconds: 30},
}

// pageShowing is which page to show at now and the times it started and
// will end. info-server's PageUpdate is followed while it's current; once
// it's due to change (or if there's never been one) the client carries on
// through the server's rotation (or its own default) on its own clock, the
// same way the server does, so a late or lost update never leaves the
// screen stuck.
func pageShowing(now time.Time, u *displaypb.PageUpdate) (page displaypb.Page, since, next time.Time) {
	cycle := defaultCycle
	if u != nil {
		next := time.Unix(u.NextUnix, 0)
		if now.Before(next) && u.Page != displaypb.Page_PAGE_UNKNOWN {
			return u.Page, time.Unix(u.SinceUnix, 0), next
		}
		if pagecycle.Valid(u.Cycle) {
			cycle = u.Cycle
		}
	}
	return pagecycle.At(now, cycle)
}

// pageOpacity fades a page in over fadeTime after since and out over
// fadeTime before next, and says when the next animation frame is due (the
// zero time if none is needed before the fade out starts).
func pageOpacity(now, since, next time.Time) (float32, time.Time) {
	in := now.Sub(since)
	out := next.Sub(now)
	switch {
	case in < fadeTime:
		return float32(in) / float32(fadeTime), now.Add(tickerFrame)
	case out < fadeTime:
		return max(float32(out)/float32(fadeTime), 0), now.Add(tickerFrame)
	}
	return 1, next.Add(-fadeTime)
}

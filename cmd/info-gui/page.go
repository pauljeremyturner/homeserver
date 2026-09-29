package main

import (
	"time"

	displaypb "homeserver/gen/display"
)

// fadeTime is how long a page takes to fade out, and the next to fade in.
const fadeTime = 350 * time.Millisecond

// defaultPagePeriod is used until info-server says otherwise.
const defaultPagePeriod = 30 * time.Second

var pageCycle = []displaypb.Page{displaypb.Page_PAGE_WEATHER, displaypb.Page_PAGE_MARKETS, displaypb.Page_PAGE_METALS}

// pageShowing is which page to show at now and the times it started and
// will end. info-server's PageUpdate is followed while it's current; once
// it's due to change (or if there's never been one) the client carries on
// cycling on its own clock the same way the server does — anchored to the
// Unix epoch — so a late or lost update never leaves the screen stuck.
func pageShowing(now time.Time, u *displaypb.PageUpdate) (page displaypb.Page, since, next time.Time) {
	period := defaultPagePeriod
	if u != nil && u.PeriodSeconds > 0 {
		period = time.Duration(u.PeriodSeconds) * time.Second
		next := time.Unix(u.NextUnix, 0)
		if now.Before(next) && u.Page != displaypb.Page_PAGE_UNKNOWN {
			return u.Page, time.Unix(u.SinceUnix, 0), next
		}
	}
	n := now.UnixNano() / int64(period)
	since = time.Unix(0, n*int64(period))
	return pageCycle[n%int64(len(pageCycle))], since, since.Add(period)
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

package main

import (
	"time"

	displaypb "homeserver/gen/display"
	"homeserver/internal/broadcast"
)

// pageCycle is the order displays show their pages in.
var pageCycle = []displaypb.Page{displaypb.Page_PAGE_WEATHER, displaypb.Page_PAGE_MARKETS, displaypb.Page_PAGE_PLANETS}

// pageAt is the page showing at t when each page is shown for period, with
// the cycle anchored to the Unix epoch so it lines up with the clock (with
// a 60s period and three pages, the weather page shows on minutes divisible
// by three).
func pageAt(t time.Time, period time.Duration) *displaypb.PageUpdate {
	n := t.UnixNano() / int64(period)
	since := time.Unix(0, n*int64(period))
	return &displaypb.PageUpdate{
		Page:          pageCycle[n%int64(len(pageCycle))],
		SinceUnix:     since.Unix(),
		NextUnix:      since.Add(period).Unix(),
		PeriodSeconds: int64(period / time.Second),
	}
}

func cyclePages(bc *broadcast.Broadcaster[*displaypb.PageUpdate], period time.Duration) {
	for {
		u := pageAt(time.Now(), period)
		bc.Publish(u)
		time.Sleep(time.Until(time.Unix(u.NextUnix, 0)))
	}
}

type displayServer struct {
	displaypb.UnimplementedDisplayServiceServer
	bc *broadcast.Broadcaster[*displaypb.PageUpdate]
}

func (s *displayServer) StreamPage(_ *displaypb.StreamPageRequest, stream displaypb.DisplayService_StreamPageServer) error {
	return forward(s.bc, stream)
}

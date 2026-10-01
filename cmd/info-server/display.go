package main

import (
	"time"

	displaypb "homeserver/gen/display"
	"homeserver/internal/broadcast"
	"homeserver/internal/pagecycle"
)

// pageCycle is the order displays show their pages in and for how long:
// the weather for pageSeconds, the two markets pages, the week's prices
// then six months', for marketsSeconds each, and the planets for
// planetsSeconds (by default 60+30+30+30s, a rotation every 2.5 minutes).
func pageCycle(pageSeconds, marketsSeconds, planetsSeconds int) []*displaypb.PageSlot {
	return []*displaypb.PageSlot{
		{Page: displaypb.Page_PAGE_WEATHER, Seconds: int64(pageSeconds)},
		{Page: displaypb.Page_PAGE_MARKETS, Seconds: int64(marketsSeconds)},
		{Page: displaypb.Page_PAGE_MARKETS_6M, Seconds: int64(marketsSeconds)},
		{Page: displaypb.Page_PAGE_PLANETS, Seconds: int64(planetsSeconds)},
	}
}

// pageAt is the PageUpdate for the page showing at t.
func pageAt(t time.Time, cycle []*displaypb.PageSlot) *displaypb.PageUpdate {
	page, since, next := pagecycle.At(t, cycle)
	return &displaypb.PageUpdate{
		Page:          page,
		SinceUnix:     since.Unix(),
		NextUnix:      next.Unix(),
		PeriodSeconds: next.Unix() - since.Unix(),
		Cycle:         cycle,
	}
}

func cyclePages(bc *broadcast.Broadcaster[*displaypb.PageUpdate], cycle []*displaypb.PageSlot) {
	for {
		u := pageAt(time.Now(), cycle)
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

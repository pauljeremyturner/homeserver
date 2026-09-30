package main

import (
	"math"
	"testing"
	"time"

	displaypb "homeserver/gen/display"
)

func TestPageAt(t *testing.T) {
	start := time.Date(2026, 9, 27, 15, 24, 0, 0, time.UTC) // minute divisible by 3
	u := pageAt(start.Add(59*time.Second), time.Minute)
	if u.Page != displaypb.Page_PAGE_WEATHER || u.SinceUnix != start.Unix() || u.NextUnix != start.Add(time.Minute).Unix() || u.PeriodSeconds != 60 {
		t.Errorf("first minute: %v", u)
	}
	if u := pageAt(start.Add(time.Minute), time.Minute); u.Page != displaypb.Page_PAGE_MARKETS {
		t.Errorf("second minute: %v", u.Page)
	}
	if u := pageAt(start.Add(2*time.Minute), time.Minute); u.Page != displaypb.Page_PAGE_PLANETS {
		t.Errorf("third minute: %v", u.Page)
	}
	if u := pageAt(start.Add(3*time.Minute), time.Minute); u.Page != displaypb.Page_PAGE_WEATHER {
		t.Errorf("fourth minute: %v", u.Page)
	}
}

func TestPlanetsUpdateHistory(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	for _, years := range []int{10, 20, 50, 200} {
		u := planetsUpdate(now, years)
		start := time.Unix(u.HistoryStartUnix, 0)
		if got := now.Sub(start).Hours() / 24 / 365.25; got < float64(years)-0.01 || got > float64(years)+0.01 {
			t.Errorf("%d years: history starts %v, %.2f years back", years, start, got)
		}
		if len(u.History) != 8 || len(u.History[0].XAu) != historyFrames+1 || len(u.Orbits) != 8 {
			t.Errorf("%d years: %d paths of %d points, %d orbits", years, len(u.History), len(u.History[0].XAu), len(u.Orbits))
		}
		// The last frame is now.
		if last := u.History[2].XAu[historyFrames]; math.Abs(float64(last)-u.Planets[2].XAu) > 1e-4 {
			t.Errorf("%d years: Earth's last frame x %.5f, now %.5f", years, last, u.Planets[2].XAu)
		}
	}
}

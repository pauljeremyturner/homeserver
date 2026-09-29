package main

import (
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
	if u := pageAt(start.Add(2*time.Minute), time.Minute); u.Page != displaypb.Page_PAGE_METALS {
		t.Errorf("third minute: %v", u.Page)
	}
	if u := pageAt(start.Add(3*time.Minute), time.Minute); u.Page != displaypb.Page_PAGE_WEATHER {
		t.Errorf("fourth minute: %v", u.Page)
	}
}

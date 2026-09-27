package main

import (
	"testing"
	"time"

	displaypb "homeserver/gen/display"
)

func TestPageAt(t *testing.T) {
	even := time.Date(2026, 9, 27, 15, 24, 0, 0, time.UTC)
	u := pageAt(even.Add(59*time.Second), time.Minute)
	if u.Page != displaypb.Page_PAGE_WEATHER || u.SinceUnix != even.Unix() || u.NextUnix != even.Add(time.Minute).Unix() || u.PeriodSeconds != 60 {
		t.Errorf("even minute: %v", u)
	}
	if u := pageAt(even.Add(time.Minute), time.Minute); u.Page != displaypb.Page_PAGE_MARKETS {
		t.Errorf("odd minute: %v", u.Page)
	}
}

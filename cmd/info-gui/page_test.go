package main

import (
	"testing"
	"time"

	displaypb "homeserver/gen/display"
)

func TestPageShowing(t *testing.T) {
	even := time.Date(2026, 9, 27, 15, 24, 0, 0, time.UTC) // an even minute
	weather, markets := displaypb.Page_PAGE_WEATHER, displaypb.Page_PAGE_MARKETS

	// With no word from the server, pages cycle each minute on the clock.
	if p, since, next := pageShowing(even.Add(10*time.Second), nil); p != weather || !since.Equal(even) || !next.Equal(even.Add(time.Minute)) {
		t.Errorf("no server, even minute: got %v %v %v", p, since, next)
	}
	if p, _, _ := pageShowing(even.Add(70*time.Second), nil); p != markets {
		t.Errorf("no server, odd minute: got %v, want markets", p)
	}

	// The server's page wins while it's current, even against the clock.
	u := &displaypb.PageUpdate{Page: markets, SinceUnix: even.Unix(), NextUnix: even.Add(30 * time.Second).Unix(), PeriodSeconds: 30}
	if p, _, next := pageShowing(even.Add(10*time.Second), u); p != markets || !next.Equal(even.Add(30*time.Second)) {
		t.Errorf("server page: got %v until %v", p, next)
	}
	// Once it's overdue, the client carries on with the server's period.
	if p, since, _ := pageShowing(even.Add(31*time.Second), u); p != markets || !since.Equal(even.Add(30*time.Second)) {
		t.Errorf("overdue server page: got %v since %v", p, since)
	}
}

func TestPageOpacity(t *testing.T) {
	since := time.Date(2026, 9, 27, 15, 24, 0, 0, time.UTC)
	next := since.Add(time.Minute)
	for _, c := range []struct {
		at   time.Duration
		want float32
	}{
		{0, 0},
		{fadeTime / 2, 0.5},
		{fadeTime, 1},
		{30 * time.Second, 1},
		{time.Minute - fadeTime/2, 0.5},
		{time.Minute, 0},
	} {
		if got, _ := pageOpacity(since.Add(c.at), since, next); got < c.want-0.01 || got > c.want+0.01 {
			t.Errorf("at +%v: opacity %.2f, want %.2f", c.at, got, c.want)
		}
	}
	// Fully shown, it sleeps until the fade out starts.
	if _, wake := pageOpacity(since.Add(30*time.Second), since, next); !wake.Equal(next.Add(-fadeTime)) {
		t.Errorf("wake %v, want %v", wake, next.Add(-fadeTime))
	}
}

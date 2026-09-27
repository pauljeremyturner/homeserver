package main

import (
	"math"
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

func TestPriceAxis(t *testing.T) {
	for _, c := range []struct{ lo, hi, bottom, step float64 }{
		{60240, 65454, 60000, 1000},
		{1931, 2087, 1920, 30},
		{100, 100, 100, 0.2}, // flat prices still get a usable axis
	} {
		bottom, step := priceAxis(c.lo, c.hi, 7)
		if math.Abs(bottom-c.bottom) > 1e-9 || math.Abs(step-c.step) > 1e-9 {
			t.Errorf("priceAxis(%v, %v) = %v, %v; want %v, %v", c.lo, c.hi, bottom, step, c.bottom, c.step)
		}
		if bottom > c.lo || bottom+6*step < c.hi {
			t.Errorf("priceAxis(%v, %v): %v..%v doesn't cover the prices", c.lo, c.hi, bottom, bottom+6*step)
		}
	}
}

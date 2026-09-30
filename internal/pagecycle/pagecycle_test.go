package pagecycle

import (
	"testing"
	"time"

	displaypb "homeserver/gen/display"
)

func TestAt(t *testing.T) {
	start := time.Date(2026, 9, 27, 15, 24, 0, 0, time.UTC) // minute divisible by 3
	cycle := []*displaypb.PageSlot{
		{Page: displaypb.Page_PAGE_WEATHER, Seconds: 60},
		{Page: displaypb.Page_PAGE_MARKETS, Seconds: 30},
		{Page: displaypb.Page_PAGE_MARKETS_6M, Seconds: 30},
		{Page: displaypb.Page_PAGE_PLANETS, Seconds: 60},
	}
	for _, c := range []struct {
		at          time.Duration
		page        displaypb.Page
		since, next time.Duration
	}{
		{0, displaypb.Page_PAGE_WEATHER, 0, 60 * time.Second},
		{59 * time.Second, displaypb.Page_PAGE_WEATHER, 0, 60 * time.Second},
		{60 * time.Second, displaypb.Page_PAGE_MARKETS, 60 * time.Second, 90 * time.Second},
		{89 * time.Second, displaypb.Page_PAGE_MARKETS, 60 * time.Second, 90 * time.Second},
		{90 * time.Second, displaypb.Page_PAGE_MARKETS_6M, 90 * time.Second, 120 * time.Second},
		{2 * time.Minute, displaypb.Page_PAGE_PLANETS, 2 * time.Minute, 3 * time.Minute},
		{3 * time.Minute, displaypb.Page_PAGE_WEATHER, 3 * time.Minute, 4 * time.Minute},
	} {
		page, since, next := At(start.Add(c.at), cycle)
		if page != c.page || !since.Equal(start.Add(c.since)) || !next.Equal(start.Add(c.next)) {
			t.Errorf("at +%v: %v from %v to %v", c.at, page, since, next)
		}
	}
}

func TestValid(t *testing.T) {
	if Valid(nil) || Valid([]*displaypb.PageSlot{{Page: displaypb.Page_PAGE_WEATHER}}) {
		t.Error("empty cycle or zero-second slot counted as valid")
	}
	if !Valid([]*displaypb.PageSlot{{Page: displaypb.Page_PAGE_WEATHER, Seconds: 1}}) {
		t.Error("one-slot cycle counted as invalid")
	}
}

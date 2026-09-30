package moon

import (
	"testing"
	"time"
)

// Moonrises and moonsets (UTC) from the US Naval Observatory
// (aa.usno.navy.mil/api/rstt/oneday).
var usno = []struct {
	name     string
	lat, lon float64
	day      string
	rise     string // "" if the Moon doesn't rise that day
	set      string
}{
	{"Glasgow", 55.86, -4.25, "2026-09-30", "18:33", "11:59"},
	{"Glasgow", 55.86, -4.25, "2026-10-05", "", "16:15"},
	{"Glasgow", 55.86, -4.25, "2026-01-15", "06:57", "12:16"},
	{"Alicante", 38.35, -0.48, "2026-09-30", "19:48", "10:16"},
}

func TestPassesMatchUSNO(t *testing.T) {
	const tolerance = 3 * time.Minute
	for _, u := range usno {
		day, err := time.Parse(time.DateOnly, u.day)
		if err != nil {
			t.Fatal(err)
		}
		end := day.Add(24 * time.Hour)
		var rises, sets []time.Time
		for _, p := range Passes(u.lat, u.lon, day.Add(-24*time.Hour), end.Add(24*time.Hour)) {
			if !p.Rise.Before(day) && p.Rise.Before(end) {
				rises = append(rises, p.Rise)
			}
			if !p.Set.Before(day) && p.Set.Before(end) {
				sets = append(sets, p.Set)
			}
		}
		check := func(what, want string, got []time.Time) {
			if want == "" {
				if len(got) != 0 {
					t.Errorf("%s %s: %s at %v, USNO has none", u.name, u.day, what, got)
				}
				return
			}
			w, _ := time.Parse(time.DateOnly+" 15:04", u.day+" "+want)
			if len(got) != 1 {
				t.Errorf("%s %s: %d %ss, want one at %s", u.name, u.day, len(got), what, want)
				return
			}
			if d := got[0].Sub(w).Abs(); d > tolerance {
				t.Errorf("%s %s: %s %s, USNO %s (off by %v)", u.name, u.day, what, got[0].Format("15:04:05"), want, d)
			}
		}
		check("rise", u.rise, rises)
		check("set", u.set, sets)
	}
}

func TestPassesAroundNow(t *testing.T) {
	// 2026-09-30 06:00 UTC in Glasgow: the Moon rose the evening before and
	// sets at 11:59, then rises again at 18:33.
	now := time.Date(2026, 9, 30, 6, 0, 0, 0, time.UTC)
	ps := Passes(55.86, -4.25, now, now.Add(36*time.Hour))
	if len(ps) < 2 {
		t.Fatalf("got %d passes, want at least 2", len(ps))
	}
	near := func(got time.Time, want string) bool {
		w, _ := time.Parse(time.RFC3339, want)
		return got.Sub(w).Abs() <= 3*time.Minute
	}
	if !ps[0].Rise.Before(now) || !near(ps[0].Set, "2026-09-30T11:59:00Z") {
		t.Errorf("first pass %v - %v, want one under way, setting ~11:59", ps[0].Rise, ps[0].Set)
	}
	if !near(ps[1].Rise, "2026-09-30T18:33:00Z") {
		t.Errorf("second pass rises %v, want ~18:33", ps[1].Rise)
	}
}

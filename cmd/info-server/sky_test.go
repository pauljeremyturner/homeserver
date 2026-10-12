package main

import (
	"testing"
	"time"
)

func TestSkyUpdate(t *testing.T) {
	const lat, lon = 55.8519, -4.2344
	afternoon := time.Date(2026, 10, 12, 15, 0, 0, 0, time.UTC)
	u := skyUpdate(afternoon, lat, lon)
	if !u.Dark || u.ViewUnix != u.DuskUnix+3600 || u.DawnUnix <= u.DuskUnix {
		t.Fatalf("October afternoon: dark %v, dusk %d, view %d, dawn %d", u.Dark, u.DuskUnix, u.ViewUnix, u.DawnUnix)
	}
	byName := map[string]int{}
	for i, c := range u.Constellations {
		byName[c.Name] = i
	}
	// An hour after dusk in mid-October, Cassiopeia is high in the north-east
	// and Orion hasn't risen; it's best in the small hours.
	cas := u.Constellations[byName["Cassiopeia"]]
	if cas.Centre.AltDeg < 40 || len(cas.Lines) == 0 {
		t.Errorf("Cassiopeia at %v with %d lines", cas.Centre, len(cas.Lines))
	}
	ori := u.Constellations[byName["Orion"]]
	if ori.Centre.AltDeg > 0 || !ori.Rising || len(ori.Lines) != 0 {
		t.Errorf("Orion at %v, rising %v, %d lines", ori.Centre, ori.Rising, len(ori.Lines))
	}
	if best := time.Unix(ori.BestUnix, 0); best.Before(time.Date(2026, 10, 13, 3, 0, 0, 0, time.UTC)) || ori.Best.AltDeg < 30 {
		t.Errorf("Orion best at %v, alt %v", best, ori.Best.AltDeg)
	}
	for _, c := range u.Constellations {
		for _, p := range c.Lines {
			for _, a := range p.AltDeg {
				if a < 0 {
					t.Fatalf("%s has a line below the horizon", c.Name)
				}
			}
		}
	}

	if u := skyUpdate(time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), lat, lon); u.Dark || len(u.Constellations) != 0 {
		t.Errorf("midsummer: dark %v, %d constellations", u.Dark, len(u.Constellations))
	}
}

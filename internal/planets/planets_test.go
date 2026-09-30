package planets

import (
	"math"
	"testing"
	"time"
)

// Heliocentric ecliptic (J2000) longitudes and distances from JPL Horizons
// (vectors, centre 500@10), Mercury first; Earth is the Earth-Moon
// barycentre and the giants their system barycentres, as in the elements.
var horizons = []struct {
	date string
	lon  [8]float64
	dist [8]float64
}{
	{"2000-01-01",
		[8]float64{252.412, 181.795, 99.871, 359.135, 36.249, 45.704, 316.413, 303.926},
		[8]float64{0.466, 0.719, 0.983, 1.391, 4.964, 9.177, 19.923, 30.120}},
	{"2026-09-27",
		[8]float64{256.305, 347.134, 3.543, 83.314, 130.810, 10.666, 62.608, 2.624},
		[8]float64{0.466, 0.726, 1.002, 1.553, 5.305, 9.427, 19.442, 29.870}},
	{"2045-06-15",
		[8]float64{175.094, 139.821, 263.695, 65.856, 329.869, 248.358, 145.766, 44.157},
		[8]float64{0.379, 0.717, 1.016, 1.509, 5.022, 9.997, 18.353, 29.797}},
}

func TestAtMatchesHorizons(t *testing.T) {
	for _, h := range horizons {
		at, err := time.Parse(time.DateOnly, h.date)
		if err != nil {
			t.Fatal(err)
		}
		got := At(at)
		if len(got) != 8 {
			t.Fatalf("%s: got %d planets, want 8", h.date, len(got))
		}
		for i, p := range got {
			if d := math.Abs(math.Remainder(p.Longitude-h.lon[i], 360)); d > 1 {
				t.Errorf("%s %s: longitude %.3f, Horizons %.3f (off by %.3f deg)", h.date, p.Name, p.Longitude, h.lon[i], d)
			}
			if d := math.Abs(p.DistanceAU-h.dist[i]) / h.dist[i]; d > 0.01 {
				t.Errorf("%s %s: distance %.3f AU, Horizons %.3f", h.date, p.Name, p.DistanceAU, h.dist[i])
			}
		}
	}
}

func TestOrbitsPassThroughPlanets(t *testing.T) {
	at := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	orbits := Orbits(at, 3600)
	for i, p := range At(at) {
		if math.Abs(math.Hypot(p.X, p.Y)-p.DistanceAU) > 1e-9 {
			t.Errorf("%s: X/Y %.3f,%.3f don't match distance %.3f", p.Name, p.X, p.Y, p.DistanceAU)
		}
		// The planet lies on its orbit, to within the spacing of the points.
		best := math.Inf(1)
		for _, o := range orbits[i] {
			best = min(best, math.Hypot(o.X-p.X, o.Y-p.Y))
		}
		if best > p.DistanceAU*0.002 {
			t.Errorf("%s: %.4f AU from its orbit", p.Name, best)
		}
	}
	// Mercury's orbit is visibly off-centre: perihelion ~0.31 AU, aphelion ~0.47.
	peri, aph := math.Hypot(orbits[0][0].X, orbits[0][0].Y), math.Hypot(orbits[0][1800].X, orbits[0][1800].Y)
	if math.Abs(peri-0.3075) > 0.001 || math.Abs(aph-0.4667) > 0.001 {
		t.Errorf("Mercury perihelion %.4f, aphelion %.4f AU", peri, aph)
	}
}

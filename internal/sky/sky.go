// Package sky works out where the stars and the Sun are as seen from a place
// on Earth: tonight's dark hours, and the prominent constellations' lines as
// altitude and azimuth, clipped at the horizon. Positions use the
// Astronomical Almanac's low-precision formulae (the Sun to ~0.01 degrees),
// far finer than a chart needs. Stars are J2000; precession since moves them
// under half a degree, which a chart this size doesn't show.
//
// The constellation lines, centres and prominence ranks are from Olaf
// Frohn's d3-celestial (BSD licence, LICENSE.d3-celestial), ranks 1 and 2.
package sky

import (
	_ "embed"
	"encoding/json"
	"math"
	"sync"
	"time"
)

// Darkness is how far (degrees) the Sun must be below the horizon for the
// constellations to be worth looking for: nautical twilight's end, when
// the bright stars and the shapes they make are out.
const Darkness = -12.0

// Equatorial is a place on the sky in degrees: right ascension (0 to 360)
// and declination.
type Equatorial struct{ RA, Dec float64 }

// Horizontal is a direction from an observer in degrees: altitude above the
// horizon, and azimuth from north through east.
type Horizontal struct{ Alt, Az float64 }

// Constellation is one constellation's stick figure.
type Constellation struct {
	Abbr, Name string
	Rank       int // 1 the most prominent, 2 the next
	Centre     Equatorial
	Lines      [][]Equatorial
}

//go:embed constellations.json
var constellationsJSON []byte

var (
	loadOnce       sync.Once
	constellations []Constellation
)

// Constellations returns the prominent constellations, most prominent
// first.
func Constellations() []Constellation {
	loadOnce.Do(func() {
		var raw []struct {
			Abbr, Name string
			Rank       int
			Centre     [2]float64
			Lines      [][][2]float64
		}
		if err := json.Unmarshal(constellationsJSON, &raw); err != nil {
			panic("sky: embedded constellations: " + err.Error())
		}
		for _, r := range raw {
			c := Constellation{Abbr: r.Abbr, Name: r.Name, Rank: r.Rank, Centre: Equatorial{r.Centre[0], r.Centre[1]}}
			for _, l := range r.Lines {
				line := make([]Equatorial, len(l))
				for i, p := range l {
					line[i] = Equatorial{p[0], p[1]}
				}
				c.Lines = append(c.Lines, line)
			}
			constellations = append(constellations, c)
		}
	})
	return constellations
}

// ToHorizontal is where e is seen from latitude lat, longitude lon (degrees,
// east positive) at t. Refraction is left out: it lifts things near the
// horizon by about half a degree.
func ToHorizontal(e Equatorial, lat, lon float64, t time.Time) Horizontal {
	h := rad(siderealDeg(t) + lon - e.RA)
	phi, dec := rad(lat), rad(e.Dec)
	alt := math.Asin(math.Sin(phi)*math.Sin(dec) + math.Cos(phi)*math.Cos(dec)*math.Cos(h))
	az := math.Atan2(-math.Cos(dec)*math.Sin(h), math.Cos(phi)*math.Sin(dec)-math.Sin(phi)*math.Cos(dec)*math.Cos(h))
	return Horizontal{deg(alt), math.Mod(deg(az)+360, 360)}
}

// FromEcliptic converts ecliptic longitude and latitude (degrees, of date)
// to Equatorial.
func FromEcliptic(lon, lat float64, t time.Time) Equatorial {
	l, b, eps := rad(lon), rad(lat), rad(23.4393-0.0130*centuries(t))
	ra := math.Atan2(math.Sin(l)*math.Cos(eps)-math.Tan(b)*math.Sin(eps), math.Cos(l))
	dec := math.Asin(math.Sin(b)*math.Cos(eps) + math.Cos(b)*math.Sin(eps)*math.Sin(l))
	return Equatorial{math.Mod(deg(ra)+360, 360), deg(dec)}
}

// Sun is where the Sun is on the sky at t.
func Sun(t time.Time) Equatorial {
	n := julianDay(t) - 2451545.0
	g := rad(357.528 + 0.9856003*n)
	lambda := 280.460 + 0.9856474*n + 1.915*math.Sin(g) + 0.020*math.Sin(2*g)
	return FromEcliptic(lambda, 0, t)
}

// SunAltitude is the Sun's altitude in degrees from lat, lon at t.
func SunAltitude(lat, lon float64, t time.Time) float64 {
	return ToHorizontal(Sun(t), lat, lon, t).Alt
}

// nightStep is how finely the Sun's altitude is sampled for dusk and dawn.
const nightStep = 5 * time.Minute

// Night is the dark spell (the Sun below Darkness) that t falls in, or else
// the next to start within a day: its dusk and dawn, to a few seconds. ok is
// false if there's none, as around midsummer at high latitudes, where the
// Sun never gets that low.
func Night(lat, lon float64, t time.Time) (dusk, dawn time.Time, ok bool) {
	dark := func(at time.Time) bool { return SunAltitude(lat, lon, at) < Darkness }
	edge := func(a, b time.Time) time.Time { // dark changes between a and b
		was := dark(a)
		for b.Sub(a) > 5*time.Second {
			mid := a.Add(b.Sub(a) / 2)
			if dark(mid) == was {
				a = mid
			} else {
				b = mid
			}
		}
		return a.Add(b.Sub(a) / 2).Truncate(time.Second)
	}

	// inside is a time known to be in the night, to search out from.
	inside := t
	for !dark(inside) {
		inside = inside.Add(nightStep)
		if inside.Sub(t) > 24*time.Hour {
			return time.Time{}, time.Time{}, false
		}
	}
	dusk = inside
	for dark(dusk.Add(-nightStep)) {
		dusk = dusk.Add(-nightStep)
		if inside.Sub(dusk) > 24*time.Hour {
			return time.Time{}, time.Time{}, false // polar night
		}
	}
	dawn = inside
	for dark(dawn.Add(nightStep)) {
		dawn = dawn.Add(nightStep)
		if dawn.Sub(inside) > 24*time.Hour {
			return time.Time{}, time.Time{}, false
		}
	}
	return edge(dusk.Add(-nightStep), dusk), edge(dawn, dawn.Add(nightStep)), true
}

// AboveHorizon splits a line through points (in order) into the pieces
// above the horizon as seen from lat, lon at t, each ending exactly on the
// horizon where it crosses it.
func AboveHorizon(points []Equatorial, lat, lon float64, t time.Time) [][]Horizontal {
	var pieces [][]Horizontal
	var cur []Horizontal
	var prev Horizontal
	for i, p := range points {
		h := ToHorizontal(p, lat, lon, t)
		if i > 0 && (prev.Alt >= 0) != (h.Alt >= 0) {
			edge := horizonCrossing(prev, h)
			if h.Alt >= 0 {
				cur = []Horizontal{edge}
			} else {
				cur = append(cur, edge)
				if len(cur) > 1 {
					pieces = append(pieces, cur)
				}
				cur = nil
			}
		}
		if h.Alt >= 0 {
			cur = append(cur, h)
		}
		prev = h
	}
	if len(cur) > 1 {
		pieces = append(pieces, cur)
	}
	return pieces
}

// horizonCrossing is where the stretch from a to b (one above the horizon,
// one below; a constellation line, so only a few degrees long) meets it.
func horizonCrossing(a, b Horizontal) Horizontal {
	f := a.Alt / (a.Alt - b.Alt)
	daz := math.Remainder(b.Az-a.Az, 360)
	return Horizontal{0, math.Mod(a.Az+f*daz+360, 360)}
}

// siderealDeg is Greenwich mean sidereal time at t, in degrees (Meeus 12.4).
func siderealDeg(t time.Time) float64 {
	d := julianDay(t) - 2451545.0
	T := d / 36525
	return math.Mod(280.46061837+360.98564736629*d+0.000387933*T*T, 360)
}

func julianDay(t time.Time) float64 { return float64(t.UTC().UnixNano())/86400e9 + 2440587.5 }

// centuries is Julian centuries since J2000.
func centuries(t time.Time) float64 { return (julianDay(t) - 2451545.0) / 36525 }

func rad(d float64) float64 { return d * math.Pi / 180 }
func deg(r float64) float64 { return r * 180 / math.Pi }

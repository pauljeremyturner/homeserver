// Package moon works out when the Moon rises and sets, from the Astronomical
// Almanac's low-precision lunar formulae (good to ~0.3 degrees, so rise and
// set times to a minute or two) and the rise/set altitude from Meeus,
// "Astronomical Algorithms", ch. 15.
package moon

import (
	"math"
	"time"
)

// Pass is one stretch of the Moon above the horizon.
type Pass struct {
	Rise, Set time.Time
}

// step is how finely the Moon's altitude is sampled looking for rises and
// sets; it's up or down for hours at a time, so none are missed.
const step = 10 * time.Minute

// Passes returns the Moon's passes at latitude lat, longitude lon (degrees,
// east positive) whose set is after from and rise before to: the one it's
// part way through at from, if any, then each rising before to. The search
// looks back up to two days for the first rise; a pass it can't find both
// ends of (only possible near the poles) is left out.
func Passes(lat, lon float64, from, to time.Time) []Pass {
	start := from.Add(-48 * time.Hour)
	end := to.Add(48 * time.Hour)
	var passes []Pass
	var rise time.Time
	prev := above(lat, lon, start)
	for t := start.Add(step); !t.After(end); t = t.Add(step) {
		now := above(lat, lon, t)
		if (prev > 0) == (now > 0) {
			prev = now
			continue
		}
		at := crossing(lat, lon, t.Add(-step), t)
		if now > 0 {
			rise = at
		} else if !rise.IsZero() {
			if at.After(from) && rise.Before(to) {
				passes = append(passes, Pass{rise, at})
			}
			rise = time.Time{}
		}
		prev = now
	}
	return passes
}

// crossing narrows a rise or set known to fall between a and b to under
// ten seconds.
func crossing(lat, lon float64, a, b time.Time) time.Time {
	up := above(lat, lon, a) > 0
	for b.Sub(a) > 10*time.Second {
		mid := a.Add(b.Sub(a) / 2)
		if (above(lat, lon, mid) > 0) == up {
			a = mid
		} else {
			b = mid
		}
	}
	return a.Add(b.Sub(a) / 2).Truncate(time.Second)
}

// above is how far (degrees) the Moon's centre is above the altitude at
// which it rises or sets as seen from lat, lon at t: positive while it's up.
func above(lat, lon float64, t time.Time) float64 {
	ra, dec, parallax := position(t)
	// Meeus 15: the standard altitude for the Moon allows for its parallax,
	// semi-diameter and atmospheric refraction.
	h0 := 0.7275*parallax - 0.5667
	lst := rad(siderealDeg(t) + lon)
	phi := rad(lat)
	sinH := math.Sin(phi)*math.Sin(dec) + math.Cos(phi)*math.Cos(dec)*math.Cos(lst-ra)
	return deg(math.Asin(sinH)) - h0
}

// position is the Moon's geocentric right ascension and declination
// (radians) and its horizontal parallax (degrees) at t, per the
// Astronomical Almanac's low-precision formulae.
func position(t time.Time) (ra, dec, parallax float64) {
	T := centuries(t)
	s := func(a, b float64) float64 { return math.Sin(rad(a + b*T)) }
	c := func(a, b float64) float64 { return math.Cos(rad(a + b*T)) }
	lambda := 218.32 + 481267.881*T +
		6.29*s(135.0, 477198.87) - 1.27*s(259.3, -413335.36) + 0.66*s(235.7, 890534.22) +
		0.21*s(269.9, 954397.74) - 0.19*s(357.5, 35999.05) - 0.11*s(186.5, 966404.03)
	beta := 5.13*s(93.3, 483202.02) + 0.28*s(228.2, 960400.89) -
		0.28*s(318.3, 6003.15) - 0.17*s(217.6, -407332.21)
	parallax = 0.9508 + 0.0518*c(135.0, 477198.87) + 0.0095*c(259.3, -413335.36) +
		0.0078*c(235.7, 890534.22) + 0.0028*c(269.9, 954397.74)

	l, b, eps := rad(lambda), rad(beta), rad(23.4393-0.0130*T)
	ra = math.Atan2(math.Sin(l)*math.Cos(eps)-math.Tan(b)*math.Sin(eps), math.Cos(l))
	dec = math.Asin(math.Sin(b)*math.Cos(eps) + math.Cos(b)*math.Sin(eps)*math.Sin(l))
	return ra, dec, parallax
}

// siderealDeg is Greenwich mean sidereal time at t, in degrees (Meeus 12.4).
func siderealDeg(t time.Time) float64 {
	d := julianDay(t) - 2451545.0
	T := d / 36525
	return math.Mod(280.46061837+360.98564736629*d+0.000387933*T*T, 360)
}

func julianDay(t time.Time) float64 { return float64(t.UTC().UnixNano())/86400e9 + 2440587.5 }

// centuries is Julian centuries since J2000 (TT; the ~1 minute TT-UTC gap
// is well within the formulae's accuracy).
func centuries(t time.Time) float64 { return (julianDay(t) - 2451545.0) / 36525 }

func rad(d float64) float64 { return d * math.Pi / 180 }
func deg(r float64) float64 { return r * 180 / math.Pi }

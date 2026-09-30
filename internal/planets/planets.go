// Package planets computes approximate heliocentric positions of the eight
// planets from JPL's Keplerian elements (E.M. Standish, "Keplerian Elements
// for Approximate Positions of the Major Planets", Table 1, valid 1800-2050),
// good to well under a degree, which is plenty for a schematic.
package planets

import (
	"math"
	"time"
)

// elements are a planet's orbital elements at J2000 and their rates per
// Julian century: semi-major axis (AU), eccentricity, inclination, mean
// longitude, longitude of perihelion and longitude of the ascending node
// (degrees).
type elements struct {
	a, e, i, l, peri, node float64
}

type planet struct {
	name       string
	at0, rates elements
}

var table = []planet{
	{"Mercury",
		elements{0.38709927, 0.20563593, 7.00497902, 252.25032350, 77.45779628, 48.33076593},
		elements{0.00000037, 0.00001906, -0.00594749, 149472.67411175, 0.16047689, -0.12534081}},
	{"Venus",
		elements{0.72333566, 0.00677672, 3.39467605, 181.97909950, 131.60246718, 76.67984255},
		elements{0.00000390, -0.00004107, -0.00078890, 58517.81538729, 0.00268329, -0.27769418}},
	{"Earth", // the Earth-Moon barycentre
		elements{1.00000261, 0.01671123, -0.00001531, 100.46457166, 102.93768193, 0},
		elements{0.00000562, -0.00004392, -0.01294668, 35999.37244981, 0.32327364, 0}},
	{"Mars",
		elements{1.52371034, 0.09339410, 1.84969142, -4.55343205, -23.94362959, 49.55953891},
		elements{0.00001847, 0.00007882, -0.00813131, 19140.30268499, 0.44441088, -0.29257343}},
	{"Jupiter",
		elements{5.20288700, 0.04838624, 1.30439695, 34.39644051, 14.72847983, 100.47390909},
		elements{-0.00011607, -0.00013253, -0.00183714, 3034.74612775, 0.21252668, 0.20469106}},
	{"Saturn",
		elements{9.53667594, 0.05386179, 2.48599187, 49.95424423, 92.59887831, 113.66242448},
		elements{-0.00125060, -0.00050991, 0.00193609, 1222.49362201, -0.41897216, -0.28867794}},
	{"Uranus",
		elements{19.18916464, 0.04725744, 0.77263783, 313.23810451, 170.95427630, 74.01692503},
		elements{-0.00196176, -0.00004397, -0.00242939, 428.48202785, 0.40805281, 0.04240589}},
	{"Neptune",
		elements{30.06992276, 0.00859048, 1.77004347, -55.12002969, 44.96476227, 131.78422574},
		elements{0.00026291, 0.00005105, 0.00035372, 218.45945325, -0.32241464, -0.00508664}},
}

// Position is a planet's place around the Sun.
type Position struct {
	Name string
	// Longitude is the heliocentric ecliptic longitude in degrees [0, 360),
	// measured anticlockwise (seen from the north) from the March equinox.
	Longitude float64
	// DistanceAU is the distance from the Sun.
	DistanceAU float64
	// X and Y place the planet in the ecliptic plane, in AU from the Sun: X
	// towards the March equinox, Y 90 degrees anticlockwise from it seen
	// from the north.
	X, Y float64
}

// Point is a place in the ecliptic plane, in AU, as Position's X and Y.
type Point struct{ X, Y float64 }

// orbit is a planet's orbit at a given time.
type orbit struct {
	a, e      float64
	m         float64 // mean anomaly, radians
	w, o, inc float64 // argument of perihelion, ascending node, inclination, radians
}

func orbitAt(p planet, t time.Time) orbit {
	// Julian centuries since J2000 (TT; the ~1 minute TT-UTC gap is
	// irrelevant here).
	jd := float64(t.UTC().UnixNano())/86400e9 + 2440587.5
	T := (jd - 2451545.0) / 36525
	el := func(v0, rate float64) float64 { return v0 + rate*T }
	l, peri, node := el(p.at0.l, p.rates.l), el(p.at0.peri, p.rates.peri), el(p.at0.node, p.rates.node)
	return orbit{
		a:   el(p.at0.a, p.rates.a),
		e:   el(p.at0.e, p.rates.e),
		m:   rad(math.Remainder(l-peri, 360)),
		w:   rad(peri - node),
		o:   rad(node),
		inc: rad(el(p.at0.i, p.rates.i)),
	}
}

// point is where on the orbit the eccentric anomaly E is, projected onto
// the ecliptic: in the orbital plane, then rotated into ecliptic x/y.
func (b orbit) point(E float64) Point {
	xp, yp := b.a*(math.Cos(E)-b.e), b.a*math.Sqrt(1-b.e*b.e)*math.Sin(E)
	w, o, inc := b.w, b.o, b.inc
	x := (math.Cos(w)*math.Cos(o)-math.Sin(w)*math.Sin(o)*math.Cos(inc))*xp +
		(-math.Sin(w)*math.Cos(o)-math.Cos(w)*math.Sin(o)*math.Cos(inc))*yp
	y := (math.Cos(w)*math.Sin(o)+math.Sin(w)*math.Cos(o)*math.Cos(inc))*xp +
		(-math.Sin(w)*math.Sin(o)+math.Cos(w)*math.Cos(o)*math.Cos(inc))*yp
	return Point{x, y}
}

// At returns the eight planets' positions at t, Mercury first.
func At(t time.Time) []Position {
	out := make([]Position, len(table))
	for n, p := range table {
		b := orbitAt(p, t)
		E := b.m
		for range 10 {
			E -= (E - b.e*math.Sin(E) - b.m) / (1 - b.e*math.Cos(E))
		}
		pt := b.point(E)
		lon := math.Mod(math.Atan2(pt.Y, pt.X)*180/math.Pi+360, 360)
		out[n] = Position{Name: p.name, Longitude: lon, DistanceAU: math.Hypot(pt.X, pt.Y), X: pt.X, Y: pt.Y}
	}
	return out
}

// Orbits returns the eight planets' orbits at t, Mercury first, each as n
// points around the ellipse (projected onto the ecliptic), starting at
// perihelion.
func Orbits(t time.Time, n int) [][]Point {
	out := make([][]Point, len(table))
	for i, p := range table {
		b := orbitAt(p, t)
		out[i] = make([]Point, n)
		for k := range n {
			out[i][k] = b.point(2 * math.Pi * float64(k) / float64(n))
		}
	}
	return out
}

func rad(d float64) float64 { return d * math.Pi / 180 }

// History returns each planet's positions (Mercury first) at frames+1 evenly
// spaced times from span before end up to end itself, with the first time
// and the spacing.
func History(end time.Time, span time.Duration, frames int) (start time.Time, step time.Duration, paths [][]Point) {
	step = span / time.Duration(frames)
	start = end.Add(-step * time.Duration(frames))
	paths = make([][]Point, len(table))
	for k := 0; k <= frames; k++ {
		for i, p := range At(start.Add(step * time.Duration(k))) {
			paths[i] = append(paths[i], Point{p.X, p.Y})
		}
	}
	return start, step, paths
}

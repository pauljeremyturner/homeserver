package sky

import (
	"math"
	"testing"
	"time"
)

const glasgowLat, glasgowLon = 55.8519, -4.2344

var london = func() *time.Location {
	l, err := time.LoadLocation("Europe/London")
	if err != nil {
		panic(err)
	}
	return l
}()

func TestPolarisAltitudeIsLatitude(t *testing.T) {
	polaris := Equatorial{37.95, 89.264}
	for h := range 24 {
		at := time.Date(2026, 10, 12, h, 0, 0, 0, time.UTC)
		p := ToHorizontal(polaris, glasgowLat, glasgowLon, at)
		if math.Abs(p.Alt-glasgowLat) > 1 || math.Abs(math.Remainder(p.Az, 360)) > 2 {
			t.Errorf("%v: Polaris at alt %.2f az %.2f, want ~%.1f due north", at, p.Alt, p.Az, glasgowLat)
		}
	}
}

func TestSunAtEquinoxNoon(t *testing.T) {
	// Solar noon at Glasgow on the 2026 March equinox (20 March) is ~12:24
	// UTC; the Sun is then due south at 90 - latitude.
	at := time.Date(2026, 3, 20, 12, 24, 0, 0, time.UTC)
	s := ToHorizontal(Sun(at), glasgowLat, glasgowLon, at)
	if math.Abs(s.Alt-(90-glasgowLat)) > 1 || math.Abs(s.Az-180) > 3 {
		t.Errorf("equinox noon Sun at alt %.2f az %.2f, want ~%.1f due south", s.Alt, s.Az, 90-glasgowLat)
	}
}

func TestNight(t *testing.T) {
	// 12 October 2026: sunset 18:24, sunrise 07:43 BST; nautical dusk and
	// dawn come about 75 minutes either side.
	afternoon := time.Date(2026, 10, 12, 16, 0, 0, 0, london)
	dusk, dawn, ok := Night(glasgowLat, glasgowLon, afternoon)
	if !ok {
		t.Fatal("no night in October")
	}
	if want := time.Date(2026, 10, 12, 19, 40, 0, 0, london); dusk.Sub(want).Abs() > 15*time.Minute {
		t.Errorf("dusk %v, want about %v", dusk.In(london), want)
	}
	if want := time.Date(2026, 10, 13, 6, 25, 0, 0, london); dawn.Sub(want).Abs() > 15*time.Minute {
		t.Errorf("dawn %v, want about %v", dawn.In(london), want)
	}

	// From the middle of that night it's the same night.
	d2, w2, ok := Night(glasgowLat, glasgowLon, time.Date(2026, 10, 13, 1, 0, 0, 0, london))
	if !ok || d2.Sub(dusk).Abs() > time.Minute || w2.Sub(dawn).Abs() > time.Minute {
		t.Errorf("from 01:00: %v to %v, want %v to %v", d2, w2, dusk, dawn)
	}

	// Midsummer: the Sun only gets to about -10.7 degrees.
	if d, w, ok := Night(glasgowLat, glasgowLon, time.Date(2026, 6, 21, 12, 0, 0, 0, london)); ok {
		t.Errorf("midsummer night %v to %v, want none", d, w)
	}
}

func TestAboveHorizon(t *testing.T) {
	// A line down the meridian through the south point, from 20 degrees up
	// to 20 degrees below: one piece, ending on the horizon due south.
	at := time.Date(2026, 10, 12, 22, 0, 0, 0, time.UTC)
	lst := math.Mod(siderealDeg(at)+glasgowLon+360, 360)
	southDec := glasgowLat - 90 // the south point's declination
	var line []Equatorial
	for d := 20.0; d >= -20; d -= 5 {
		line = append(line, Equatorial{lst, southDec + d})
	}
	pieces := AboveHorizon(line, glasgowLat, glasgowLon, at)
	if len(pieces) != 1 {
		t.Fatalf("got %d pieces, want 1", len(pieces))
	}
	end := pieces[0][len(pieces[0])-1]
	if math.Abs(end.Alt) > 1e-6 || math.Abs(end.Az-180) > 0.5 {
		t.Errorf("ends at alt %.3f az %.2f, want the horizon due south", end.Alt, end.Az)
	}
	if first := pieces[0][0]; math.Abs(first.Alt-20) > 0.5 {
		t.Errorf("starts at alt %.2f, want 20", first.Alt)
	}
}

func TestConstellations(t *testing.T) {
	cs := Constellations()
	if len(cs) != 46 {
		t.Errorf("got %d constellations, want 46", len(cs))
	}
	var orion *Constellation
	for i := range cs {
		if cs[i].Name == "Orion" {
			orion = &cs[i]
		}
	}
	if orion == nil || orion.Rank != 1 || len(orion.Lines) == 0 {
		t.Fatalf("Orion: %+v", orion)
	}
	// Orion is around RA 5h30m (82 degrees), on the celestial equator; its
	// label point sits a little above the figure.
	if math.Abs(orion.Centre.RA-83) > 10 || math.Abs(orion.Centre.Dec) > 15 {
		t.Errorf("Orion's centre %+v", orion.Centre)
	}
}

package main

import (
	"log"
	"math"
	"strconv"
	"time"

	skypb "homeserver/gen/sky"
	"homeserver/internal/broadcast"
	"homeserver/internal/moon"
	"homeserver/internal/planets"
	"homeserver/internal/sky"
)

// skyInterval is how often the sky is worked out again: the stars turn 2.5
// degrees in that time.
const skyInterval = 10 * time.Minute

// skyPlanets are the planets bright enough to pick out by eye. Mercury is
// too, but only ever low in twilight.
var skyPlanets = map[string]bool{"Venus": true, "Mars": true, "Jupiter": true, "Saturn": true}

// skyCoords is where the sky is seen from: LOCATION if set, else wherever
// the weather is for, once wttr.in has said.
func skyCoords(loc *location) (lat, lon float64, ok bool) {
	if loc != nil {
		la, errLa := strconv.ParseFloat(loc.lat, 64)
		lo, errLo := strconv.ParseFloat(loc.lon, 64)
		if errLa == nil && errLo == nil {
			return la, lo, true
		}
	}
	if c := weatherCoords.Load(); c != nil {
		return c[0], c[1], true
	}
	return 0, 0, false
}

// darkTonight reports whether tonight (or, by day, the coming night) gets
// dark enough to see the constellations from loc; false while loc isn't
// known yet.
func darkTonight(loc *location, t time.Time) bool {
	lat, lon, ok := skyCoords(loc)
	if !ok {
		return false
	}
	_, _, dark := sky.Night(lat, lon, t)
	return dark
}

// skyUpdate is the night sky from lat, lon: now if it's dark at t, else an
// hour after the coming dusk.
func skyUpdate(t time.Time, lat, lon float64) *skypb.SkyUpdate {
	u := &skypb.SkyUpdate{ComputedAtUnix: t.Unix()}
	dusk, dawn, ok := sky.Night(lat, lon, t)
	if !ok {
		return u
	}
	view := t
	if t.Before(dusk) {
		view = dusk.Add(min(time.Hour, dawn.Sub(dusk)/2))
	}
	u.Dark, u.DuskUnix, u.DawnUnix, u.ViewUnix = true, dusk.Unix(), dawn.Unix(), view.Unix()

	for _, c := range sky.Constellations() {
		sc := &skypb.Constellation{Name: c.Name, Rank: int32(c.Rank)}
		for _, line := range c.Lines {
			for _, piece := range sky.AboveHorizon(line, lat, lon, view) {
				p := &skypb.Path{}
				for _, h := range piece {
					p.AltDeg = append(p.AltDeg, float32(h.Alt))
					p.AzDeg = append(p.AzDeg, float32(h.Az))
				}
				sc.Lines = append(sc.Lines, p)
			}
		}
		centre := sky.ToHorizontal(c.Centre, lat, lon, view)
		sc.Centre = skyPoint(centre)
		sc.Rising = sky.ToHorizontal(c.Centre, lat, lon, view.Add(30*time.Minute)).Alt > centre.Alt
		best, bestAt := sky.Horizontal{Alt: math.Inf(-1)}, dusk
		for at := dusk; !at.After(dawn); at = at.Add(10 * time.Minute) {
			if h := sky.ToHorizontal(c.Centre, lat, lon, at); h.Alt > best.Alt {
				best, bestAt = h, at
			}
		}
		sc.Best, sc.BestUnix = skyPoint(best), bestAt.Unix()
		u.Constellations = append(u.Constellations, sc)
	}

	ra, dec := moon.Position(view)
	if h := sky.ToHorizontal(sky.Equatorial{RA: ra, Dec: dec}, lat, lon, view); h.Alt > 0 {
		u.Bodies = append(u.Bodies, &skypb.Body{Name: "Moon", At: skyPoint(h)})
	}
	// The planets as seen from Earth, in the ecliptic plane (they stray a
	// few degrees from it at most).
	ps := planets.At(view)
	var earth planets.Position
	for _, p := range ps {
		if p.Name == "Earth" {
			earth = p
		}
	}
	for _, p := range ps {
		if !skyPlanets[p.Name] {
			continue
		}
		lonDeg := math.Atan2(p.Y-earth.Y, p.X-earth.X) * 180 / math.Pi
		if h := sky.ToHorizontal(sky.FromEcliptic(lonDeg, 0, view), lat, lon, view); h.Alt > 0 {
			u.Bodies = append(u.Bodies, &skypb.Body{Name: p.Name, At: skyPoint(h)})
		}
	}
	return u
}

func skyPoint(h sky.Horizontal) *skypb.Point {
	return &skypb.Point{AltDeg: float32(h.Alt), AzDeg: float32(h.Az)}
}

// pollSky publishes the sky every skyInterval, once the location is known.
func pollSky(bc *broadcast.Broadcaster[*skypb.SkyUpdate], loc *location) {
	wasDark := true
	for {
		lat, lon, ok := skyCoords(loc)
		if !ok {
			time.Sleep(30 * time.Second)
			continue
		}
		u := skyUpdate(time.Now(), lat, lon)
		if u.Dark != wasDark {
			if u.Dark {
				log.Printf("sky: dark nights again at %.2f,%.2f, showing the sky page", lat, lon)
			} else {
				log.Printf("sky: no dark night at %.2f,%.2f, skipping the sky page", lat, lon)
			}
			wasDark = u.Dark
		}
		bc.Publish(u)
		time.Sleep(skyInterval)
	}
}

type skyServer struct {
	skypb.UnimplementedSkyServiceServer
	bc *broadcast.Broadcaster[*skypb.SkyUpdate]
}

func (s *skyServer) StreamSky(_ *skypb.StreamSkyRequest, stream skypb.SkyService_StreamSkyServer) error {
	return forward(s.bc, stream)
}

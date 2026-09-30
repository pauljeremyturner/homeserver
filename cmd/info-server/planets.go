package main

import (
	"time"

	planetspb "homeserver/gen/planets"
	"homeserver/internal/broadcast"
	"homeserver/internal/planets"
)

const planetsInterval = time.Hour

// historyFrames is how many steps the history takes whatever its span: at
// ~30fps over the ~40s a display animates it, that's a frame or two per
// step, so displays only interpolate a little between them.
const historyFrames = 1800

// orbitPoints is how many points each orbit's shape is drawn with.
const orbitPoints = 180

func planetsUpdate(t time.Time, historyYears int) *planetspb.PlanetsUpdate {
	u := &planetspb.PlanetsUpdate{ComputedAtUnix: t.Unix()}
	for _, p := range planets.At(t) {
		u.Planets = append(u.Planets, &planetspb.Planet{
			Name:         p.Name,
			LongitudeDeg: p.Longitude,
			DistanceAu:   p.DistanceAU,
			XAu:          p.X,
			YAu:          p.Y,
		})
	}
	for _, o := range planets.Orbits(t, orbitPoints) {
		u.Orbits = append(u.Orbits, path(o))
	}
	span := time.Duration(float64(historyYears) * 365.25 * 24 * float64(time.Hour))
	start, step, history := planets.History(t, span, historyFrames)
	u.HistoryStartUnix, u.HistoryStepSeconds = start.Unix(), int64(step/time.Second)
	for _, h := range history {
		u.History = append(u.History, path(h))
	}
	return u
}

func path(pts []planets.Point) *planetspb.Path {
	p := &planetspb.Path{XAu: make([]float32, len(pts)), YAu: make([]float32, len(pts))}
	for i, pt := range pts {
		p.XAu[i], p.YAu[i] = float32(pt.X), float32(pt.Y)
	}
	return p
}

func pollPlanets(bc *broadcast.Broadcaster[*planetspb.PlanetsUpdate], historyYears int) {
	bc.Publish(planetsUpdate(time.Now(), historyYears))
	for t := range time.Tick(planetsInterval) {
		bc.Publish(planetsUpdate(t, historyYears))
	}
}

type planetServer struct {
	planetspb.UnimplementedPlanetServiceServer
	bc *broadcast.Broadcaster[*planetspb.PlanetsUpdate]
}

func (s *planetServer) StreamPlanets(_ *planetspb.StreamPlanetsRequest, stream planetspb.PlanetService_StreamPlanetsServer) error {
	return forward(s.bc, stream)
}

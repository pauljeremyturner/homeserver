package main

import (
	"time"

	planetspb "homeserver/gen/planets"
	"homeserver/internal/broadcast"
	"homeserver/internal/planets"
)

const planetsInterval = time.Hour

func planetsUpdate(t time.Time) *planetspb.PlanetsUpdate {
	u := &planetspb.PlanetsUpdate{ComputedAtUnix: t.Unix()}
	for _, p := range planets.At(t) {
		u.Planets = append(u.Planets, &planetspb.Planet{
			Name:         p.Name,
			LongitudeDeg: p.Longitude,
			DistanceAu:   p.DistanceAU,
		})
	}
	return u
}

func pollPlanets(bc *broadcast.Broadcaster[*planetspb.PlanetsUpdate]) {
	bc.Publish(planetsUpdate(time.Now()))
	for t := range time.Tick(planetsInterval) {
		bc.Publish(planetsUpdate(t))
	}
}

type planetServer struct {
	planetspb.UnimplementedPlanetServiceServer
	bc *broadcast.Broadcaster[*planetspb.PlanetsUpdate]
}

func (s *planetServer) StreamPlanets(_ *planetspb.StreamPlanetsRequest, stream planetspb.PlanetService_StreamPlanetsServer) error {
	return forward(s.bc, stream)
}

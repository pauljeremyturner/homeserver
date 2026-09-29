package main

import (
	"math"
	"time"

	cryptopb "homeserver/gen/crypto"
	newspb "homeserver/gen/news"
	planetspb "homeserver/gen/planets"
	weatherpb "homeserver/gen/weather"
	"homeserver/internal/planets"
)

// fillDemo loads sample data (-demo), for working on the layout without an
// info-server.
func fillDemo(s *state) {
	now := time.Now().Unix()
	s.update(func(s *state) {
		s.weather = &weatherpb.WeatherUpdate{
			FetchedAtUnix:        now,
			Location:             "Glasgow, United Kingdom",
			Timezone:             "Europe/London",
			CurrentDesc:          "Light rain shower",
			CurrentCategory:      weatherpb.Category_CATEGORY_RAIN,
			TempC:                "12",
			FeelsLikeC:           "10",
			Humidity:             "82",
			WindKmph:             "19",
			WindDir:              "SW",
			WindBeaufort:         "Moderate breeze",
			TodaySunrise:         "07:08 AM",
			TodaySunset:          "07:09 PM",
			MoonPhase:            weatherpb.MoonPhase_MOON_PHASE_WAXING_GIBBOUS,
			MoonIllum:            "71",
			TomorrowSunrise:      "07:10 AM",
			TomorrowDesc:         "Partly cloudy",
			TomorrowCategory:     weatherpb.Category_CATEGORY_PARTLY_CLOUDY,
			TomorrowMaxC:         "15",
			TomorrowMinC:         "8",
			TomorrowWindKmph:     "14",
			TomorrowChanceOfRain: "30",
		}
		s.crypto["BTC"] = demoCoin("BTC", now, 83000, 4.7, 1)
		s.crypto["ETH"] = demoCoin("ETH", now, 3310, -4.3, 2)
		s.crypto["XAU"] = demoCoin("XAU", now, 3190, 1.2, 3)
		s.crypto["XAG"] = demoCoin("XAG", now, 47.9, 2.1, 4)
		s.planets = &planetspb.PlanetsUpdate{ComputedAtUnix: now}
		for _, p := range planets.At(time.Unix(now, 0)) {
			s.planets.Planets = append(s.planets.Planets, &planetspb.Planet{Name: p.Name, LongitudeDeg: p.Longitude, DistanceAu: p.DistanceAU})
		}
		s.wallet = &cryptopb.WalletBalanceUpdate{Label: "BTC Wallet", FetchedAtUnix: now, BalanceBtc: 0.04215, BalanceSats: 4215000}
		s.news = &newspb.NewsUpdate{FetchedAtUnix: now, Headlines: []string{
			"Scottish Parliament backs new rail investment plan",
			"Clyde shipyard wins frigate contract extension",
			"Storm warning issued for west coast this weekend",
		}}
	})
}

// demoCoin makes a week of hourly prices, starting at start and ending
// changePct higher, with some made-up wobble.
func demoCoin(sym string, now int64, start, changePct, seed float64) *cryptopb.CryptoUpdate {
	const hours = 168
	c := &cryptopb.CryptoUpdate{Symbol: sym, FetchedAtUnix: now, ChangePct: changePct}
	for i := 0; i <= hours; i++ {
		f := float64(i) / hours
		p := start * (1 + changePct/100*f + 0.02*math.Sin(f*9+seed) + 0.008*math.Sin(f*53*seed))
		c.Prices = append(c.Prices, p)
		c.TimesUnix = append(c.TimesUnix, now-int64(hours-i)*3600)
	}
	c.Latest = c.Prices[hours]
	c.Min, c.Max = c.Prices[0], c.Prices[0]
	for _, p := range c.Prices {
		c.Min, c.Max = min(c.Min, p), max(c.Max, p)
	}
	return c
}

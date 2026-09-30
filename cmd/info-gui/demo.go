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
			FetchedAtUnix:   now,
			Location:        "Glasgow, United Kingdom",
			Timezone:        "Europe/London",
			CurrentDesc:     "Light rain shower",
			CurrentCategory: weatherpb.Category_CATEGORY_RAIN,
			TempC:           "12",
			FeelsLikeC:      "10",
			Humidity:        "82",
			WindKmph:        "19",
			WindDir:         "SW",
			WindBeaufort:    "Moderate breeze",
			TodaySunrise:    "07:08 AM",
			TodaySunset:     "07:09 PM",
			MoonPhase:       weatherpb.MoonPhase_MOON_PHASE_WAXING_GIBBOUS,
			MoonIllum:       "71",
			TomorrowSunrise: "07:10 AM",
			UvIndex:         "1",
			IsDay:           true,
			Hourly:          demoHours(now),
			Daily:           demoDays(now),
			MoonPasses: []*weatherpb.MoonPass{
				{RiseUnix: now - 4*3600, SetUnix: now + 8*3600},
				{RiseUnix: now + 20*3600, SetUnix: now + 33*3600},
			},
		}
		for _, c := range []struct {
			sym                   string
			start, week, halfYear float64 // price a week ago; % change over each window
		}{
			{"BTC", 83000, 4.7, 18.5},
			{"ETH", 3310, -4.3, -12.1},
			{"XAU", 3190, 1.2, 9.4},
			{"XAG", 47.9, 2.1, 15.8},
		} {
			wk := demoCoin(c.sym, now, c.start, c.week, weekDays)
			s.crypto[weekDays][c.sym] = wk
			// Six months ending at the same price as the week.
			s.crypto[halfYearDays][c.sym] = demoCoin(c.sym, now, wk.Latest/(1+c.halfYear/100), c.halfYear, halfYearDays)
		}
		s.planets = demoPlanets(time.Unix(now, 0))
		s.wallet = &cryptopb.WalletBalanceUpdate{Label: "BTC Wallet", FetchedAtUnix: now, BalanceBtc: 0.04215, BalanceSats: 4215000}
		s.news = &newspb.NewsUpdate{FetchedAtUnix: now, Headlines: []string{
			"Scottish Parliament backs new rail investment plan",
			"Clyde shipyard wins frigate contract extension",
			"Storm warning issued for west coast this weekend",
		}}
	})
}

// demoHours makes a day of hourly forecasts from now's hour, clearing up
// after the rain and cooling into the evening.
func demoHours(now int64) []*weatherpb.HourForecast {
	var hrs []*weatherpb.HourForecast
	start := time.Unix(now, 0).Truncate(time.Hour)
	for i := range 24 {
		t := start.Add(time.Duration(i) * time.Hour)
		cat, rain := weatherpb.Category_CATEGORY_PARTLY_CLOUDY, int32(10)
		switch {
		case i < 2:
			cat, rain = weatherpb.Category_CATEGORY_RAIN, 80-int32(i)*30
		case i < 5:
			cat, rain = weatherpb.Category_CATEGORY_CLOUDY, 25
		case i > 9:
			cat = weatherpb.Category_CATEGORY_SUNNY
		}
		hrs = append(hrs, &weatherpb.HourForecast{
			TimeUnix:     t.Unix(),
			Category:     cat,
			IsDay:        t.Hour() >= 7 && t.Hour() < 19,
			TempC:        int32(math.Round(12 + 3*math.Cos(float64(t.Hour()-15)*math.Pi/12))),
			ChanceOfRain: rain,
		})
	}
	return hrs
}

func demoDays(now int64) []*weatherpb.DayForecast {
	c := func(n int) weatherpb.Category { return weatherpb.Category(n) }
	days := []struct {
		day, night   weatherpb.Category
		lo, hi, rain int32
	}{
		{c(5), c(3), 9, 14, 80},
		{c(2), c(1), 8, 15, 30},
		{c(5), c(5), 10, 16, 95},
		{c(3), c(2), 9, 14, 45},
		{c(6), c(5), 11, 14, 70},
		{c(1), c(1), 7, 16, 5},
		{c(2), c(4), 6, 12, 15},
	}
	var out []*weatherpb.DayForecast
	for i, d := range days {
		out = append(out, &weatherpb.DayForecast{
			Date:          time.Unix(now, 0).AddDate(0, 0, i).Format("2006-01-02"),
			DayCategory:   d.day,
			NightCategory: d.night,
			MinC:          d.lo,
			MaxC:          d.hi,
			ChanceOfRain:  d.rain,
		})
	}
	return out
}

// demoPlanets is what info-server sends for the planets, with 20 years of
// history.
func demoPlanets(t time.Time) *planetspb.PlanetsUpdate {
	u := &planetspb.PlanetsUpdate{ComputedAtUnix: t.Unix()}
	for _, p := range planets.At(t) {
		u.Planets = append(u.Planets, &planetspb.Planet{Name: p.Name, LongitudeDeg: p.Longitude, DistanceAu: p.DistanceAU, XAu: p.X, YAu: p.Y})
	}
	path := func(pts []planets.Point) *planetspb.Path {
		p := &planetspb.Path{}
		for _, pt := range pts {
			p.XAu, p.YAu = append(p.XAu, float32(pt.X)), append(p.YAu, float32(pt.Y))
		}
		return p
	}
	for _, o := range planets.Orbits(t, 180) {
		u.Orbits = append(u.Orbits, path(o))
	}
	start, step, history := planets.History(t, time.Duration(20*365.25*24*float64(time.Hour)), 1800)
	u.HistoryStartUnix, u.HistoryStepSeconds = start.Unix(), int64(step/time.Second)
	for _, h := range history {
		u.History = append(u.History, path(h))
	}
	return u
}

// demoCoin makes days of prices, hourly for a week and daily beyond, as
// CoinGecko sends them, starting at start and ending changePct higher, with
// some made-up wobble.
func demoCoin(sym string, now int64, start, changePct float64, days int32) *cryptopb.CryptoUpdate {
	points, spacing := int(days)*24, int64(3600)
	if days > 90 {
		points, spacing = int(days), 86400
	}
	seed := float64(len(sym)) + float64(sym[0]%7)
	c := &cryptopb.CryptoUpdate{Symbol: sym, FetchedAtUnix: now, ChangePct: changePct, Days: days}
	for i := 0; i <= points; i++ {
		f := float64(i) / float64(points)
		p := start * (1 + changePct/100*f + 0.02*math.Sin(f*9+seed) + 0.008*math.Sin(f*53*seed))
		c.Prices = append(c.Prices, p)
		c.TimesUnix = append(c.TimesUnix, now-int64(points-i)*spacing)
	}
	c.Latest = c.Prices[points]
	c.Min, c.Max = c.Prices[0], c.Prices[0]
	for _, p := range c.Prices {
		c.Min, c.Max = min(c.Min, p), max(c.Max, p)
	}
	return c
}

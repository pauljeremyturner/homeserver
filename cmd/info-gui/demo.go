package main

import (
	"time"

	cryptopb "homeserver/gen/crypto"
	newspb "homeserver/gen/news"
	weatherpb "homeserver/gen/weather"
)

// fillDemo loads sample data (-demo), for working on the layout without an
// info-server.
func fillDemo(s *state) {
	now := time.Now().Unix()
	s.update(func(s *state) {
		s.weather = &weatherpb.WeatherUpdate{
			FetchedAtUnix:        now,
			Location:             "Glasgow, United Kingdom",
			CurrentDesc:          "Light rain shower",
			CurrentCategory:      weatherpb.Category_CATEGORY_RAIN,
			TempC:                "12",
			FeelsLikeC:           "10",
			Humidity:             "82",
			WindKmph:             "19",
			WindDir:              "SW",
			TodaySunrise:         "07:08 AM",
			TodaySunset:          "07:09 PM",
			MoonPhase:            weatherpb.MoonPhase_MOON_PHASE_WAXING_GIBBOUS,
			MoonIllum:            "71",
			TomorrowDesc:         "Partly cloudy",
			TomorrowCategory:     weatherpb.Category_CATEGORY_PARTLY_CLOUDY,
			TomorrowMaxC:         "15",
			TomorrowMinC:         "8",
			TomorrowWindKmph:     "14",
			TomorrowChanceOfRain: "30",
		}
		s.crypto["BTC"] = &cryptopb.CryptoUpdate{
			Symbol: "BTC", FetchedAtUnix: now,
			Prices: []float64{83120, 84010, 82650, 85200, 86330, 85710, 87045},
			Latest: 87045, ChangePct: 4.7, Min: 82650, Max: 87045,
		}
		s.crypto["ETH"] = &cryptopb.CryptoUpdate{
			Symbol: "ETH", FetchedAtUnix: now,
			Prices: []float64{3310, 3275, 3198, 3240, 3150, 3122, 3168},
			Latest: 3168, ChangePct: -4.3, Min: 3122, Max: 3310,
		}
		s.wallet = &cryptopb.WalletBalanceUpdate{Label: "BTC Wallet", FetchedAtUnix: now, BalanceBtc: 0.04215, BalanceSats: 4215000}
		s.news = &newspb.NewsUpdate{FetchedAtUnix: now, Headlines: []string{
			"Scottish Parliament backs new rail investment plan",
			"Clyde shipyard wins frigate contract extension",
			"Storm warning issued for west coast this weekend",
		}}
	})
}

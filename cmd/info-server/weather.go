package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	weatherpb "homeserver/gen/weather"
)

type valueField struct {
	Value string `json:"value"`
}

type currentCondition struct {
	TempC          string       `json:"temp_C"`
	FeelsLikeC     string       `json:"FeelsLikeC"`
	Humidity       string       `json:"humidity"`
	WeatherCode    string       `json:"weatherCode"`
	WeatherDesc    []valueField `json:"weatherDesc"`
	WindspeedKmph  string       `json:"windspeedKmph"`
	Winddir16Point string       `json:"winddir16Point"`
	UvIndex        string       `json:"uvIndex"`
	Visibility     string       `json:"visibility"`
	PrecipMM       string       `json:"precipMM"`
}

type nearestArea struct {
	AreaName []valueField `json:"areaName"`
	Country  []valueField `json:"country"`
	Region   []valueField `json:"region"`
}

type astronomy struct {
	Sunrise          string `json:"sunrise"`
	Sunset           string `json:"sunset"`
	Moonrise         string `json:"moonrise"`
	Moonset          string `json:"moonset"`
	MoonPhase        string `json:"moon_phase"`
	MoonIllumination string `json:"moon_illumination"`
}

type hourEntry struct {
	Time          string       `json:"time"`
	WeatherCode   string       `json:"weatherCode"`
	WeatherDesc   []valueField `json:"weatherDesc"`
	TempC         string       `json:"tempC"`
	ChanceOfRain  string       `json:"chanceofrain"`
	WindspeedKmph string       `json:"windspeedKmph"`
}

type dayForecast struct {
	Date      string      `json:"date"`
	MaxtempC  string      `json:"maxtempC"`
	MintempC  string      `json:"mintempC"`
	Astronomy []astronomy `json:"astronomy"`
	Hourly    []hourEntry `json:"hourly"`
}

type wttrResponse struct {
	CurrentCondition []currentCondition `json:"current_condition"`
	NearestArea      []nearestArea      `json:"nearest_area"`
	Weather          []dayForecast      `json:"weather"`
}

// categorizeCode buckets wttr.in's WorldWeatherOnline condition codes into a
// handful of display categories, so clients never need to know wttr.in's raw
// numeric code scheme.
func categorizeCode(code string) weatherpb.Category {
	n, err := strconv.Atoi(code)
	if err != nil {
		return weatherpb.Category_CATEGORY_UNKNOWN
	}
	switch {
	case n == 113:
		return weatherpb.Category_CATEGORY_SUNNY
	case n == 116:
		return weatherpb.Category_CATEGORY_PARTLY_CLOUDY
	case n == 119 || n == 122:
		return weatherpb.Category_CATEGORY_CLOUDY
	case n == 143 || n == 248 || n == 260:
		return weatherpb.Category_CATEGORY_FOG
	case n == 200 || n == 386 || n == 389 || n == 392 || n == 395:
		return weatherpb.Category_CATEGORY_THUNDER
	case n == 179 || n == 182 || n == 185 || n == 227 || n == 230 ||
		n == 317 || n == 320 || n == 350 || n == 362 || n == 365 ||
		n == 374 || n == 377:
		return weatherpb.Category_CATEGORY_SLEET
	case n == 323 || n == 326 || n == 329 || n == 332 || n == 335 || n == 338 ||
		n == 368 || n == 371:
		return weatherpb.Category_CATEGORY_SNOW
	case n == 176 || n == 263 || n == 266 || n == 281 || n == 284 ||
		n == 293 || n == 296 || n == 299 || n == 302 || n == 305 || n == 308 ||
		n == 311 || n == 314 || n == 353 || n == 356 || n == 359:
		return weatherpb.Category_CATEGORY_RAIN
	default:
		return weatherpb.Category_CATEGORY_UNKNOWN
	}
}

// moonPhaseEnum maps wttr.in's astronomy.moon_phase English name to our enum.
func moonPhaseEnum(phase string) weatherpb.MoonPhase {
	switch phase {
	case "New Moon":
		return weatherpb.MoonPhase_MOON_PHASE_NEW_MOON
	case "Waxing Crescent":
		return weatherpb.MoonPhase_MOON_PHASE_WAXING_CRESCENT
	case "First Quarter":
		return weatherpb.MoonPhase_MOON_PHASE_FIRST_QUARTER
	case "Waxing Gibbous":
		return weatherpb.MoonPhase_MOON_PHASE_WAXING_GIBBOUS
	case "Full Moon":
		return weatherpb.MoonPhase_MOON_PHASE_FULL_MOON
	case "Waning Gibbous":
		return weatherpb.MoonPhase_MOON_PHASE_WANING_GIBBOUS
	case "Last Quarter":
		return weatherpb.MoonPhase_MOON_PHASE_LAST_QUARTER
	case "Waning Crescent":
		return weatherpb.MoonPhase_MOON_PHASE_WANING_CRESCENT
	default:
		return weatherpb.MoonPhase_MOON_PHASE_UNKNOWN
	}
}

func fetchWeather() (*weatherpb.WeatherUpdate, error) {
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://wttr.in/?format=j1")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var w wttrResponse
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, err
	}

	if len(w.CurrentCondition) == 0 || len(w.Weather) == 0 {
		return nil, fmt.Errorf("wttr.in: unexpected empty response")
	}

	cur := w.CurrentCondition[0]
	update := &weatherpb.WeatherUpdate{
		FetchedAtUnix:   time.Now().Unix(),
		CurrentCategory: categorizeCode(cur.WeatherCode),
		TempC:           cur.TempC,
		FeelsLikeC:      cur.FeelsLikeC,
		Humidity:        cur.Humidity,
		WindKmph:        cur.WindspeedKmph,
		WindDir:         cur.Winddir16Point,
		UvIndex:         cur.UvIndex,
		VisibilityKm:    cur.Visibility,
	}
	if len(cur.WeatherDesc) > 0 {
		update.CurrentDesc = cur.WeatherDesc[0].Value
	}

	if len(w.NearestArea) > 0 {
		na := w.NearestArea[0]
		area, country := "", ""
		if len(na.AreaName) > 0 {
			area = na.AreaName[0].Value
		}
		if len(na.Country) > 0 {
			country = na.Country[0].Value
		}
		update.Location = area
		if country != "" {
			update.Location += ", " + country
		}
	}

	today := w.Weather[0]
	if len(today.Astronomy) > 0 {
		a := today.Astronomy[0]
		update.TodaySunrise = a.Sunrise
		update.TodaySunset = a.Sunset
		update.MoonPhase = moonPhaseEnum(a.MoonPhase)
		update.MoonIllum = a.MoonIllumination
	}

	if len(w.Weather) > 1 {
		tomorrow := w.Weather[1]
		update.TomorrowMaxC = tomorrow.MaxtempC
		update.TomorrowMinC = tomorrow.MintempC
		// use the midday (1200) hourly slot as the representative condition
		found := false
		for _, h := range tomorrow.Hourly {
			if h.Time == "1200" {
				update.TomorrowCategory = categorizeCode(h.WeatherCode)
				if len(h.WeatherDesc) > 0 {
					update.TomorrowDesc = h.WeatherDesc[0].Value
				}
				update.TomorrowWindKmph = h.WindspeedKmph
				update.TomorrowChanceOfRain = h.ChanceOfRain
				found = true
				break
			}
		}
		if !found && len(tomorrow.Hourly) > 0 {
			mid := tomorrow.Hourly[len(tomorrow.Hourly)/2]
			update.TomorrowCategory = categorizeCode(mid.WeatherCode)
			if len(mid.WeatherDesc) > 0 {
				update.TomorrowDesc = mid.WeatherDesc[0].Value
			}
			update.TomorrowWindKmph = mid.WindspeedKmph
			update.TomorrowChanceOfRain = mid.ChanceOfRain
		}
	}

	return update, nil
}

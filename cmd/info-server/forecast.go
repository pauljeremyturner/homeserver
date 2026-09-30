package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	weatherpb "homeserver/gen/weather"
)

// wttr.in only forecasts 3 days in 3-hour steps, so the hourly and daily
// forecasts (and, so they agree with them, the current conditions) come from
// Open-Meteo (free, no key) for wttr.in's coordinates.

type openMeteoResponse struct {
	UTCOffsetSeconds int `json:"utc_offset_seconds"`
	Current          struct {
		Temp        float64 `json:"temperature_2m"`
		FeelsLike   float64 `json:"apparent_temperature"`
		Humidity    float64 `json:"relative_humidity_2m"`
		WeatherCode int     `json:"weather_code"`
		WindKmph    float64 `json:"wind_speed_10m"`
		WindDeg     float64 `json:"wind_direction_10m"`
		IsDay       int     `json:"is_day"`
		UVIndex     float64 `json:"uv_index"`
	} `json:"current"`
	Hourly struct {
		Time         []int64   `json:"time"`
		Temp         []float64 `json:"temperature_2m"`
		WeatherCode  []int     `json:"weather_code"`
		ChanceOfRain []int     `json:"precipitation_probability"`
		IsDay        []int     `json:"is_day"`
	} `json:"hourly"`
	Daily struct {
		Time []int64   `json:"time"`
		Max  []float64 `json:"temperature_2m_max"`
		Min  []float64 `json:"temperature_2m_min"`
	} `json:"daily"`
}

const forecastDays = 7

func fetchOpenMeteo(lat, lon string) (*openMeteoResponse, error) {
	q := url.Values{
		"latitude":   {lat},
		"longitude":  {lon},
		"current":    {"temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m,wind_direction_10m,is_day,uv_index"},
		"hourly":     {"temperature_2m,weather_code,precipitation_probability,is_day"},
		"daily":      {"temperature_2m_max,temperature_2m_min"},
		"timezone":   {"auto"},
		"timeformat": {"unixtime"},
		// One day more than shown, for the last day's night.
		"forecast_days": {strconv.Itoa(forecastDays + 1)},
	}
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://api.open-meteo.com/v1/forecast?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo: %s", resp.Status)
	}
	var r openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	h := r.Hourly
	if len(h.Temp) != len(h.Time) || len(h.WeatherCode) != len(h.Time) || len(h.ChanceOfRain) != len(h.Time) || len(h.IsDay) != len(h.Time) ||
		len(r.Daily.Max) != len(r.Daily.Time) || len(r.Daily.Min) != len(r.Daily.Time) {
		return nil, fmt.Errorf("open-meteo: mismatched series lengths")
	}
	return &r, nil
}

// applyForecast replaces update's current conditions with Open-Meteo's and
// adds its hourly forecast from now's hour and daily forecast from today.
func applyForecast(update *weatherpb.WeatherUpdate, r *openMeteoResponse, now time.Time) {
	c := r.Current
	update.CurrentCategory = categorizeWMO(c.WeatherCode)
	update.CurrentDesc = describeWMO(c.WeatherCode)
	update.TempC = roundC(c.Temp)
	update.FeelsLikeC = roundC(c.FeelsLike)
	update.Humidity = strconv.Itoa(int(math.Round(c.Humidity)))
	update.WindKmph = strconv.Itoa(int(math.Round(c.WindKmph)))
	update.WindDir = compassPoint(c.WindDeg)
	update.WindBeaufort = beaufortName(update.WindKmph)
	update.UvIndex = strconv.Itoa(int(math.Round(c.UVIndex)))
	update.IsDay = c.IsDay == 1

	h := r.Hourly
	hour := now.Truncate(time.Hour).Unix()
	for i, t := range h.Time {
		if t < hour || len(update.Hourly) == 24 {
			continue
		}
		update.Hourly = append(update.Hourly, &weatherpb.HourForecast{
			TimeUnix:     t,
			Category:     categorizeWMO(h.WeatherCode[i]),
			IsDay:        h.IsDay[i] == 1,
			TempC:        int32(math.Round(h.Temp[i])),
			ChanceOfRain: int32(h.ChanceOfRain[i]),
		})
	}

	zone := time.FixedZone("", r.UTCOffsetSeconds)
	for d, midnight := range r.Daily.Time {
		if d == forecastDays {
			break
		}
		day := &weatherpb.DayForecast{
			Date: time.Unix(midnight, 0).In(zone).Format("2006-01-02"),
			MaxC: int32(math.Round(r.Daily.Max[d])),
			MinC: int32(math.Round(r.Daily.Min[d])),
		}
		// The hours' codes are WMO codes, where higher is (roughly) worse,
		// as Open-Meteo itself picks a day's code.
		dayCode, nightCode := -1, -1
		for i, t := range h.Time {
			switch hrs := (t - midnight) / 3600; {
			case hrs >= 6 && hrs < 18:
				dayCode = max(dayCode, h.WeatherCode[i])
			case hrs >= 18 && hrs < 30:
				nightCode = max(nightCode, h.WeatherCode[i])
			}
			if t >= midnight && t < midnight+24*3600 {
				day.ChanceOfRain = max(day.ChanceOfRain, int32(h.ChanceOfRain[i]))
			}
		}
		day.DayCategory, day.NightCategory = categorizeWMO(dayCode), categorizeWMO(nightCode)
		update.Daily = append(update.Daily, day)
	}
}

func roundC(c float64) string {
	return strconv.Itoa(int(math.Round(c)))
}

// compassPoint names a bearing (degrees clockwise from north) as one of the
// 16 compass points.
func compassPoint(deg float64) string {
	points := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	i := int(math.Round(math.Mod(deg, 360)/22.5)) % 16
	if i < 0 {
		i += 16
	}
	return points[i]
}

// categorizeWMO buckets a WMO weather interpretation code (as used by
// Open-Meteo) into a display category.
func categorizeWMO(code int) weatherpb.Category {
	switch {
	case code == 0 || code == 1:
		return weatherpb.Category_CATEGORY_SUNNY
	case code == 2:
		return weatherpb.Category_CATEGORY_PARTLY_CLOUDY
	case code == 3:
		return weatherpb.Category_CATEGORY_CLOUDY
	case code == 45 || code == 48:
		return weatherpb.Category_CATEGORY_FOG
	case code == 56 || code == 57 || code == 66 || code == 67:
		return weatherpb.Category_CATEGORY_SLEET
	case code >= 51 && code <= 65, code >= 80 && code <= 82:
		return weatherpb.Category_CATEGORY_RAIN
	case code >= 71 && code <= 77, code == 85 || code == 86:
		return weatherpb.Category_CATEGORY_SNOW
	case code >= 95 && code <= 99:
		return weatherpb.Category_CATEGORY_THUNDER
	}
	return weatherpb.Category_CATEGORY_UNKNOWN
}

// describeWMO is a short description of a WMO weather code.
func describeWMO(code int) string {
	switch code {
	case 0:
		return "Clear"
	case 1:
		return "Mainly clear"
	case 2:
		return "Partly cloudy"
	case 3:
		return "Overcast"
	case 45:
		return "Fog"
	case 48:
		return "Freezing fog"
	case 51:
		return "Light drizzle"
	case 53:
		return "Drizzle"
	case 55:
		return "Heavy drizzle"
	case 56, 57:
		return "Freezing drizzle"
	case 61:
		return "Light rain"
	case 63:
		return "Rain"
	case 65:
		return "Heavy rain"
	case 66, 67:
		return "Freezing rain"
	case 71:
		return "Light snow"
	case 73:
		return "Snow"
	case 75:
		return "Heavy snow"
	case 77:
		return "Snow grains"
	case 80:
		return "Light showers"
	case 81:
		return "Showers"
	case 82:
		return "Heavy showers"
	case 85:
		return "Snow showers"
	case 86:
		return "Heavy snow showers"
	case 95:
		return "Thunderstorm"
	case 96, 99:
		return "Thunderstorm, hail"
	}
	return "Unknown"
}

package main

import (
	"testing"
	"time"

	weatherpb "homeserver/gen/weather"
)

func TestApplyForecast(t *testing.T) {
	// Two days of hours from midnight at UTC+1, clear except for rain at
	// 10:00 on day one and thunder at 02:00 on day two (day one's night).
	const offset = 3600
	midnight := time.Date(2026, 9, 30, 0, 0, 0, 0, time.FixedZone("", offset)).Unix()
	var r openMeteoResponse
	r.UTCOffsetSeconds = offset
	for i := range 48 {
		code, rain := 0, 0
		switch i {
		case 10:
			code, rain = 61, 70
		case 26:
			code, rain = 95, 40
		}
		r.Hourly.Time = append(r.Hourly.Time, midnight+int64(i)*3600)
		r.Hourly.Temp = append(r.Hourly.Temp, float64(i))
		r.Hourly.WeatherCode = append(r.Hourly.WeatherCode, code)
		r.Hourly.ChanceOfRain = append(r.Hourly.ChanceOfRain, rain)
		r.Hourly.IsDay = append(r.Hourly.IsDay, 0)
	}
	r.Daily.Time = []int64{midnight, midnight + 24*3600}
	r.Daily.Max = []float64{18.6, 15.4}
	r.Daily.Min = []float64{10.2, 9.5}
	r.Current.WindDeg = 200

	var u weatherpb.WeatherUpdate
	applyForecast(&u, &r, time.Unix(midnight+13*3600+1800, 0)) // 13:30
	if len(u.Hourly) != 24 || u.Hourly[0].TimeUnix != midnight+13*3600 || u.Hourly[0].TempC != 13 {
		t.Errorf("hourly: got %d hours starting %v", len(u.Hourly), u.Hourly[0])
	}
	if u.WindDir != "SSW" {
		t.Errorf("wind dir: got %q, want SSW", u.WindDir)
	}
	if len(u.Daily) != 2 {
		t.Fatalf("daily: got %d days, want 2", len(u.Daily))
	}
	d := u.Daily[0]
	if d.Date != "2026-09-30" || d.MaxC != 19 || d.MinC != 10 || d.ChanceOfRain != 70 ||
		d.DayCategory != weatherpb.Category_CATEGORY_RAIN || d.NightCategory != weatherpb.Category_CATEGORY_THUNDER {
		t.Errorf("day one: got %v", d)
	}
	d = u.Daily[1]
	if d.Date != "2026-10-01" || d.ChanceOfRain != 40 ||
		d.DayCategory != weatherpb.Category_CATEGORY_SUNNY || d.NightCategory != weatherpb.Category_CATEGORY_SUNNY {
		t.Errorf("day two: got %v", d)
	}
}

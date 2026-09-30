package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	cryptopb "homeserver/gen/crypto"
	displaypb "homeserver/gen/display"
	weatherpb "homeserver/gen/weather"
)

var (
	colBg        = rgb(0x0b0d10)
	colPanel     = rgb(0x14171c)
	colRule      = rgb(0x262a31)
	colText      = rgb(0xe8eaed)
	colDim       = rgb(0x8a919c)
	colFaint     = rgb(0x5a616b)
	colSun       = rgb(0xf5c542)
	colCloud     = rgb(0xc9cfd8)
	colCloudDark = rgb(0x7d8591)
	colRain      = rgb(0x5aa9ff)
	colSnow      = rgb(0xffffff)
	colMoon      = rgb(0xe6e2d3)
	colMoonDark  = rgb(0x2a2e35)
	colUp        = rgb(0x4cc38a)
	colDown      = rgb(0xe5534b)
	colBTC       = rgb(0xf7931a)
	colETH       = rgb(0x8c9eff)
	colGold      = rgb(0xd4af37)
	colSilver    = rgb(0xc3c9d2)
	colNewsTag   = rgb(0xe5534b)
)

func rgb(c uint32) color.NRGBA {
	return color.NRGBA{R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c), A: 0xff}
}

// tickerSpeed is how fast the news ticker scrolls, in dp per second, and
// tickerFrame paces its redraws (~30fps is smooth enough and halves the
// Tinker Board's GPU work compared to 60).
const (
	tickerSpeed = 45
	tickerFrame = 33 * time.Millisecond
)

type ui struct {
	th    *material.Theme
	start time.Time
	// loc is the weather location's time zone (from info-server), cached by
	// name; nil until known, when the board's own zone is used.
	locName string
	loc     *time.Location
	// forcePage pins one page with no fading (-page, for screenshots), and
	// forceElapsed is how long it's been up (-anim).
	forcePage    displaypb.Page
	forceElapsed time.Duration
}

func newUI() *ui {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette.Fg = colText
	th.Palette.Bg = colBg
	return &ui{th: th, start: time.Now()}
}

func (u *ui) label(size float32, c color.NRGBA, weight font.Weight, s string) material.LabelStyle {
	l := material.Label(u.th, unit.Sp(size), s)
	l.Color = c
	l.Font.Weight = weight
	l.MaxLines = 1
	return l
}

func (u *ui) layout(gtx layout.Context, now time.Time, s snapshot) layout.Dimensions {
	if loc := u.location(s.weather); loc != nil {
		now = now.In(loc)
	}
	paint.Fill(gtx.Ops, colBg)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.header(gtx, now, s) }),
		layout.Rigid(u.rule),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.page(gtx, now, s) }),
	)
}

// page draws whichever page is showing below the header, fading between
// them; the header itself never fades, so the clock stays steady.
func (u *ui) page(gtx layout.Context, now time.Time, s snapshot) layout.Dimensions {
	page := u.forcePage
	// The planets animate from when the page came up.
	elapsed, period := u.forceElapsed, defaultPagePeriod
	if page == displaypb.Page_PAGE_UNKNOWN {
		var since, next time.Time
		page, since, next = pageShowing(gtx.Now, s.page)
		elapsed, period = gtx.Now.Sub(since), next.Sub(since)
		alpha, wake := pageOpacity(gtx.Now, since, next)
		gtx.Execute(op.InvalidateCmd{At: wake})
		defer paint.PushOpacity(gtx.Ops, alpha).Pop()
	}
	switch page {
	case displaypb.Page_PAGE_PLANETS:
		return u.planets(gtx, now, s.planets, elapsed, period)
	// An older server still cycles through a separate metals page.
	case displaypb.Page_PAGE_MARKETS, displaypb.Page_PAGE_METALS:
		return u.markets(gtx, now, s)
	}
	return u.weather(gtx, now, s)
}

// location returns the time zone of the weather location, or nil if it's
// unknown, so the clock and sun follow the place the weather is for rather
// than however the board's clock is set.
func (u *ui) location(w *weatherpb.WeatherUpdate) *time.Location {
	if w == nil || w.Timezone == "" {
		return u.loc
	}
	if w.Timezone != u.locName {
		loc, err := time.LoadLocation(w.Timezone)
		if err != nil {
			log.Printf("time zone %q: %v", w.Timezone, err)
			loc = nil
		}
		u.locName, u.loc = w.Timezone, loc
	}
	return u.loc
}

func (u *ui) rule(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(1)
	size := image.Pt(gtx.Constraints.Max.X, h)
	paint.FillShape(gtx.Ops, colRule, clip.Rect{Max: size}.Op())
	return layout.Dimensions{Size: size}
}

// --- date / time -----------------------------------------------------------

func (u *ui) header(gtx layout.Context, now time.Time, s snapshot) layout.Dimensions {
	return layout.Inset{Top: 6, Bottom: 4, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(u.label(58, colText, font.Medium, now.Format("15:04")).Layout),
					layout.Rigid(u.label(26, colFaint, font.Normal, now.Format(":05")).Layout),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						l := u.label(24, colText, font.Normal, now.Format("Monday 2 January"))
						l.Alignment = text.End
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						loc := "Locating..."
						if s.weather != nil && s.weather.Location != "" {
							loc = s.weather.Location
						}
						l := u.label(15, colDim, font.Normal, loc)
						l.Alignment = text.End
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						vers := "client " + version
						if s.serverVersion != "" {
							vers += "  ·  server " + s.serverVersion
						}
						children := []layout.FlexChild{
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								l := u.label(13, colFaint, font.Normal, vers)
								l.Alignment = text.End
								return l.Layout(gtx)
							}),
						}
						if s.hasTemp {
							children = append(children,
								layout.Rigid(u.label(13, colFaint, font.Normal, "  ·  ").Layout),
								layout.Rigid(u.label(13, boardTempColour(s.boardTemp), font.Normal, fmt.Sprintf("board %.1f°C", s.boardTemp)).Layout))
						}
						return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, children...)
					}),
				)
			}),
		)
	})
}

// boardTempColour is dim at the Tinker Board's normal running temperature
// (~65C with the dashboard up), amber when it's running hot and red as it
// nears the RK3288's ~85C throttling point.
func boardTempColour(c float64) color.NRGBA {
	switch {
	case c >= 80:
		return colDown
	case c >= 70:
		return colSun
	}
	return colFaint
}

// --- weather / moon --------------------------------------------------------

func (u *ui) weather(gtx layout.Context, now time.Time, s snapshot) layout.Dimensions {
	w := s.weather
	if w == nil {
		return layout.Center.Layout(gtx, u.label(18, colFaint, font.Normal, "Waiting for weather...").Layout)
	}
	// Three columns, top-aligned so their headings line up: now and the
	// moon, then sun and wind, then the week; the next hours along the
	// bottom.
	column := func(width unit.Dp, w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(width), gtx.Dp(width)
			return w(gtx)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 14, Bottom: 10, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					column(340, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.today(gtx, w) }),
							layout.Rigid(layout.Spacer{Height: 26}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.moon(gtx, w) }),
						)
					}),
					layout.Rigid(layout.Spacer{Width: 12}.Layout),
					column(132, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.sun(gtx, now, w) }),
							layout.Rigid(layout.Spacer{Height: 16}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.wind(gtx, w) }),
						)
					}),
					layout.Rigid(layout.Spacer{Width: 32}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.week(gtx, now, w) }),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.hours(gtx, now, w) }),
	)
}

// iconBlock lays out a column heading, then an icon with text lines to its
// right, vertically centred against each other.
func (u *ui) iconBlock(gtx layout.Context, heading string, icon layout.Widget, lines ...layout.Widget) layout.Dimensions {
	children := make([]layout.FlexChild, len(lines))
	for i, l := range lines {
		children[i] = layout.Rigid(l)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.label(13, colDim, font.Medium, heading).Layout),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(icon),
				layout.Rigid(layout.Spacer{Width: 12}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				}),
			)
		}),
	)
}

func (u *ui) today(gtx layout.Context, w *weatherpb.WeatherUpdate) layout.Dimensions {
	// is_day only means something once the forecast has come through.
	night := len(w.Hourly) > 0 && !w.IsDay
	lines := []layout.Widget{
		u.label(64, colText, font.Medium, w.TempC+"°").Layout,
		u.label(17, colText, font.Normal, w.CurrentDesc).Layout,
		u.label(14, colDim, font.Normal, "feels "+w.FeelsLikeC+"°  ·  "+w.Humidity+"% humidity").Layout,
	}
	if uv, err := strconv.Atoi(w.UvIndex); err == nil {
		name, c := uvLevel(uv)
		lines = append(lines, u.label(14, c, font.Normal, fmt.Sprintf("UV %d  ·  %s", uv, name)).Layout)
	}
	return u.iconBlock(gtx, "NOW",
		func(gtx layout.Context) layout.Dimensions {
			return weatherIcon(gtx, w.CurrentCategory, night, gtx.Dp(120))
		},
		lines...)
}

// uvLevel names a UV index on the WHO scale, with a colour that stays quiet
// until the sun needs thinking about.
func uvLevel(uv int) (string, color.NRGBA) {
	switch {
	case uv >= 11:
		return "extreme", colDown
	case uv >= 8:
		return "very high", colDown
	case uv >= 6:
		return "high", colSun
	case uv >= 3:
		return "moderate", colDim
	}
	return "low", colDim
}

// week lists the daily forecast: the day, its chance of rain, day and night
// icons, and its low and high either side of a bar placing them within the
// week's range.
func (u *ui) week(gtx layout.Context, now time.Time, w *weatherpb.WeatherUpdate) layout.Dimensions {
	width := gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.label(13, colDim, font.Medium, "THIS WEEK").Layout),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			days := w.Daily
			if len(days) == 0 {
				return u.label(16, colFaint, font.Normal, "Waiting for forecast...").Layout(gtx)
			}
			lo, hi := days[0].MinC, days[0].MaxC
			for _, d := range days {
				lo, hi = min(lo, d.MinC), max(hi, d.MaxC)
			}
			rowH := min(gtx.Constraints.Max.Y/len(days), gtx.Dp(46))
			icon := gtx.Dp(32)
			barL, barR := gtx.Dp(262), width-gtx.Dp(44)
			today := now.Format("2006-01-02")
			for i, d := range days {
				y := i*rowH + rowH/2
				name := d.Date
				if d.Date == today {
					name = "Today"
				} else if t, err := time.Parse("2006-01-02", d.Date); err == nil {
					name = t.Format("Mon")
				}
				u.labelLeft(gtx, 0, y, u.label(17, colText, font.Normal, name))

				rain := colRain
				if d.ChanceOfRain < 20 {
					rain = colFaint
				}
				r := float32(gtx.Dp(4))
				drawDrop(gtx.Ops, rain, float32(gtx.Dp(70)), float32(y)+r*0.6, r)
				u.labelLeft(gtx, gtx.Dp(80), y, u.label(14, rain, font.Normal, fmt.Sprintf("%d%%", d.ChanceOfRain)))

				for j, cat := range []weatherpb.Category{d.DayCategory, d.NightCategory} {
					off := op.Offset(image.Pt(gtx.Dp(128)+j*(icon+gtx.Dp(6)), y-icon/2)).Push(gtx.Ops)
					weatherIcon(gtx, cat, j == 1, icon)
					off.Pop()
				}

				u.labelRight(gtx, barL-gtx.Dp(10), y, u.label(17, colDim, font.Normal, fmt.Sprintf("%d°", d.MinC)))
				u.labelLeft(gtx, barR+gtx.Dp(10), y, u.label(17, colText, font.Medium, fmt.Sprintf("%d°", d.MaxC)))
				xAt := func(c int32) float32 {
					return float32(barL) + float32(c-lo)/float32(max(hi-lo, 1))*float32(barR-barL)
				}
				thick := float32(gtx.Dp(5))
				yf := float32(y)
				strokeLine(gtx.Ops, colRule, thick, f32.Pt(float32(barL), yf), f32.Pt(float32(barR), yf))
				strokeLine(gtx.Ops, colSun, thick, f32.Pt(xAt(d.MinC), yf), f32.Pt(xAt(d.MaxC)+0.01, yf))
				if name == "Today" {
					if t, err := strconv.Atoi(w.TempC); err == nil && int32(t) >= lo && int32(t) <= hi {
						fillCircle(gtx.Ops, colBg, xAt(int32(t)), yf, thick*0.9)
						fillCircle(gtx.Ops, colText, xAt(int32(t)), yf, thick*0.55)
					}
				}
			}
			return layout.Dimensions{Size: image.Pt(width, rowH*len(days))}
		}),
	)
}

// hoursShown is how many hours ahead the strip along the bottom forecasts.
const hoursShown = 12

// hours is a strip along the bottom of the next hours' forecasts: the time,
// an icon, the temperature over a line tracing it, and the chance of rain.
// Nothing if there's no forecast.
func (u *ui) hours(gtx layout.Context, now time.Time, w *weatherpb.WeatherUpdate) layout.Dimensions {
	var hrs []*weatherpb.HourForecast
	for _, h := range w.Hourly {
		if h.TimeUnix > now.Unix() && len(hrs) < hoursShown {
			hrs = append(hrs, h)
		}
	}
	if len(hrs) == 0 {
		return layout.Dimensions{}
	}
	width, height := gtx.Constraints.Max.X, gtx.Dp(158)
	paint.FillShape(gtx.Ops, colPanel, clip.Rect{Max: image.Pt(width, height)}.Op())
	pad := gtx.Dp(18)
	u.labelLeft(gtx, pad, gtx.Dp(16), u.label(13, colDim, font.Medium, fmt.Sprintf("NEXT %d HOURS", hoursShown)))

	lo, hi := hrs[0].TempC, hrs[0].TempC
	for _, h := range hrs {
		lo, hi = min(lo, h.TempC), max(hi, h.TempC)
	}
	// The line gets at least a 4 degree range, so a steady day stays flat.
	if hi-lo < 4 {
		mid := float32(lo+hi) / 2
		lo, hi = int32(math.Floor(float64(mid-2))), int32(math.Ceil(float64(mid+2)))
	}
	slot := (width - 2*pad) / hoursShown
	icon := gtx.Dp(36)
	lineTop, lineBottom := float32(gtx.Dp(114)), float32(gtx.Dp(130))
	pts := make([]f32.Point, len(hrs))
	for i, h := range hrs {
		x := pad + slot*i + slot/2
		pts[i] = f32.Pt(float32(x), lineBottom-float32(h.TempC-lo)/float32(hi-lo)*(lineBottom-lineTop))
		u.labelAt(gtx, x, gtx.Dp(38), u.label(14, colDim, font.Normal, time.Unix(h.TimeUnix, 0).In(now.Location()).Format("15:04")))
		off := op.Offset(image.Pt(x-icon/2, gtx.Dp(50))).Push(gtx.Ops)
		weatherIcon(gtx, h.Category, !h.IsDay, icon)
		off.Pop()
		u.labelAt(gtx, x, gtx.Dp(101), u.label(17, colText, font.Medium, fmt.Sprintf("%d°", h.TempC)))
		rain := colRain
		if h.ChanceOfRain < 20 {
			rain = colFaint
		}
		u.labelAt(gtx, x, gtx.Dp(146), u.label(13, rain, font.Normal, fmt.Sprintf("%d%%", h.ChanceOfRain)))
	}
	line := colSun
	line.A = 140
	if len(pts) > 1 {
		strokeLine(gtx.Ops, line, float32(gtx.Dp(2)), pts...)
	}
	for _, p := range pts {
		fillCircle(gtx.Ops, colText, p.X, p.Y, float32(gtx.Dp(3)))
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (u *ui) moon(gtx layout.Context, w *weatherpb.WeatherUpdate) layout.Dimensions {
	illum, _ := strconv.ParseFloat(strings.TrimSpace(w.MoonIllum), 64)
	illum /= 100
	switch w.MoonPhase {
	case weatherpb.MoonPhase_MOON_PHASE_NEW_MOON:
		illum = 0
	case weatherpb.MoonPhase_MOON_PHASE_FULL_MOON:
		illum = 1
	}
	waxing := w.MoonPhase <= weatherpb.MoonPhase_MOON_PHASE_FULL_MOON
	return u.iconBlock(gtx, "MOON",
		func(gtx layout.Context) layout.Dimensions { return moonIcon(gtx, illum, waxing, gtx.Dp(52)) },
		u.label(15, colText, font.Normal, moonPhaseLabel(w.MoonPhase)).Layout,
		u.label(14, colDim, font.Normal, strings.TrimSpace(w.MoonIllum)+"% lit").Layout,
	)
}

// centred lays out a label centred in a box w wide.
func (u *ui) centred(gtx layout.Context, w int, l material.LabelStyle) layout.Dimensions {
	gtx.Constraints = layout.Exact(image.Pt(w, gtx.Constraints.Max.Y))
	gtx.Constraints.Min.Y = 0
	l.Alignment = text.Middle
	return l.Layout(gtx)
}

// labelAt draws a label centred on (x, y), without affecting layout.
func (u *ui) labelAt(gtx layout.Context, x, y int, l material.LabelStyle) {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	dims := l.Layout(gtx)
	call := macro.Stop()
	defer op.Offset(image.Pt(x-dims.Size.X/2, y-dims.Size.Y/2)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// sun shows today's daylight as the sun's path over a semicircle, with the
// next sunrise or sunset underneath.
func (u *ui) sun(gtx layout.Context, now time.Time, w *weatherpb.WeatherUpdate) layout.Dimensions {
	width := gtx.Dp(128)
	rise, riseOK := clockOn(now, w.TodaySunrise)
	set, setOK := clockOn(now, w.TodaySunset)
	frac := -1.0
	next := ""
	switch {
	case !riseOK || !setOK || !set.After(rise):
	case now.Before(rise):
		next = "sunrise " + rise.Format("15:04")
	case now.Before(set):
		frac = now.Sub(rise).Seconds() / set.Sub(rise).Seconds()
		next = "sunset " + set.Format("15:04")
	default:
		next = "sunset " + set.Format("15:04")
		if w.TomorrowSunrise != "" {
			next = "sunrise " + clockTime(w.TomorrowSunrise)
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.centred(gtx, width, u.label(13, colDim, font.Medium, "SUN"))
		}),
		layout.Rigid(layout.Spacer{Height: 14}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return sunArc(gtx, frac, width, width/2+gtx.Dp(2))
		}),
		layout.Rigid(layout.Spacer{Height: 3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(u.label(13, colDim, font.Normal, clockTime(w.TodaySunrise)).Layout),
				layout.Rigid(u.label(13, colDim, font.Normal, clockTime(w.TodaySunset)).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.centred(gtx, width, u.label(16, colText, font.Normal, next))
		}),
	)
}

// wind shows the wind direction on a compass, with the direction, speed and
// Beaufort name below.
func (u *ui) wind(gtx layout.Context, w *weatherpb.WeatherUpdate) layout.Dimensions {
	// The column is wider than the compass so the longer Beaufort names
	// ("Moderate breeze") fit under it.
	width, size, margin := gtx.Dp(132), gtx.Dp(104), gtx.Dp(14)
	bearing, ok := compassBearing(w.WindDir)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.centred(gtx, width, u.label(13, colDim, font.Medium, "WIND"))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			defer op.Offset(image.Pt((width-size)/2, 0)).Push(gtx.Ops).Pop()
			compass(gtx, bearing, ok, size, margin)
			c, off := size/2, size/2-margin/2
			for i, l := range []string{"N", "E", "S", "W"} {
				dx, dy := [4]int{0, 1, 0, -1}[i], [4]int{-1, 0, 1, 0}[i]
				u.labelAt(gtx, c+dx*off, c+dy*off, u.label(11, colDim, font.Medium, l))
			}
			return layout.Dimensions{Size: image.Pt(width, size)}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.centred(gtx, width, u.label(16, colText, font.Normal, strings.TrimSpace(w.WindDir+" "+w.WindKmph+" km/h")))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.centred(gtx, width, u.label(14, colDim, font.Normal, w.WindBeaufort))
		}),
	)
}

// compassBearing turns a 16-point compass direction ("SSW") into degrees
// clockwise from north.
func compassBearing(dir string) (float64, bool) {
	points := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	for i, p := range points {
		if p == strings.TrimSpace(dir) {
			return float64(i) * 22.5, true
		}
	}
	return 0, false
}

// clockOn is wttr.in's "07:12 AM" style time on now's date.
func clockOn(now time.Time, s string) (time.Time, bool) {
	t, err := time.Parse("03:04 PM", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()), true
}

// clockTime turns wttr.in's "07:12 AM" style into 24h "07:12".
func clockTime(s string) string {
	t, err := time.Parse("03:04 PM", strings.TrimSpace(s))
	if err != nil {
		return s
	}
	return t.Format("15:04")
}

func moonPhaseLabel(phase weatherpb.MoonPhase) string {
	switch phase {
	case weatherpb.MoonPhase_MOON_PHASE_NEW_MOON:
		return "New Moon"
	case weatherpb.MoonPhase_MOON_PHASE_WAXING_CRESCENT:
		return "Waxing Crescent"
	case weatherpb.MoonPhase_MOON_PHASE_FIRST_QUARTER:
		return "First Quarter"
	case weatherpb.MoonPhase_MOON_PHASE_WAXING_GIBBOUS:
		return "Waxing Gibbous"
	case weatherpb.MoonPhase_MOON_PHASE_FULL_MOON:
		return "Full Moon"
	case weatherpb.MoonPhase_MOON_PHASE_WANING_GIBBOUS:
		return "Waning Gibbous"
	case weatherpb.MoonPhase_MOON_PHASE_LAST_QUARTER:
		return "Last Quarter"
	case weatherpb.MoonPhase_MOON_PHASE_WANING_CRESCENT:
		return "Waning Crescent"
	}
	return "Unknown"
}

// --- markets / news --------------------------------------------------------

// chartCard is one price chart on the markets page.
type chartCard struct {
	title, note string // "XAU / GBP", and a faint aside after it
	c           *cryptopb.CryptoUpdate
	accent      color.NRGBA
}

// markets is a page of four price charts, crypto above metals, with the
// wallet and the news below.
func (u *ui) markets(gtx layout.Context, now time.Time, s snapshot) layout.Dimensions {
	row := func(left, right chartCard) layout.FlexChild {
		return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.coin(gtx, now, left) }),
				layout.Rigid(layout.Spacer{Width: 32}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.coin(gtx, now, right) }),
			)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 14, Bottom: 12, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					row(chartCard{"BTC / GBP", "", s.btc, colBTC},
						chartCard{"ETH / GBP", "", s.eth, colETH}),
					layout.Rigid(layout.Spacer{Height: 16}.Layout),
					row(chartCard{"XAU / GBP", "gold, via PAXG", s.xau, colGold},
						chartCard{"XAG / GBP", "silver, via KAG", s.xag, colSilver}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.wallet(gtx, s) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.ticker(gtx, now, s) }),
	)
}

func (u *ui) coin(gtx layout.Context, now time.Time, card chartCard) layout.Dimensions {
	c, accent := card.c, card.accent
	heading := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(u.label(13, accent, font.Bold, card.title).Layout),
			layout.Rigid(u.label(12, colFaint, font.Normal, "  "+card.note).Layout),
		)
	}
	if c == nil {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(heading),
			layout.Rigid(u.label(16, colFaint, font.Normal, "waiting...").Layout),
		)
	}
	arrow, changeCol := "▲", colUp
	if c.ChangePct < 0 {
		arrow, changeCol = "▼", colDown
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(heading),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					l := u.label(13, changeCol, font.Medium, fmt.Sprintf("%s %.1f%% 7d", arrow, math.Abs(c.ChangePct)))
					l.Alignment = text.End
					return l.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(u.label(30, colText, font.Medium, pounds(c.Latest, c.Latest)).Layout),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.priceChart(gtx, now, c, accent) }),
	)
}

// priceChart draws a coin's price history filling the space it's given: a
// line with a faint fill, a price axis of axisLabels round-number gridlines
// labelled on the right, and the days along the bottom (in now's time zone).
func (u *ui) priceChart(gtx layout.Context, now time.Time, c *cryptopb.CryptoUpdate, accent color.NRGBA) layout.Dimensions {
	size := gtx.Constraints.Max
	dims := layout.Dimensions{Size: size}
	n := len(c.Prices)
	if n < 2 {
		return dims
	}
	lo, step := priceAxis(c.Min, c.Max, axisLabels)
	hi := lo + step*(axisLabels-1)
	// Times place the points and the day labels; an older server sends none,
	// so fall back to even spacing and no days.
	times := c.TimesUnix
	if len(times) != n {
		times = nil
	}
	// The price labels get their own column on the right, clear of the line.
	gutter := gtx.Dp(54)
	w := float32(size.X - gutter)
	dayRow := gtx.Dp(18)
	top, bottom := float32(gtx.Dp(8)), float32(size.Y-dayRow-gtx.Dp(8))
	xAt := func(i int) float32 { return float32(i) / float32(n-1) * w }
	var t0, t1 int64
	if times != nil {
		t0, t1 = times[0], times[n-1]
		xAt = func(i int) float32 { return float32(times[i]-t0) / float32(max(t1-t0, 1)) * w }
	}
	yAt := func(p float64) float32 { return top + (1-float32((p-lo)/(hi-lo)))*(bottom-top) }

	// Day boundaries and names.
	if times != nil {
		loc := now.Location()
		start := time.Unix(t0, 0).In(loc)
		day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
		for ; day.Unix() < t1; day = day.AddDate(0, 0, 1) {
			if day.Unix() > t0 {
				x := float32(day.Unix()-t0) / float32(t1-t0) * w
				strokeLine(gtx.Ops, colRule, float32(gtx.Dp(1)), f32.Pt(x, top), f32.Pt(x, float32(size.Y-dayRow)))
			}
			noon := day.Add(12 * time.Hour).Unix()
			if noon > t0 && noon < t1 {
				x := int(float32(noon-t0) / float32(t1-t0) * w)
				u.labelAt(gtx, x, size.Y-dayRow/2, u.label(12, colFaint, font.Normal, day.Format("Mon")))
			}
		}
	}
	// Price axis.
	for k := range axisLabels {
		p := lo + step*float64(k)
		y := yAt(p)
		strokeLine(gtx.Ops, colRule, float32(gtx.Dp(1)), f32.Pt(0, y), f32.Pt(w, y))
		u.labelRight(gtx, size.X, int(y), u.label(12, colDim, font.Normal, pounds(p, step)))
	}

	pts := make([]f32.Point, n)
	for i, p := range c.Prices {
		pts[i] = f32.Pt(xAt(i), yAt(p))
	}
	area := append([]f32.Point{f32.Pt(pts[0].X, bottom)}, pts...)
	area = append(area, f32.Pt(pts[n-1].X, bottom))
	fill := accent
	fill.A = 40
	fillPolygon(gtx.Ops, fill, area)
	stroke := float32(gtx.Dp(2))
	strokeLine(gtx.Ops, accent, stroke, pts...)
	fillCircle(gtx.Ops, accent, pts[n-1].X, pts[n-1].Y, stroke*1.8)
	return dims
}

// axisLabels is how many round-number prices are marked up a chart.
const axisLabels = 7

// priceAxis picks a round-number step and a bottom value, a multiple of the
// step, so that n evenly spaced labels from the bottom cover lo..hi as
// tightly as possible.
func priceAxis(lo, hi float64, n int) (bottom, step float64) {
	if hi <= lo {
		hi = lo + 1
	}
	mag := math.Pow(10, math.Floor(math.Log10((hi-lo)/float64(n-1))))
	for {
		for _, m := range []float64{1, 1.5, 2, 2.5, 3, 4, 5, 6, 8} {
			step = m * mag
			bottom = math.Floor(lo/step) * step
			if bottom+step*float64(n-1) >= hi {
				return bottom, step
			}
		}
		mag *= 10
	}
}

// labelWidth measures a label without drawing it.
func (u *ui) labelWidth(gtx layout.Context, l material.LabelStyle) int {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	dims := l.Layout(gtx)
	macro.Stop()
	return dims.Size.X
}

// labelLeft draws a label left-aligned to x and vertically centred on y.
func (u *ui) labelLeft(gtx layout.Context, x, y int, l material.LabelStyle) {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	dims := l.Layout(gtx)
	call := macro.Stop()
	defer op.Offset(image.Pt(x, y-dims.Size.Y/2)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// labelRight draws a label right-aligned to x and vertically centred on y.
func (u *ui) labelRight(gtx layout.Context, x, y int, l material.LabelStyle) {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	dims := l.Layout(gtx)
	call := macro.Stop()
	defer op.Offset(image.Pt(x-dims.Size.X, y-dims.Size.Y/2)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// wallet is a single line under the charts; nothing if no wallet is
// configured on the server.
func (u *ui) wallet(gtx layout.Context, s snapshot) layout.Dimensions {
	if s.wallet == nil {
		return layout.Dimensions{}
	}
	children := []layout.FlexChild{
		layout.Rigid(u.label(13, colDim, font.Bold, strings.ToUpper(s.wallet.Label)).Layout),
		layout.Rigid(layout.Spacer{Width: 18}.Layout),
		layout.Rigid(u.label(22, colText, font.Medium, fmt.Sprintf("%.8f BTC", s.wallet.BalanceBtc)).Layout),
	}
	if s.btc != nil {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: 18}.Layout),
			layout.Rigid(u.label(22, colBTC, font.Medium, "≈ £"+thousands(s.wallet.BalanceBtc*s.btc.Latest)).Layout))
	}
	return layout.Inset{Bottom: 12, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, children...)
	})
}

// pounds formats a price with a £ sign, in whole pounds with thousands
// separators unless scale (the price itself, or an axis step) is small
// enough that pence matter, e.g. silver at £48.82.
func pounds(v, scale float64) string {
	if math.Abs(scale) < 100 && math.Abs(v) < 1000 {
		return fmt.Sprintf("£%.2f", v)
	}
	return "£" + thousands(v)
}

// thousands formats a whole-pound amount with comma separators.
func thousands(v float64) string {
	n := strconv.FormatInt(int64(math.Round(v)), 10)
	neg := strings.HasPrefix(n, "-")
	n = strings.TrimPrefix(n, "-")
	var b strings.Builder
	for i, r := range n {
		if i > 0 && (len(n)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// --- news ticker -----------------------------------------------------------

func (u *ui) ticker(gtx layout.Context, now time.Time, s snapshot) layout.Dimensions {
	height := gtx.Dp(40)
	width := gtx.Constraints.Max.X
	paint.FillShape(gtx.Ops, colPanel, clip.Rect{Max: image.Pt(width, height)}.Op())
	gtx.Constraints = layout.Exact(image.Pt(width, height))

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return layout.Inset{Left: 18, Right: 14}.Layout(gtx, u.label(13, colNewsTag, font.Bold, "NEWS").Layout)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if s.news == nil || len(s.news.Headlines) == 0 {
				return u.label(16, colFaint, font.Normal, "Waiting for news...").Layout(gtx)
			}
			return u.scroll(gtx, now, strings.Join(s.news.Headlines, "     •     ")+"     •     ")
		}),
	)
}

// scroll draws txt scrolling right-to-left, looping seamlessly, and asks for
// the next animation frame.
func (u *ui) scroll(gtx layout.Context, now time.Time, txt string) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()

	// Measure (and record) the text once, unconstrained in width.
	mgtx := gtx
	mgtx.Constraints = layout.Constraints{Max: image.Pt(math.MaxInt32/2, size.Y)}
	macro := op.Record(gtx.Ops)
	l := u.label(17, colText, font.Normal, txt)
	dims := l.Layout(mgtx)
	call := macro.Stop()
	if dims.Size.X == 0 {
		return layout.Dimensions{Size: size}
	}

	elapsed := now.Sub(u.start).Seconds()
	offset := int(elapsed*float64(gtx.Dp(tickerSpeed))) % dims.Size.X
	y := (size.Y - dims.Size.Y) / 2
	for x := -offset; x < size.X; x += dims.Size.X {
		t := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		t.Pop()
	}
	gtx.Execute(op.InvalidateCmd{At: now.Add(tickerFrame)})
	return layout.Dimensions{Size: size}
}

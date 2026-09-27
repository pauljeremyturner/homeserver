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
	planetspb "homeserver/gen/planets"
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
	// forcePage pins one page with no fading (-page, for screenshots).
	forcePage displaypb.Page
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
	if page == displaypb.Page_PAGE_UNKNOWN {
		var since, next time.Time
		page, since, next = pageShowing(gtx.Now, s.page)
		alpha, wake := pageOpacity(gtx.Now, since, next)
		gtx.Execute(op.InvalidateCmd{At: wake})
		defer paint.PushOpacity(gtx.Ops, alpha).Pop()
	}
	if page == displaypb.Page_PAGE_MARKETS {
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
	// Three columns, top-aligned so their headings line up: the forecasts,
	// then sun and wind, then the solar system in whatever width is left.
	column := func(width unit.Dp, w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(width), gtx.Dp(width)
			return w(gtx)
		})
	}
	return layout.Inset{Top: 16, Bottom: 12, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			column(300, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.today(gtx, w) }),
					layout.Rigid(layout.Spacer{Height: 22}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.tomorrow(gtx, w) }),
					layout.Rigid(layout.Spacer{Height: 22}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.moon(gtx, w) }),
				)
			}),
			layout.Rigid(layout.Spacer{Width: 12}.Layout),
			column(132, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.sun(gtx, now, w) }),
					layout.Rigid(layout.Spacer{Height: 26}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.wind(gtx, w) }),
				)
			}),
			layout.Rigid(layout.Spacer{Width: 20}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.solarSystem(gtx, s.planets) }),
		)
	})
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
	return u.iconBlock(gtx, "NOW",
		func(gtx layout.Context) layout.Dimensions { return weatherIcon(gtx, w.CurrentCategory, gtx.Dp(96)) },
		u.label(48, colText, font.Medium, w.TempC+"°").Layout,
		u.label(16, colText, font.Normal, w.CurrentDesc).Layout,
		u.label(14, colDim, font.Normal, "feels "+w.FeelsLikeC+"°  ·  "+w.Humidity+"% humidity").Layout,
	)
}

func (u *ui) tomorrow(gtx layout.Context, w *weatherpb.WeatherUpdate) layout.Dimensions {
	if w.TomorrowDesc == "" {
		return layout.Dimensions{}
	}
	return u.iconBlock(gtx, "TOMORROW",
		func(gtx layout.Context) layout.Dimensions { return weatherIcon(gtx, w.TomorrowCategory, gtx.Dp(64)) },
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(u.label(30, colText, font.Medium, w.TomorrowMaxC+"°").Layout),
				layout.Rigid(u.label(20, colDim, font.Normal, " / "+w.TomorrowMinC+"°").Layout),
			)
		},
		u.label(15, colText, font.Normal, w.TomorrowDesc).Layout,
		u.label(14, colDim, font.Normal, "rain "+w.TomorrowChanceOfRain+"%  ·  "+w.TomorrowWindKmph+" km/h").Layout,
	)
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

// --- solar system ----------------------------------------------------------

// solarSystem draws the planets on a top-down dial filling the column's
// width (or height, if that's smaller), with each planet's initial beside it.
func (u *ui) solarSystem(gtx layout.Context, p *planetspb.PlanetsUpdate) layout.Dimensions {
	width := gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.centred(gtx, width, u.label(13, colDim, font.Medium, "SOLAR SYSTEM"))
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			size := min(width, gtx.Constraints.Max.Y)
			if p == nil || len(p.Planets) == 0 {
				return layout.Dimensions{Size: image.Pt(width, size)}
			}
			defer op.Offset(image.Pt((width-size)/2, 0)).Push(gtx.Ops).Pop()
			lons := make([]float64, len(p.Planets))
			for i, pl := range p.Planets {
				lons[i] = pl.LongitudeDeg
			}
			// Leave room outside the last orbit for its planet's initial.
			spots := planetDial(gtx, lons, size, gtx.Dp(14))
			for i, pl := range p.Planets {
				if pl.Name == "" {
					continue
				}
				// The initial sits just outside the planet along its radius.
				off := float32(gtx.Dp(10))
				x := spots[i].pos.X + spots[i].out.X*off
				y := spots[i].pos.Y + spots[i].out.Y*off
				u.labelAt(gtx, int(x), int(y), u.label(10, planetColour(i), font.Medium, pl.Name[:1]))
			}
			return layout.Dimensions{Size: image.Pt(width, size)}
		}),
	)
}

// --- crypto / wallet / news ------------------------------------------------

func (u *ui) markets(gtx layout.Context, now time.Time, s snapshot) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 14, Bottom: 12, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.coin(gtx, now, "BTC", s.btc, colBTC) }),
					layout.Rigid(layout.Spacer{Width: 24}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.coin(gtx, now, "ETH", s.eth, colETH) }),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.wallet(gtx, s) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.ticker(gtx, now, s) }),
	)
}

func (u *ui) coin(gtx layout.Context, now time.Time, sym string, c *cryptopb.CryptoUpdate, accent color.NRGBA) layout.Dimensions {
	if c == nil {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(u.label(13, accent, font.Bold, sym+" / GBP").Layout),
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
				layout.Rigid(u.label(13, accent, font.Bold, sym+" / GBP").Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					l := u.label(13, changeCol, font.Medium, fmt.Sprintf("%s %.1f%% 7d", arrow, math.Abs(c.ChangePct)))
					l.Alignment = text.End
					return l.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(u.label(30, colText, font.Medium, "£"+thousands(c.Latest)).Layout),
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
		u.labelRight(gtx, size.X, int(y), u.label(12, colDim, font.Normal, "£"+thousands(p)))
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

package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

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
	paint.Fill(gtx.Ops, colBg)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.header(gtx, now, s) }),
		layout.Rigid(u.rule),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.weather(gtx, s.weather) }),
		layout.Rigid(u.rule),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.crypto(gtx, s) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return u.ticker(gtx, now, s) }),
	)
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
				)
			}),
		)
	})
}

// --- weather / moon --------------------------------------------------------

func (u *ui) weather(gtx layout.Context, w *weatherpb.WeatherUpdate) layout.Dimensions {
	if w == nil {
		return layout.Center.Layout(gtx, u.label(18, colFaint, font.Normal, "Waiting for weather...").Layout)
	}
	// The band is centred vertically in whatever height is left over, with
	// the three columns top-aligned so their headings line up.
	return layout.Inset{Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(0.42, func(gtx layout.Context) layout.Dimensions { return u.today(gtx, w) }),
				layout.Flexed(0.31, func(gtx layout.Context) layout.Dimensions { return u.tomorrow(gtx, w) }),
				layout.Flexed(0.27, func(gtx layout.Context) layout.Dimensions { return u.moon(gtx, w) }),
			)
		})
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
		u.label(14, colDim, font.Normal, "wind "+w.WindKmph+" km/h "+w.WindDir).Layout,
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
		layout.Spacer{Height: 6}.Layout,
		u.label(14, colDim, font.Normal, "↑ "+clockTime(w.TodaySunrise)+"   ↓ "+clockTime(w.TodaySunset)).Layout,
	)
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

// --- crypto / wallet -------------------------------------------------------

func (u *ui) crypto(gtx layout.Context, s snapshot) layout.Dimensions {
	return layout.Inset{Top: 10, Bottom: 14, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.coin(gtx, "BTC", s.btc, colBTC) }),
			layout.Rigid(layout.Spacer{Width: 20}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return u.coin(gtx, "ETH", s.eth, colETH) }),
			layout.Rigid(layout.Spacer{Width: 20}.Layout),
			layout.Flexed(0.8, func(gtx layout.Context) layout.Dimensions { return u.wallet(gtx, s) }),
		)
	})
}

func (u *ui) coin(gtx layout.Context, sym string, c *cryptopb.CryptoUpdate, accent color.NRGBA) layout.Dimensions {
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
		layout.Rigid(u.label(28, colText, font.Medium, "£"+thousands(c.Latest)).Layout),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return sparkline(gtx, c.Prices, c.Min, c.Max, accent, gtx.Constraints.Max.X, gtx.Dp(40))
		}),
	)
}

func (u *ui) wallet(gtx layout.Context, s snapshot) layout.Dimensions {
	if s.wallet == nil {
		return layout.Dimensions{}
	}
	lines := []layout.FlexChild{
		layout.Rigid(u.label(13, colDim, font.Bold, strings.ToUpper(s.wallet.Label)).Layout),
		layout.Rigid(u.label(22, colText, font.Medium, fmt.Sprintf("%.8f", s.wallet.BalanceBtc)).Layout),
		layout.Rigid(u.label(13, colDim, font.Normal, "BTC").Layout),
	}
	if s.btc != nil {
		lines = append(lines,
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(u.label(18, colBTC, font.Medium, "≈ £"+thousands(s.wallet.BalanceBtc*s.btc.Latest)).Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lines...)
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

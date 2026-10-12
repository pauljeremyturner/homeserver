package main

import (
	"cmp"
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	skypb "homeserver/gen/sky"
	weatherpb "homeserver/gen/weather"
)

// skyPlanetIndex is each planet's index in planetColour, for the sky chart
// to colour them as the planets page does.
var skyPlanetIndex = map[string]int{"Venus": 1, "Mars": 3, "Jupiter": 4, "Saturn": 5}

// skyNicknames are the names constellations go by here, shown after their
// own.
var skyNicknames = map[string]string{"Ursa Major": "the Plough"}

// skyBestRows is how many constellations the best-tonight list names.
const skyBestRows = 6

// skyShowpieces are the easiest constellations to spot and the best known,
// in order; they go first among those best later in the night.
var skyShowpieces = []string{"Orion", "Ursa Major", "Cassiopeia", "Cygnus", "Taurus", "Leo", "Gemini", "Scorpius"}

// showpiece is c's place in skyShowpieces, or after them all.
func showpiece(c *skypb.Constellation) int {
	if i := slices.Index(skyShowpieces, c.Name); i >= 0 {
		return i
	}
	return len(skyShowpieces)
}

// sky draws tonight's sky from info-server: a chart of the whole sky as
// seen lying on your back with your head to the north (so east is on the
// left), and beside it when it's dark, what the cloud and Moon will allow,
// and the constellations best placed, with where to look.
func (u *ui) sky(gtx layout.Context, now time.Time, s snapshot) layout.Dimensions {
	k := s.sky
	switch {
	case k == nil:
		return layout.Center.Layout(gtx, u.label(18, colFaint, font.Normal, "Waiting for the sky...").Layout)
	case !k.Dark:
		return layout.Center.Layout(gtx, u.label(18, colFaint, font.Normal, "Not dark enough tonight to see the stars").Layout)
	}
	return layout.Inset{Top: 12, Bottom: 12, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width, height := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
		size := min(height, width*11/20)
		u.skyChart(gtx, k, size)
		off := op.Offset(image.Pt(size+gtx.Dp(28), 0)).Push(gtx.Ops)
		u.skyNotes(gtx, now, k, s.weather)
		off.Pop()
		return layout.Dimensions{Size: image.Pt(width, size)}
	})
}

// skyChart draws the sky in a size x size box: the horizon round the edge,
// the zenith in the middle, rings at 30 and 60 degrees up.
func (u *ui) skyChart(gtx layout.Context, k *skypb.SkyUpdate, size int) {
	c := float32(size) / 2
	radius := c - float32(gtx.Dp(16))
	at := func(alt, az float32) f32.Point {
		r := radius * (90 - alt) / 90
		a := float64(az) * math.Pi / 180
		return f32.Pt(c-r*float32(math.Sin(a)), c-r*float32(math.Cos(a)))
	}

	fillCircle(gtx.Ops, colPanel, c, c, radius)
	for _, alt := range []float32{30, 60} {
		strokeCircle(gtx, colRule, c, c, radius*(90-alt)/90)
	}
	strokeCircle(gtx, colFaint, c, c, radius)
	for _, d := range []struct {
		name   string
		dx, dy float32
	}{{"N", 0, -1}, {"E", -1, 0}, {"S", 0, 1}, {"W", 1, 0}} {
		r := radius + float32(gtx.Dp(9))
		u.labelAt(gtx, int(c+d.dx*r), int(c+d.dy*r), u.label(12, colDim, font.Medium, d.name))
	}

	// The less prominent constellations first, so the others draw over them.
	cs := slices.Clone(k.Constellations)
	slices.SortStableFunc(cs, func(a, b *skypb.Constellation) int { return cmp.Compare(b.Rank, a.Rank) })
	for _, con := range cs {
		line, star := colDim, colText
		line.A, star.A = 150, 230
		if con.Rank > 1 {
			line, star = colFaint, colDim
			line.A = 120
		}
		for _, p := range con.Lines {
			pts := make([]f32.Point, len(p.AltDeg))
			for i := range pts {
				pts[i] = at(p.AltDeg[i], p.AzDeg[i])
			}
			strokeLine(gtx.Ops, line, float32(gtx.Dp(1)), pts...)
			for i, pt := range pts {
				if p.AltDeg[i] > 0.01 { // not where it's cut at the horizon
					fillCircle(gtx.Ops, star, pt.X, pt.Y, float32(gtx.Dp(1.5)))
				}
			}
		}
	}
	for _, con := range cs {
		if con.Centre.AltDeg < 8 || (con.Rank > 1 && con.Centre.AltDeg < 20) {
			continue
		}
		col, size := colText, float32(11)
		if con.Rank > 1 {
			col, size = colFaint, 10
		}
		pos := at(con.Centre.AltDeg, con.Centre.AzDeg)
		u.labelAt(gtx, int(pos.X), int(pos.Y), u.label(size, col, font.Normal, con.Name))
	}

	for _, b := range k.Bodies {
		pos := at(b.At.AltDeg, b.At.AzDeg)
		col, dot := colMoon, float32(gtx.Dp(6))
		if i, ok := skyPlanetIndex[b.Name]; ok {
			col, dot = planetColour(i), float32(gtx.Dp(4))
		}
		fillCircle(gtx.Ops, col, pos.X, pos.Y, dot)
		u.labelLeft(gtx, int(pos.X+dot+float32(gtx.Dp(4))), int(pos.Y), u.label(11, col, font.Medium, b.Name))
	}
}

// skyNotes is the column beside the chart: when it's for and when it's
// dark, the forecast and the Moon then, and the best constellations.
func (u *ui) skyNotes(gtx layout.Context, now time.Time, k *skypb.SkyUpdate, w *weatherpb.WeatherUpdate) {
	tz := now.Location()
	view, dusk, dawn := time.Unix(k.ViewUnix, 0).In(tz), time.Unix(k.DuskUnix, 0).In(tz), time.Unix(k.DawnUnix, 0).In(tz)
	clock := func(t time.Time) string { return t.Format("15:04") }

	y := gtx.Dp(8)
	line := func(size float32, col color.NRGBA, weight font.Weight, s string, gap unit.Dp) {
		u.labelLeft(gtx, 0, y, u.label(size, col, weight, s))
		y += gtx.Dp(gap)
	}
	title := "TONIGHT'S SKY"
	if !now.Before(dusk) {
		title = "THE SKY NOW"
	}
	line(13, colDim, font.Medium, title, 22)
	line(15, colText, font.Normal, fmt.Sprintf("as at %s", clock(view)), 20)
	line(12, colFaint, font.Normal, fmt.Sprintf("dark %s to %s", clock(dusk), clock(dawn)), 26)

	if text, col := skyCloud(w, view); text != "" {
		line(13, col, font.Normal, text, 20)
	}
	if text := skyMoon(w, k); text != "" {
		line(13, colMoon, font.Normal, text, 20)
	}
	for _, b := range k.Bodies {
		if i, ok := skyPlanetIndex[b.Name]; ok {
			line(13, planetColour(i), font.Normal, b.Name+", "+skyWhere(b.At), 20)
		}
	}

	y += gtx.Dp(10)
	line(13, colDim, font.Medium, "BEST TONIGHT", 24)
	for _, con := range skyBest(k.Constellations, k.DawnUnix) {
		name := con.Name
		if nick := skyNicknames[name]; nick != "" {
			name += ", " + nick
		}
		line(15, colText, font.Normal, name, 18)
		where := skyWhere(con.Centre)
		if con.Centre.AltDeg < 20 {
			where = fmt.Sprintf("best at %s, %s", clock(time.Unix(con.BestUnix, 0).In(tz)), skyWhere(con.Best))
		} else if con.Rising {
			where += ", rising"
		}
		line(12, colDim, font.Normal, where, 26)
	}
}

// skyBest picks the most prominent constellations worth looking for: those
// well up at the time the chart's for, highest first, then (keeping two
// rows for them, so Orion gets a mention through autumn) those best later
// in the night: not just still climbing when it gets light at dawn.
func skyBest(cs []*skypb.Constellation, dawn int64) []*skypb.Constellation {
	var up, later []*skypb.Constellation
	for _, c := range cs {
		switch {
		case c.Rank != 1:
		case c.Centre.AltDeg >= 20:
			up = append(up, c)
		case c.Best.AltDeg >= 30 && c.BestUnix < dawn-30*60:
			later = append(later, c)
		}
	}
	slices.SortFunc(up, func(a, b *skypb.Constellation) int { return cmp.Compare(b.Centre.AltDeg, a.Centre.AltDeg) })
	slices.SortFunc(later, func(a, b *skypb.Constellation) int {
		return cmp.Or(cmp.Compare(showpiece(a), showpiece(b)), cmp.Compare(a.BestUnix, b.BestUnix))
	})
	later = later[:min(len(later), 2)]
	slices.SortFunc(later, func(a, b *skypb.Constellation) int { return cmp.Compare(a.BestUnix, b.BestUnix) })
	up = up[:min(len(up), skyBestRows-len(later))]
	return append(up, later...)
}

// skyWhere says where to look for something at p: "high in the south",
// "low in the north-east", "overhead".
func skyWhere(p *skypb.Point) string {
	if p.AltDeg >= 75 {
		return "overhead"
	}
	dirs := []string{"north", "north-east", "east", "south-east", "south", "south-west", "west", "north-west"}
	dir := dirs[int(math.Mod(float64(p.AzDeg)+22.5, 360)/45)%8]
	switch {
	case p.AltDeg >= 50:
		return "high in the " + dir
	case p.AltDeg >= 20:
		return "in the " + dir
	}
	return "low in the " + dir
}

// skyCloud is the forecast for the hour nearest view, as it bears on
// seeing the stars.
func skyCloud(w *weatherpb.WeatherUpdate, view time.Time) (string, color.NRGBA) {
	if w == nil || len(w.Hourly) == 0 {
		return "", colFaint
	}
	near := w.Hourly[0]
	for _, h := range w.Hourly {
		if math.Abs(float64(h.TimeUnix-view.Unix())) < math.Abs(float64(near.TimeUnix-view.Unix())) {
			near = h
		}
	}
	if math.Abs(float64(near.TimeUnix-view.Unix())) > 2*3600 {
		return "", colFaint // the forecast doesn't reach that far
	}
	switch near.Category {
	case weatherpb.Category_CATEGORY_SUNNY:
		return "Clear skies forecast", colUp
	case weatherpb.Category_CATEGORY_PARTLY_CLOUDY:
		return "Some cloud forecast", colSun
	case weatherpb.Category_CATEGORY_FOG:
		return "Fog forecast: stars hidden", colDim
	case weatherpb.Category_CATEGORY_CLOUDY:
		return "Overcast forecast: stars hidden", colDim
	}
	return "Rain forecast: stars hidden", colDim
}

// skyMoon is how much the Moon will get in the way.
func skyMoon(w *weatherpb.WeatherUpdate, k *skypb.SkyUpdate) string {
	var moon *skypb.Body
	for _, b := range k.Bodies {
		if b.Name == "Moon" {
			moon = b
		}
	}
	if moon == nil {
		return "Moon below the horizon"
	}
	text := "Moon " + skyWhere(moon.At)
	if w != nil && w.MoonIllum != "" {
		text = fmt.Sprintf("Moon %s%% lit, %s", w.MoonIllum, skyWhere(moon.At))
	}
	return text
}

// strokeCircle draws a thin circle of radius r centred on cx, cy.
func strokeCircle(gtx layout.Context, col color.NRGBA, cx, cy, r float32) {
	const n = 96
	pts := make([]f32.Point, n+1)
	for i := range pts {
		a := 2 * math.Pi * float64(i) / n
		pts[i] = f32.Pt(cx+r*float32(math.Cos(a)), cy+r*float32(math.Sin(a)))
	}
	strokeLine(gtx.Ops, col, float32(gtx.Dp(1)), pts...)
}

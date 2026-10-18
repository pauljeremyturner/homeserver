package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	planetspb "homeserver/gen/planets"
)

// The planets page plays the planets' history, from info-server, up to
// today: it waits a moment on the first frame, runs through the years
// (easing in and out) for two thirds of the page's time, then holds on
// today, with the signs of the zodiac and Christmas marked on Earth's orbit.
const animDelay = 1500 * time.Millisecond

// planetDot is each planet's dot radius, in dp, Mercury first.
var planetDot = []unit.Dp{3.5, 5, 5.5, 4.5, 8, 7, 5.5, 5.5}

// animFrame is how far through the history the page is (0 to the last
// frame, fractional) shown for elapsed, when the page is up for period, and
// whether it's still moving.
func animFrame(elapsed, period time.Duration, frames int) (float64, bool) {
	length := period*2/3 - animDelay
	p := float64(elapsed-animDelay) / float64(max(length, time.Second))
	if p <= 0 {
		return 0, true
	}
	if p >= 1 {
		return float64(frames), false
	}
	return float64(frames) * (1 - math.Cos(math.Pi*p)) / 2, true
}

// planets draws the inner and outer planets side by side, each to its own
// true scale, at the point in their history that elapsed into the page
// reaches.
func (u *ui) planets(gtx layout.Context, now time.Time, p *planetspb.PlanetsUpdate, elapsed, period time.Duration) layout.Dimensions {
	if p == nil || len(p.Planets) != 8 || len(p.Orbits) != 8 || len(p.History) != 8 || len(p.History[0].XAu) < 2 {
		return layout.Center.Layout(gtx, u.label(18, colFaint, font.Normal, "Waiting for planets...").Layout)
	}
	frames := len(p.History[0].XAu) - 1
	f, moving := animFrame(elapsed, period, frames)
	if moving {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(tickerFrame)})
	}
	step := time.Duration(p.HistoryStepSeconds) * time.Second
	at := time.Unix(p.HistoryStartUnix, 0).Add(time.Duration(f * float64(step))).In(now.Location())

	return layout.Inset{Top: 12, Bottom: 12, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width, height := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
		gap := gtx.Dp(28)
		size := min(height, (width-gap)/2)
		// The inner planets' trails are their last couple of months; the
		// outer planets' go back to the start, showing how far they've come.
		trail := int(60 * 24 * time.Hour / max(step, time.Hour))
		named := !moving && f > 0
		var marks []seasonMark
		if named {
			marks = christmasMarks(p, at)
		}
		u.planetPanel(gtx, p, f, []int{0, 1, 2, 3}, size, trail, false, named, marks)
		off := op.Offset(image.Pt(width-size, 0)).Push(gtx.Ops)
		u.planetPanel(gtx, p, f, []int{4, 5, 6, 7}, size, frames, true, false, nil)
		off.Pop()

		u.labelLeft(gtx, 0, gtx.Dp(8), u.label(13, colDim, font.Medium, "INNER PLANETS"))
		u.labelLeft(gtx, 0, gtx.Dp(26), u.label(12, colFaint, font.Normal, "to scale"))
		u.labelRight(gtx, width, gtx.Dp(8), u.label(13, colDim, font.Medium, "OUTER PLANETS"))
		u.labelRight(gtx, width, gtx.Dp(26), u.label(12, colFaint, font.Normal, "to scale"))

		years := float64(frames) * step.Hours() / (24 * 365.25)
		when := at.Format("2 January")
		whenCol := colDim
		if !moving && f > 0 {
			when, whenCol = "today, "+when, colSun
		}
		u.labelLeft(gtx, 0, size-gtx.Dp(62), u.label(12, colFaint, font.Normal, fmt.Sprintf("the last %.0f years", years)))
		u.labelLeft(gtx, 0, size-gtx.Dp(36), u.label(40, colText, font.Medium, at.Format("2006")))
		u.labelLeft(gtx, 0, size-gtx.Dp(6), u.label(15, whenCol, font.Normal, when))
		return layout.Dimensions{Size: image.Pt(width, size)}
	})
}

// planetPanel draws the planets in idx in a size x size box: their orbits,
// scaled so the widest fills the box, marks on Earth's, the Sun, a trail of
// up to trail frames behind each (fading unless whole), and each planet,
// named, at frame f of its history.
func (u *ui) planetPanel(gtx layout.Context, p *planetspb.PlanetsUpdate, f float64, idx []int, size, trail int, whole, named bool, marks []seasonMark) {
	c := float32(size) / 2
	var reach float32
	for _, i := range idx {
		o := p.Orbits[i]
		for k := range o.XAu {
			reach = max(reach, float32(math.Hypot(float64(o.XAu[k]), float64(o.YAu[k]))))
		}
	}
	scale := (c - float32(gtx.Dp(6))) / reach
	// Ecliptic x is to the right and y up, so the planets go anticlockwise.
	pt := func(x, y float32) f32.Point { return f32.Pt(c+x*scale, c-y*scale) }

	h := p.History[2]
	k0 := int(f)
	k1 := min(k0+1, len(h.XAu)-1)
	earth := arcLerp(f32.Pt(h.XAu[k0], h.YAu[k0]), f32.Pt(h.XAu[k1], h.YAu[k1]), float32(f-float64(k0)))
	for _, i := range idx {
		o := p.Orbits[i]
		if i == 2 && named {
			u.zodiacBand(gtx, o, earth, c, scale)
			continue
		}
		pts := make([]f32.Point, 0, len(o.XAu)+1)
		for k := range o.XAu {
			pts = append(pts, pt(o.XAu[k], o.YAu[k]))
		}
		pts = append(pts, pts[0])
		col := planetColour(i)
		col.A = 70
		strokeLine(gtx.Ops, col, float32(gtx.Dp(1)), pts...)
	}
	if len(marks) > 0 {
		u.markOrbit(gtx, marks, earth, c, scale)
	}
	fillCircle(gtx.Ops, colSun, c, c, float32(gtx.Dp(7)))

	for _, i := range idx {
		h := p.History[i]
		at := func(k int) f32.Point { return f32.Pt(h.XAu[k], h.YAu[k]) }
		k0 := int(f)
		k1 := min(k0+1, len(h.XAu)-1)
		here := arcLerp(at(k0), at(k1), float32(f-float64(k0)))
		pos := pt(here.X, here.Y)

		// The trail, oldest first, ending at the planet; long trails skip
		// frames, which the slow outer planets don't miss. Each step follows
		// the arc between frames rather than cutting across it, which
		// Mercury, going a fifth of the way round between frames, would.
		first := max(k0-trail, 0)
		stride := max((k0-first)/300, 1)
		col := planetColour(i)
		width := float32(gtx.Dp(2))
		alpha := func(k float32) uint8 {
			if whole {
				return 110
			}
			return uint8(140 * (k - float32(first)) / float32(max(k0-first, 1)))
		}
		prevAU, prev := at(first), pt(at(first).X, at(first).Y)
		segment := func(to f32.Point, k float32) {
			// A zero-length stroke upsets Gio's stroker.
			next := pt(to.X, to.Y)
			if d := next.Sub(prev); d.X*d.X+d.Y*d.Y < 0.25 {
				return
			}
			col.A = alpha(k)
			strokeLine(gtx.Ops, col, width, prev, next)
			prevAU, prev = to, next
		}
		for k := first + stride; k <= k0; k += stride {
			from, to := prevAU, at(k)
			n := arcSteps(from, to)
			for j := 1; j <= n; j++ {
				segment(arcLerp(from, to, float32(j)/float32(n)), float32(k))
			}
		}
		segment(here, float32(k0))

		fillCircle(gtx.Ops, col, pos.X, pos.Y, float32(gtx.Dp(planetDot[i])))
		u.planetName(gtx, p.Planets[i].Name, i, pos, f32.Pt(pos.X-c, pos.Y-c), float32(gtx.Dp(planetDot[i]+4)), size)
	}
}

// seasonMark is a point on Earth's orbit, in AU, to mark and name.
type seasonMark struct {
	name   string
	col    color.NRGBA
	inside bool // named on the Sun's side, clear of a mark just outside
	pos    f32.Point
}

var colChristmas = rgb(0xd9534f)

// onOrbit is the point of orbit o, in AU, at heliocentric longitude deg.
func onOrbit(o *planetspb.Path, deg float64) f32.Point {
	th := deg * math.Pi / 180
	best, bestD := 0, math.Inf(1)
	for k := range o.XAu {
		d := math.Abs(math.Remainder(math.Atan2(float64(o.YAu[k]), float64(o.XAu[k]))-th, 2*math.Pi))
		if d < bestD {
			best, bestD = k, d
		}
	}
	r := math.Hypot(float64(o.XAu[best]), float64(o.YAu[best]))
	return f32.Pt(float32(r*math.Cos(th)), float32(r*math.Sin(th)))
}

// zodiac is the signs of the (tropical) zodiac from Aries, which the Sun
// enters at the March equinox, each 30 degrees of ecliptic longitude.
var zodiac = []string{"Aries", "Taurus", "Gemini", "Cancer", "Leo", "Virgo",
	"Libra", "Scorpio", "Sagittarius", "Capricorn", "Aquarius", "Pisces"}

// zodiacBand draws Earth's orbit o, once the page reaches today, as a wide
// band of twelve sections shaded alternately, one per sign: while Earth is in
// a section the Sun, seen from Earth opposite it, is in that sign. Each
// section is named just outside its middle, moved along clear of Earth's own
// name if Earth is near it. The panel is centred on c at scale pixels per AU.
func (u *ui) zodiacBand(gtx layout.Context, o *planetspb.Path, earth f32.Point, c, scale float32) {
	width := float32(gtx.Dp(5))
	gap := float32(gtx.Dp(6))
	lineH := float32(gtx.Dp(14))
	earthTh := math.Atan2(float64(earth.Y), float64(earth.X))
	for s, sign := range zodiac {
		// Earth is at 180 degrees when the Sun enters Aries, at 0.
		from := float64(180 + 30*s)
		col := planetColour(2)
		col.A = 150
		if s%2 == 1 {
			col.A = 55
		}
		pts := make([]f32.Point, 0, 31)
		for d := 0; d <= 30; d++ {
			p := onOrbit(o, from+float64(d))
			pts = append(pts, f32.Pt(c+p.X*scale, c-p.Y*scale))
		}
		strokeLine(gtx.Ops, col, width, pts...)

		th := (from + 15) * math.Pi / 180
		if d := math.Remainder(th-earthTh, 2*math.Pi); math.Abs(d) < 12*math.Pi/180 {
			th = earthTh + math.Copysign(13*math.Pi/180, d)
		}
		p := onOrbit(o, th*180/math.Pi)
		r := float32(math.Hypot(float64(p.X), float64(p.Y))) * scale
		dx, dy := float32(math.Cos(th)), -float32(math.Sin(th))
		l := u.label(11, colDim, font.Normal, sign)
		half := float32(math.Abs(float64(dx)))*float32(u.labelWidth(gtx, l))/2 + float32(math.Abs(float64(dy)))*lineH/2
		rr := r + width/2 + gap + half
		u.labelAt(gtx, int(c+dx*rr), int(c+dy*rr), l)
	}
}

// christmasMarks marks Christmas where Earth was on the last 25 December
// before now, if the history goes back that far.
func christmasMarks(p *planetspb.PlanetsUpdate, now time.Time) []seasonMark {
	var marks []seasonMark
	h := p.History[2]
	step := float64(p.HistoryStepSeconds)
	xmas := time.Date(now.Year(), time.December, 25, 12, 0, 0, 0, now.Location())
	if xmas.After(now) {
		xmas = xmas.AddDate(-1, 0, 0)
	}
	if f := float64(xmas.Unix()-p.HistoryStartUnix) / step; step > 0 && f >= 0 && f <= float64(len(h.XAu)-1) {
		k0 := int(f)
		k1 := min(k0+1, len(h.XAu)-1)
		pos := arcLerp(f32.Pt(h.XAu[k0], h.YAu[k0]), f32.Pt(h.XAu[k1], h.YAu[k1]), float32(f-float64(k0)))
		marks = append(marks, seasonMark{"Christmas", colChristmas, true, pos})
	}
	return marks
}

// markOrbit draws each mark as a tick across Earth's orbit, named just
// outside it (or inside), for a panel centred on c at scale pixels per AU.
// A mark Earth is sitting on isn't named, so it doesn't collide with
// Earth's own name.
func (u *ui) markOrbit(gtx layout.Context, marks []seasonMark, earth f32.Point, c, scale float32) {
	tick := float32(gtx.Dp(4))
	gap := float32(gtx.Dp(6))
	lineH := float32(gtx.Dp(14))
	earthTh := math.Atan2(float64(earth.Y), float64(earth.X))
	for _, m := range marks {
		th := math.Atan2(float64(m.pos.Y), float64(m.pos.X))
		r := float32(math.Hypot(float64(m.pos.X), float64(m.pos.Y))) * scale
		// Screen y runs down, ecliptic y up.
		dx, dy := float32(math.Cos(th)), -float32(math.Sin(th))
		at := func(rr float32) f32.Point { return f32.Pt(c+dx*rr, c+dy*rr) }

		col := m.col
		col.A = 200
		strokeLine(gtx.Ops, col, float32(gtx.Dp(1.5)), at(r-tick), at(r+tick))

		if math.Abs(math.Remainder(th-earthTh, 2*math.Pi)) < 12*math.Pi/180 {
			continue
		}
		l := u.label(11, m.col, font.Normal, m.name)
		// Far enough out (or in) that the label's nearest edge clears the
		// tick, whichever way round the orbit it is.
		half := float32(math.Abs(float64(dx)))*float32(u.labelWidth(gtx, l))/2 + float32(math.Abs(float64(dy)))*lineH/2
		rr := r + tick + gap + half
		if m.inside {
			rr = r - tick - gap - half
		}
		pos := at(rr)
		u.labelAt(gtx, int(pos.X), int(pos.Y), l)
	}
}

// planetName labels a planet beside it on its side away from the Sun (out),
// unless that would run off the size-wide panel.
func (u *ui) planetName(gtx layout.Context, name string, i int, pos, out f32.Point, gap float32, size int) {
	l := u.label(12, planetColour(i), font.Medium, name)
	w := u.labelWidth(gtx, l)
	right := out.X >= 0
	if right && int(pos.X+gap)+w > size {
		right = false
	} else if !right && int(pos.X-gap)-w < 0 {
		right = true
	}
	if right {
		u.labelLeft(gtx, int(pos.X+gap), int(pos.Y), l)
	} else {
		u.labelRight(gtx, int(pos.X-gap), int(pos.Y), l)
	}
}

// arcLerp goes t of the way from a to b (points around the Sun) by angle
// and distance, following the orbit's curve rather than the chord across it.
func arcLerp(a, b f32.Point, t float32) f32.Point {
	ra, rb := math.Hypot(float64(a.X), float64(a.Y)), math.Hypot(float64(b.X), float64(b.Y))
	ta, tb := math.Atan2(float64(a.Y), float64(a.X)), math.Atan2(float64(b.Y), float64(b.X))
	th := ta + math.Remainder(tb-ta, 2*math.Pi)*float64(t)
	r := ra + (rb-ra)*float64(t)
	return f32.Pt(float32(r*math.Cos(th)), float32(r*math.Sin(th)))
}

// arcSteps is how many pieces to draw the arc from a to b in, so each turns
// no more than 3 degrees.
func arcSteps(a, b f32.Point) int {
	ta, tb := math.Atan2(float64(a.Y), float64(a.X)), math.Atan2(float64(b.Y), float64(b.X))
	return max(int(math.Ceil(math.Abs(math.Remainder(tb-ta, 2*math.Pi))/(3*math.Pi/180))), 1)
}

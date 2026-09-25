package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	weatherpb "homeserver/gen/weather"
)

// Vector icons, drawn rather than shipped as images so they stay crisp at
// any size. Every function draws into a size x size box at the current
// offset, and returns that box as its dimensions.

func fillCircle(ops *op.Ops, c color.NRGBA, cx, cy, r float32) {
	rect := image.Rect(int(cx-r), int(cy-r), int(math.Ceil(float64(cx+r))), int(math.Ceil(float64(cy+r))))
	paint.FillShape(ops, c, clip.Ellipse(rect).Op(ops))
}

func polygon(ops *op.Ops, pts []f32.Point) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(pts[0])
	for _, pt := range pts[1:] {
		p.LineTo(pt)
	}
	p.Close()
	return p.End()
}

func fillPolygon(ops *op.Ops, c color.NRGBA, pts []f32.Point) {
	paint.FillShape(ops, c, clip.Outline{Path: polygon(ops, pts)}.Op())
}

func strokeLine(ops *op.Ops, c color.NRGBA, width float32, pts ...f32.Point) {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(pts[0])
	for _, pt := range pts[1:] {
		p.LineTo(pt)
	}
	paint.FillShape(ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}

func drawSun(ops *op.Ops, cx, cy, r float32) {
	fillCircle(ops, colSun, cx, cy, r)
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		sin, cos := float32(math.Sin(a)), float32(math.Cos(a))
		strokeLine(ops, colSun, r*0.28,
			f32.Pt(cx+cos*r*1.35, cy+sin*r*1.35),
			f32.Pt(cx+cos*r*1.75, cy+sin*r*1.75))
	}
}

// drawCloud draws a cloud whose bounding box is roughly w wide, anchored with
// its flat base at (x, baseY).
func drawCloud(ops *op.Ops, c color.NRGBA, x, baseY, w float32) {
	h := w * 0.3
	paint.FillShape(ops, c, clip.UniformRRect(image.Rect(int(x), int(baseY-h), int(x+w), int(baseY)), int(h/2)).Op(ops))
	fillCircle(ops, c, x+w*0.32, baseY-h*0.95, w*0.2)
	fillCircle(ops, c, x+w*0.6, baseY-h*1.2, w*0.26)
}

func weatherIcon(gtx layout.Context, cat weatherpb.Category, size int) layout.Dimensions {
	ops := gtx.Ops
	s := float32(size)
	cloudX, cloudBase, cloudW := s*0.08, s*0.62, s*0.84
	precip := func(draw func(x, y float32)) {
		for i := 0; i < 3; i++ {
			draw(s*0.28+float32(i)*s*0.22, s*0.72)
		}
	}
	switch cat {
	case weatherpb.Category_CATEGORY_SUNNY:
		drawSun(ops, s/2, s/2, s*0.24)
	case weatherpb.Category_CATEGORY_PARTLY_CLOUDY:
		drawSun(ops, s*0.62, s*0.36, s*0.17)
		drawCloud(ops, colCloud, s*0.06, s*0.8, s*0.72)
	case weatherpb.Category_CATEGORY_CLOUDY:
		drawCloud(ops, colCloudDark, s*0.28, s*0.55, s*0.62)
		drawCloud(ops, colCloud, cloudX, s*0.75, cloudW)
	case weatherpb.Category_CATEGORY_FOG:
		for i := 0; i < 4; i++ {
			y := s*0.3 + float32(i)*s*0.14
			inset := float32(i%2) * s * 0.1
			strokeLine(ops, colCloud, s*0.06, f32.Pt(s*0.14+inset, y), f32.Pt(s*0.86-inset, y))
		}
	case weatherpb.Category_CATEGORY_RAIN:
		drawCloud(ops, colCloud, cloudX, cloudBase, cloudW)
		precip(func(x, y float32) {
			strokeLine(ops, colRain, s*0.05, f32.Pt(x, y), f32.Pt(x-s*0.06, y+s*0.16))
		})
	case weatherpb.Category_CATEGORY_THUNDER:
		drawCloud(ops, colCloudDark, cloudX, cloudBase, cloudW)
		fillPolygon(ops, colSun, []f32.Point{
			f32.Pt(s*0.54, s*0.55), f32.Pt(s*0.38, s*0.78), f32.Pt(s*0.5, s*0.78),
			f32.Pt(s*0.42, s*0.97), f32.Pt(s*0.66, s*0.7), f32.Pt(s*0.53, s*0.7), f32.Pt(s*0.62, s*0.55),
		})
	case weatherpb.Category_CATEGORY_SNOW:
		drawCloud(ops, colCloud, cloudX, cloudBase, cloudW)
		precip(func(x, y float32) {
			fillCircle(ops, colSnow, x, y+s*0.05, s*0.045)
			fillCircle(ops, colSnow, x-s*0.07, y+s*0.18, s*0.045)
		})
	case weatherpb.Category_CATEGORY_SLEET:
		drawCloud(ops, colCloud, cloudX, cloudBase, cloudW)
		precip(func(x, y float32) {
			strokeLine(ops, colRain, s*0.05, f32.Pt(x, y), f32.Pt(x-s*0.04, y+s*0.1))
			fillCircle(ops, colSnow, x-s*0.08, y+s*0.2, s*0.04)
		})
	default:
		drawCloud(ops, colCloudDark, cloudX, s*0.7, cloudW)
	}
	return layout.Dimensions{Size: image.Pt(size, size)}
}

// moonIcon draws the moon lit to the given fraction (0 new .. 1 full). The
// lit side is the right while waxing and the left while waning, as seen from
// the northern hemisphere.
func moonIcon(gtx layout.Context, illum float64, waxing bool, size int) layout.Dimensions {
	ops := gtx.Ops
	r := float32(size) / 2
	cx, cy := r, r
	fillCircle(ops, colMoonDark, cx, cy, r)

	side := float32(1)
	if !waxing {
		side = -1
	}
	// The lit area as a single shape (two shapes meeting along a shared
	// edge leave an antialiasing seam): down the lit limb, then back up the
	// terminator. The terminator is a half ellipse with x-radius (1-2*illum)*r
	// on the lit side — the limb itself at new moon, a straight line at a
	// quarter, and the opposite limb at full.
	const steps = 48
	k := side * float32(1-2*illum)
	lit := make([]f32.Point, 0, 2*(steps+1))
	for i := 0; i <= steps; i++ {
		a := -math.Pi/2 + math.Pi*float64(i)/steps
		lit = append(lit, f32.Pt(cx+side*r*float32(math.Cos(a)), cy+r*float32(math.Sin(a))))
	}
	for i := steps; i >= 0; i-- {
		a := -math.Pi/2 + math.Pi*float64(i)/steps
		lit = append(lit, f32.Pt(cx+k*r*float32(math.Cos(a)), cy+r*float32(math.Sin(a))))
	}
	fillPolygon(ops, colMoon, lit)
	return layout.Dimensions{Size: image.Pt(size, size)}
}

// sparkline draws prices (oldest first) as a line with a faint fill below,
// scaled to fill a w x h box.
func sparkline(gtx layout.Context, prices []float64, lo, hi float64, c color.NRGBA, w, h int) layout.Dimensions {
	dims := layout.Dimensions{Size: image.Pt(w, h)}
	if len(prices) < 2 {
		return dims
	}
	if hi <= lo {
		hi = lo + 1
	}
	stroke := float32(gtx.Dp(2))
	pad := stroke
	pts := make([]f32.Point, len(prices))
	for i, p := range prices {
		x := float32(i) / float32(len(prices)-1) * float32(w)
		y := pad + (1-float32((p-lo)/(hi-lo)))*(float32(h)-2*pad)
		pts[i] = f32.Pt(x, y)
	}
	area := append([]f32.Point{f32.Pt(0, float32(h))}, pts...)
	area = append(area, f32.Pt(float32(w), float32(h)))
	fill := c
	fill.A = 40
	fillPolygon(gtx.Ops, fill, area)
	strokeLine(gtx.Ops, c, stroke, pts...)
	fillCircle(gtx.Ops, c, pts[len(pts)-1].X, pts[len(pts)-1].Y, stroke*1.6)
	return dims
}

// Package countdown draws the line along the bottom of a display that
// shows how long the current page (or photo) has left on screen: white for
// the time remaining, from the left, and grey for the time gone, from the
// right. It's worked out from the page's own times, so it needs nothing
// more from the server.
package countdown

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

var (
	colLeft = color.NRGBA{R: 0xe8, G: 0xea, B: 0xed, A: 0xff}
	colGone = color.NRGBA{R: 0x5a, G: 0x61, B: 0x6b, A: 0xff}
)

// Height is the line's thickness.
const Height = 3 // dp

// Layout draws the line across the width it's given, elapsed into period.
// Live, it asks for a redraw when the white part is next due to shrink by
// a pixel; a still frame (a screenshot) doesn't.
func Layout(gtx layout.Context, elapsed, period time.Duration, live bool) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(Height))
	if size.X <= 0 || period <= 0 {
		return layout.Dimensions{Size: size}
	}
	left := 1 - min(max(float64(elapsed)/float64(period), 0), 1)
	split := int(left * float64(size.X))
	paint.FillShape(gtx.Ops, colLeft, clip.Rect(image.Rect(0, 0, split, size.Y)).Op())
	paint.FillShape(gtx.Ops, colGone, clip.Rect(image.Rect(split, 0, size.X, size.Y)).Op())
	if live && elapsed < period {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(period / time.Duration(size.X))})
	}
	return layout.Dimensions{Size: size}
}

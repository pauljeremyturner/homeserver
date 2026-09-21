package main

import "strings"

// sparkScale is a pure-ASCII "ink density" gradient, low to high, used to
// approximate a bar height with a single character - deliberately avoids
// any Unicode block/box-drawing characters, since the raw Linux console
// font in use may not have a correct Unicode glyph table (symbols outside
// Latin-1 can render as garbled accented letters instead).
const sparkScale = " .-:=+*#%@"

// renderSparkline renders a price series as a single-line, pure-ASCII
// sparkline (one character per data point, height approximated by "ink
// density"), safe to display on any console font.
func renderSparkline(prices []float64, min, max float64) string {
	if len(prices) == 0 {
		return ""
	}
	span := max - min
	var sb strings.Builder
	for _, p := range prices {
		level := 0
		if span > 0 {
			level = int((p - min) / span * float64(len(sparkScale)-1))
		}
		sb.WriteByte(sparkScale[level])
	}
	return sb.String()
}

package main

import "strings"

// marqueeText joins headlines into one continuously-scrollable string.
func marqueeText(headlines []string) string {
	return strings.Join(headlines, "   *   ") + "   *   "
}

// marqueeWindow returns a fixed-width slice of the (conceptually looping)
// marquee text starting at rune offset `pos`.
func marqueeWindow(text string, pos, width int) string {
	if text == "" {
		return strings.Repeat(" ", width)
	}
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return strings.Repeat(" ", width)
	}
	pos = pos % n
	out := make([]rune, 0, width)
	for i := 0; i < width; i++ {
		out = append(out, runes[(pos+i)%n])
	}
	return string(out)
}

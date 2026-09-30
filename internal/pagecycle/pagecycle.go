// Package pagecycle works out which page an info display shows when, from
// a rotation of pages that each show for their own number of seconds. The
// rotation is anchored to the Unix epoch, so info-server and a display that
// has lost touch with it agree on the page from the clock alone.
package pagecycle

import (
	"time"

	displaypb "homeserver/gen/display"
)

// At returns the page showing at t, and the times it came up and will
// change. cycle must pass Valid.
func At(t time.Time, cycle []*displaypb.PageSlot) (page displaypb.Page, since, next time.Time) {
	var total int64
	for _, s := range cycle {
		total += s.Seconds
	}
	start := t.Unix() - t.Unix()%total
	for _, s := range cycle {
		if t.Unix() < start+s.Seconds {
			return s.Page, time.Unix(start, 0), time.Unix(start+s.Seconds, 0)
		}
		start += s.Seconds
	}
	panic("pagecycle: t outside its own rotation")
}

// Valid reports whether cycle can be passed to At: at least one slot, and
// every slot a positive number of seconds.
func Valid(cycle []*displaypb.PageSlot) bool {
	for _, s := range cycle {
		if s.Seconds <= 0 {
			return false
		}
	}
	return len(cycle) > 0
}

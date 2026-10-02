package main

import "testing"

func TestParseLocation(t *testing.T) {
	loc, err := parseLocation("", "")
	if loc != nil || err != nil {
		t.Errorf("empty: got %v, %v; want nil, nil (locate by IP)", loc, err)
	}

	loc, err = parseLocation(" 55.8642 , -4.2518 ", " Glasgow ")
	if err != nil {
		t.Fatalf("valid: %v", err)
	}
	if loc.lat != "55.8642" || loc.lon != "-4.2518" || loc.name != "Glasgow" {
		t.Errorf("valid: got %+v", *loc)
	}

	for _, bad := range []string{"55.8642", "55.8642,", "north,west", "91,0", "0,181", "55.8642;-4.2518"} {
		if _, err := parseLocation(bad, ""); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}

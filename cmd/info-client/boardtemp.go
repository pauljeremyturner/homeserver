package main

import (
	"os"
	"strconv"
	"strings"
)

// thermalZonePaths are tried in order; the first one that exists wins.
// thermal_zone0 is cpu-thermal on the ASUS Tinker Board (Rockchip RK3288).
var thermalZonePaths = []string{
	"/sys/class/thermal/thermal_zone0/temp",
	"/sys/class/thermal/thermal_zone1/temp",
}

func readBoardTempC() (float64, error) {
	var lastErr error
	for _, p := range thermalZonePaths {
		data, err := os.ReadFile(p)
		if err != nil {
			lastErr = err
			continue
		}
		milli, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			lastErr = err
			continue
		}
		return float64(milli) / 1000.0, nil
	}
	return 0, lastErr
}

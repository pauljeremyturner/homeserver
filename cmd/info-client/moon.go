package main

import weatherpb "homeserver/gen/weather"

// moonPhaseLabel returns a human-readable name for the given moon phase,
// for display (the enum's own String() returns the Go constant name, e.g.
// "MOON_PHASE_WAXING_GIBBOUS", which isn't fit for the UI).
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
	default:
		return "Unknown"
	}
}

// moonIcon returns a small 5-line ASCII glyph for the given moon phase,
// as classified server-side by cmd/info-server (see its moonPhaseEnum).
// Shading characters (space/./:/*) approximate illumination; pure ASCII
// throughout.
func moonIcon(phase weatherpb.MoonPhase) []string {
	switch phase {
	case weatherpb.MoonPhase_MOON_PHASE_NEW_MOON:
		return []string{
			`  .----.  `,
			` /      \ `,
			`|        |`,
			` \      / `,
			`  '----'  `,
		}
	case weatherpb.MoonPhase_MOON_PHASE_WAXING_CRESCENT:
		return []string{
			`  .----.  `,
			` /     .* `,
			`|       * |`,
			` \     .* `,
			`  '----'  `,
		}
	case weatherpb.MoonPhase_MOON_PHASE_FIRST_QUARTER:
		return []string{
			`  .----.  `,
			` /    .**|`,
			`|      **|`,
			` \    .**|`,
			`  '----'  `,
		}
	case weatherpb.MoonPhase_MOON_PHASE_WAXING_GIBBOUS:
		return []string{
			`  .----.  `,
			` /  .****|`,
			`|    ****|`,
			` \  .****|`,
			`  '----'  `,
		}
	case weatherpb.MoonPhase_MOON_PHASE_FULL_MOON:
		return []string{
			`  .----.  `,
			` /********\ `,
			`|**********|`,
			` \********/ `,
			`  '----'  `,
		}
	case weatherpb.MoonPhase_MOON_PHASE_WANING_GIBBOUS:
		return []string{
			`  .----.  `,
			` |****.  \ `,
			`|****    |`,
			` |****.  / `,
			`  '----'  `,
		}
	case weatherpb.MoonPhase_MOON_PHASE_LAST_QUARTER:
		return []string{
			`  .----.  `,
			`|**.    \ `,
			`|**      |`,
			`|**.    / `,
			`  '----'  `,
		}
	case weatherpb.MoonPhase_MOON_PHASE_WANING_CRESCENT:
		return []string{
			`  .----.  `,
			` |*.     \ `,
			`|  *      |`,
			` |*.     / `,
			`  '----'  `,
		}
	default:
		return []string{
			`  .----.  `,
			` /      \ `,
			`|    ?   |`,
			` \      / `,
			`  '----'  `,
		}
	}
}

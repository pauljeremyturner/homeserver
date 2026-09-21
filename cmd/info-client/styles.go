package main

import "github.com/charmbracelet/lipgloss"

var (
	colorClock    = lipgloss.Color("51")  // bright cyan
	colorDate     = lipgloss.Color("255") // white
	colorDim      = lipgloss.Color("244") // gray
	colorSun      = lipgloss.Color("220") // yellow
	colorTemp     = lipgloss.Color("214") // orange
	colorRule     = lipgloss.Color("39")  // blue
	colorTomorrow = lipgloss.Color("120") // green
	colorMoon     = lipgloss.Color("183") // light purple
	colorErr      = lipgloss.Color("203") // red
	colorTempOK   = lipgloss.Color("120") // green
	colorTempWarn = lipgloss.Color("220") // yellow
	colorTempHot  = lipgloss.Color("203") // red
	colorBtcUp    = lipgloss.Color("120") // green
	colorBtcDown  = lipgloss.Color("203") // red
	colorChart    = lipgloss.Color("214") // orange
	colorNews     = lipgloss.Color("255") // white
	colorNewsTag  = lipgloss.Color("208") // orange-red
)

var (
	clockStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorClock)
	dateStyle     = lipgloss.NewStyle().Foreground(colorDate)
	locationStyle = lipgloss.NewStyle().Foreground(colorDim).Italic(true)
	ruleStyle     = lipgloss.NewStyle().Foreground(colorRule)
	labelStyle    = lipgloss.NewStyle().Foreground(colorDim)
	tempStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorTemp)
	descStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorSun)
	tomorrowStyle = lipgloss.NewStyle().Foreground(colorTomorrow)
	moonStyle     = lipgloss.NewStyle().Foreground(colorMoon)
	errStyle      = lipgloss.NewStyle().Foreground(colorErr)
	chartStyle    = lipgloss.NewStyle().Foreground(colorChart)
	newsTagStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorNewsTag)
	newsStyle     = lipgloss.NewStyle().Foreground(colorNews)
)

func btcChangeStyle(pct float64) lipgloss.Style {
	if pct >= 0 {
		return lipgloss.NewStyle().Bold(true).Foreground(colorBtcUp)
	}
	return lipgloss.NewStyle().Bold(true).Foreground(colorBtcDown)
}

func boardTempStyle(c float64) lipgloss.Style {
	switch {
	case c >= 75:
		return lipgloss.NewStyle().Bold(true).Foreground(colorTempHot)
	case c >= 60:
		return lipgloss.NewStyle().Bold(true).Foreground(colorTempWarn)
	default:
		return lipgloss.NewStyle().Bold(true).Foreground(colorTempOK)
	}
}

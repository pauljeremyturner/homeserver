package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	cryptopb "homeserver/gen/crypto"
	newspb "homeserver/gen/news"
	weatherpb "homeserver/gen/weather"
)

const width = 58
const marqueeWidth = width

// clockTime extracts "HH:MM" from wttr.in's "HH:MM AM/PM" style strings.
func clockTime(s string) string {
	fields := strings.Fields(s)
	if len(fields) > 0 {
		return fields[0]
	}
	return s
}

type tickMsg time.Time
type boardTempTickMsg time.Time
type marqueeTickMsg time.Time
type boardTempResultMsg struct {
	tempC float64
	err   error
}

func tickClock() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}
func tickBoardTemp() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg { return boardTempTickMsg(t) })
}
func tickMarquee() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return marqueeTickMsg(t) })
}
func fetchBoardTempCmd() tea.Cmd {
	return func() tea.Msg {
		t, err := readBoardTempC()
		return boardTempResultMsg{t, err}
	}
}

type model struct {
	now     time.Time
	weather *weatherpb.WeatherUpdate
	news    *newspb.NewsUpdate
	crypto  map[string]*cryptopb.CryptoUpdate
	wallet  *cryptopb.WalletBalanceUpdate

	boardTempC   float64
	boardTempErr error

	marqueePos int
}

func initialModel() model {
	return model{
		now:    time.Now(),
		crypto: make(map[string]*cryptopb.CryptoUpdate),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickClock(), tickBoardTemp(), tickMarquee(),
		fetchBoardTempCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case tickMsg:
		m.now = time.Time(msg)
		return m, tickClock()
	case boardTempTickMsg:
		return m, tea.Batch(tickBoardTemp(), fetchBoardTempCmd())
	case boardTempResultMsg:
		if msg.err != nil {
			m.boardTempErr = msg.err
		} else {
			m.boardTempC = msg.tempC
			m.boardTempErr = nil
		}
		return m, nil
	case marqueeTickMsg:
		m.marqueePos++
		return m, tickMarquee()
	case weatherMsg:
		m.weather = msg
		return m, nil
	case newsMsg:
		m.news = msg
		return m, nil
	case cryptoMsg:
		m.crypto[msg.Symbol] = msg
		return m, nil
	case walletMsg:
		m.wallet = msg
		return m, nil
	}
	return m, nil
}

func rule() string {
	return ruleStyle.Render(strings.Repeat("-", width))
}

func padRight(s string, w int) string {
	l := lipgloss.Width(s)
	if l >= w {
		return s
	}
	return s + strings.Repeat(" ", w-l)
}

func renderCrypto(b *strings.Builder, snap *cryptopb.CryptoUpdate) {
	if snap == nil {
		return
	}
	arrow := "^"
	if snap.ChangePct < 0 {
		arrow = "v"
	}
	header := fmt.Sprintf("%s/GBP  GBP %.0f  ", snap.Symbol, snap.Latest)
	change := fmt.Sprintf("%s %.1f%% (7d)", arrow, snap.ChangePct)
	b.WriteString(labelStyle.Render(header) + btcChangeStyle(snap.ChangePct).Render(change))
	b.WriteString("\n")
	b.WriteString(chartStyle.Render(renderSparkline(snap.Prices, snap.Min, snap.Max)) +
		labelStyle.Render(fmt.Sprintf("  %.0f-%.0f", snap.Min, snap.Max)))
	b.WriteString("\n")
}

func (m model) View() string {
	var b strings.Builder

	clock := clockStyle.Render(m.now.Format("15:04:05"))
	date := dateStyle.Render(m.now.Format("Mon 02 Jan 2006"))
	b.WriteString(padRight(clock, 24))
	b.WriteString(date)
	b.WriteString("\n")

	if m.weather != nil && m.weather.Location != "" {
		b.WriteString(locationStyle.Render(m.weather.Location))
	} else {
		b.WriteString(locationStyle.Render("Locating..."))
	}
	b.WriteString("\n")
	b.WriteString(rule())
	b.WriteString("\n")

	if m.weather == nil {
		b.WriteString(errStyle.Render("Weather unavailable"))
		b.WriteString("\n")
	} else {
		w := m.weather
		lines := icon(w.CurrentCategory)
		info := []string{
			descStyle.Render(w.CurrentDesc),
			tempStyle.Render(w.TempC+"C") + labelStyle.Render(" (feels "+w.FeelsLikeC+"C)"),
			labelStyle.Render("wind ") + w.WindKmph + " km/h " + w.WindDir,
			labelStyle.Render("humidity ") + w.Humidity + "%",
			labelStyle.Render("UV ") + w.UvIndex + labelStyle.Render("  vis ") + w.VisibilityKm + "km",
		}
		for i := 0; i < len(lines); i++ {
			iconCell := descStyle.Render(lines[i])
			text := ""
			if i < len(info) {
				text = info[i]
			}
			b.WriteString(padRight(iconCell, 12))
			b.WriteString(text)
			b.WriteString("\n")
		}
	}

	if m.weather != nil && m.weather.TomorrowDesc != "" {
		w := m.weather
		lines := icon(w.TomorrowCategory)
		info := []string{
			tomorrowStyle.Render("Tomorrow: " + w.TomorrowDesc),
			tempStyle.Render(w.TomorrowMinC + "C/" + w.TomorrowMaxC + "C"),
			labelStyle.Render("wind ") + w.TomorrowWindKmph + " km/h",
			labelStyle.Render("chance of rain ") + w.TomorrowChanceOfRain + "%",
			"",
		}
		for i := 0; i < len(lines); i++ {
			iconCell := tomorrowStyle.Render(lines[i])
			text := ""
			if i < len(info) {
				text = info[i]
			}
			b.WriteString(padRight(iconCell, 12))
			b.WriteString(text)
			b.WriteString("\n")
		}
	}

	b.WriteString(rule())
	b.WriteString("\n")

	if m.weather != nil {
		w := m.weather
		mLines := moonIcon(w.MoonPhase)
		mInfo := []string{
			moonStyle.Render(moonPhaseLabel(w.MoonPhase)),
			labelStyle.Render("illumination ") + w.MoonIllum + "%",
			"",
			labelStyle.Render("sunrise ") + clockTime(w.TodaySunrise),
			labelStyle.Render("sunset  ") + clockTime(w.TodaySunset),
		}
		for i := 0; i < len(mLines); i++ {
			iconCell := moonStyle.Render(mLines[i])
			text := ""
			if i < len(mInfo) {
				text = mInfo[i]
			}
			b.WriteString(padRight(iconCell, 14))
			b.WriteString(text)
			b.WriteString("\n")
		}
	}

	b.WriteString(rule())
	b.WriteString("\n")

	if m.boardTempErr != nil {
		b.WriteString(labelStyle.Render("Board temp: n/a"))
	} else {
		b.WriteString(labelStyle.Render("Board temp: ") +
			boardTempStyle(m.boardTempC).Render(fmt.Sprintf("%.1fC", m.boardTempC)))
	}
	b.WriteString("\n")
	b.WriteString(rule())
	b.WriteString("\n")

	renderCrypto(&b, m.crypto["BTC"])
	renderCrypto(&b, m.crypto["ETH"])

	if m.wallet != nil {
		b.WriteString(labelStyle.Render(m.wallet.Label+"  ") +
			tempStyle.Render(fmt.Sprintf("%.8f BTC", m.wallet.BalanceBtc)))
		b.WriteString("\n")
	}

	b.WriteString(rule())
	b.WriteString("\n")

	if m.news == nil || len(m.news.Headlines) == 0 {
		b.WriteString(errStyle.Render("News unavailable"))
		b.WriteString("\n")
	} else {
		var line1, line2 []string
		for i, h := range m.news.Headlines {
			if i%2 == 0 {
				line1 = append(line1, h)
			} else {
				line2 = append(line2, h)
			}
		}
		w1 := marqueeWindow(marqueeText(line1), m.marqueePos, marqueeWidth-6)
		w2 := marqueeWindow(marqueeText(line2), m.marqueePos, marqueeWidth-6)
		b.WriteString(newsTagStyle.Render("NEWS ") + newsStyle.Render(w1))
		b.WriteString("\n")
		b.WriteString(newsTagStyle.Render("     ") + newsStyle.Render(w2))
		b.WriteString("\n")
	}

	return b.String()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	serverAddr := envOr("INFO_SERVER_ADDR", "localhost:9090")

	conn, err := dialInfoServer(serverAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error connecting to info-server:", err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	startStreams(ctx, conn, p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running program:", err)
		os.Exit(1)
	}
}

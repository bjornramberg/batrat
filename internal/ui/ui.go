package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"batrat/internal/model"
	"batrat/internal/sampler"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	topStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

type tickMsg time.Time

type app struct {
	s        *sampler.Sampler
	interval time.Duration
	top      int
	paused   bool
	rankMode string
	width    int
	height   int
}

func New(s *sampler.Sampler, interval time.Duration, top int) app {
	return app{s: s, interval: interval, top: top, rankMode: "power"}
}

func (m app) Init() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		if !m.paused {
			m.s.Tick()
		}
		return m, tea.Tick(m.interval, func(t time.Time) tea.Msg { return tickMsg(t) })
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case " ":
			m.paused = !m.paused
			return m, nil
		case "r":
			m.s.Reset()
			return m, nil
		case "s":
			if m.rankMode == "power" {
				m.rankMode = "energy"
			} else {
				m.rankMode = "power"
			}
			return m, nil
		case "+", "=":
			if m.interval < 10*time.Second {
				m.interval *= 2
				return m, tea.Tick(m.interval, func(t time.Time) tea.Msg { return tickMsg(t) })
			}
		case "-":
			if m.interval > 250*time.Millisecond {
				m.interval /= 2
				return m, tea.Tick(m.interval, func(t time.Time) tea.Msg { return tickMsg(t) })
			}
		}
	}
	return m, nil
}

func (m app) View() string {
	if m.width == 0 {
		return "loading..."
	}
	var b strings.Builder
	elapsed := time.Since(m.s.StartTime()).Round(time.Second)
	fmt.Fprintf(&b, "%s  %.1f W   %.2f Wh total   %s elapsed   sort: %s",
		headerStyle.Render("batrat"), m.s.SysPower(), m.s.TotalEnergyJ()/3600, elapsed, m.rankMode)
	if m.paused {
		b.WriteString("   " + topStyle.Render("PAUSED"))
	}
	b.WriteString("\n\n")

	procs := m.s.LastSample().Procs
	if m.rankMode == "energy" {
		procs = make([]model.ProcSample, len(procs))
		copy(procs, m.s.LastSample().Procs)
		sort.Slice(procs, func(i, j int) bool { return procs[i].EnergyJ > procs[j].EnergyJ })
	}
	maxP := 0.0
	for _, p := range procs {
		if p.Power > maxP {
			maxP = p.Power
		}
	}
	if maxP <= 0 {
		maxP = 1
	}

	n := m.top
	if n > len(procs) {
		n = len(procs)
	}
	fmt.Fprintf(&b, "  %-4s  %-22s  %-7s  %-6s  %-7s  %-9s  %s\n",
		"RANK", "NAME", "PID", "CPU%", "POWER", "ENERGY", "LAST 40s")
	for i := 0; i < n; i++ {
		p := procs[i]
		name := p.Name
		if r := []rune(name); len(r) > 22 {
			name = string(r[:22])
		}
		style := lipgloss.NewStyle()
		if i == 0 {
			style = topStyle
		}
		fmt.Fprintf(&b, "  %-4d  %s  %-7d  %-5.1f%%  %-6.1f W  %-8s  %s\n",
			i+1, style.Width(22).Render(name), p.PID, p.CPUPct, p.Power,
			model.FormatWh(p.EnergyJ/3600), sparkline(m.s.History(p.PID), maxP))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("[q]uit  [space] pause  [r]eset  [s]ort  [+/-] interval"))
	return b.String()
}

func sparkline(hist []float64, max float64) string {
	if len(hist) == 0 {
		return ""
	}
	start := len(hist) - 40
	if start < 0 {
		start = 0
	}
	var b strings.Builder
	for _, v := range hist[start:] {
		idx := int(v / max * float64(len(sparkChars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		b.WriteRune(sparkChars[idx])
	}
	return b.String()
}

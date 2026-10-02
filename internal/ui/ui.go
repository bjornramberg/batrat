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
	upStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	downStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	chartStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("45"))
)

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
		return m, tea.ClearScreen
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
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}
	var b strings.Builder
	elapsed := time.Duration(0)
	if started := m.s.StartTime(); !started.IsZero() {
		elapsed = time.Since(started).Round(time.Second)
	}
	fmt.Fprintf(&b, "%s   draw %.1f W   total %s   up %s   sort: %s   src: %s",
		headerStyle.Render("batrat"), m.s.SysPower(),
		model.FormatWh(m.s.TotalEnergyJ()/3600), elapsed, m.rankMode, m.s.Source())
	if m.paused {
		b.WriteString("   " + topStyle.Render("PAUSED"))
	}
	b.WriteString("\n\n")

	chartHeight := m.height / 4
	if chartHeight < 3 {
		chartHeight = 3
	}
	if chartHeight > 10 {
		chartHeight = 10
	}
	header := fmt.Sprintf("  %-4s  %-22s  %-7s  %-6s  %-7s  %-9s  %s",
		"RANK", "NAME", "PID", "CPU%", "POWER", "ENERGY", "TREND")
	chartWidth := lipgloss.Width(header) - 2
	if chartWidth < 1 {
		chartWidth = 1
	}
	b.WriteString(chartStyle.Render(indent(renderAreaChart(m.s.SysHistory(), chartWidth, chartHeight), 2)))
	b.WriteString("\n\n")

	procs := m.s.LastSample().Procs
	if m.rankMode == "energy" {
		procs = make([]model.ProcSample, len(procs))
		copy(procs, m.s.LastSample().Procs)
		sort.Slice(procs, func(i, j int) bool { return procs[i].EnergyJ > procs[j].EnergyJ })
	}

	n := m.height - chartHeight - 5
	if n < 0 {
		n = 0
	}
	if n > m.top {
		n = m.top
	}
	if n > len(procs) {
		n = len(procs)
	}
	b.WriteString(header + "\n")
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
			model.FormatWh(p.EnergyJ/3600), trendIndicator(m.s.History(p.PID)))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("[q]uit  [space] pause  [r]eset  [s]ort  [+/-] interval"))
	return b.String()
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}

func trendIndicator(hist []float64) string {
	if len(hist) < 2 {
		return dimStyle.Render("▬")
	}
	recent := avgLast(hist, 3)
	prev := avgPrev(hist, 3)
	switch {
	case recent > prev*1.1:
		return upStyle.Render("▲")
	case recent < prev*0.9:
		return downStyle.Render("▼")
	default:
		return dimStyle.Render("▬")
	}
}

func avgLast(v []float64, k int) float64 {
	if len(v) == 0 {
		return 0
	}
	if k > len(v) {
		k = len(v)
	}
	var sum float64
	for _, x := range v[len(v)-k:] {
		sum += x
	}
	return sum / float64(k)
}

func avgPrev(v []float64, k int) float64 {
	if len(v) < 2*k {
		return avgLast(v, k)
	}
	var sum float64
	for _, x := range v[len(v)-2*k : len(v)-k] {
		sum += x
	}
	return sum / float64(k)
}

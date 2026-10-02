package report

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"batrat/internal/model"
)

type Entry struct {
	Rank     int
	Name     string
	PID      int
	CPUPct   float64
	AvgW     float64
	EnergyJ  float64
	EnergyWh float64
	SharePct float64
}

type Report struct {
	Start        time.Time
	End          time.Time
	Duration     time.Duration
	Interval     time.Duration
	IdleW        float64
	MaxW         float64
	TotalEnergyJ float64
	Entries      []Entry
}

func Build(data model.ReportData, top int) Report {
	r := Report{
		Start:        data.Start,
		End:          time.Now(),
		Interval:     data.Interval,
		IdleW:        data.IdleW,
		MaxW:         data.MaxW,
		TotalEnergyJ: data.TotalEnergyJ,
	}
	r.Duration = r.End.Sub(r.Start)
	dur := r.Duration.Seconds()
	if dur <= 0 {
		dur = 1
	}
	n := top
	if n > len(data.Procs) {
		n = len(data.Procs)
	}
	for i := 0; i < n; i++ {
		p := data.Procs[i]
		share := 0.0
		if data.TotalEnergyJ > 0 {
			share = p.EnergyJ / data.TotalEnergyJ * 100
		}
		r.Entries = append(r.Entries, Entry{
			Rank:     i + 1,
			Name:     p.Name,
			PID:      p.PID,
			CPUPct:   p.CPUPct,
			AvgW:     p.EnergyJ / dur,
			EnergyJ:  p.EnergyJ,
			EnergyWh: p.EnergyJ / 3600,
			SharePct: share,
		})
	}
	return r
}

func (r Report) Render(format string) string {
	if format == "csv" {
		return r.CSV()
	}
	return r.Text()
}

func (r Report) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "batrat report\n")
	fmt.Fprintf(&b, "  started   %s\n", r.Start.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "  ended     %s\n", r.End.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "  duration  %s\n", r.Duration.Round(time.Second))
	fmt.Fprintf(&b, "  interval  %s\n", r.Interval)
	fmt.Fprintf(&b, "  power     idle %.1f W, max %.1f W\n", r.IdleW, r.MaxW)
	fmt.Fprintf(&b, "  total     %.2f Wh\n\n", r.TotalEnergyJ/3600)
	fmt.Fprintf(&b, "  %-4s  %-24s  %-7s  %-6s  %-7s  %-10s  %-6s\n",
		"RANK", "NAME", "PID", "CPU%", "AVG W", "ENERGY", "SHARE")
	for _, e := range r.Entries {
		fmt.Fprintf(&b, "  %-4d  %-24s  %-7d  %-6.1f  %-7.1f  %-10s  %-5.1f%%\n",
			e.Rank, trunc(e.Name, 24), e.PID, e.CPUPct, e.AvgW,
			model.FormatWh(e.EnergyWh), e.SharePct)
	}
	return b.String()
}

func (r Report) CSV() string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	w.Write([]string{"rank", "name", "pid", "cpu_pct", "avg_w", "energy_j", "energy_wh", "share_pct"})
	for _, e := range r.Entries {
		w.Write([]string{
			strconv.Itoa(e.Rank),
			e.Name,
			strconv.Itoa(e.PID),
			fmt.Sprintf("%.2f", e.CPUPct),
			fmt.Sprintf("%.3f", e.AvgW),
			fmt.Sprintf("%.1f", e.EnergyJ),
			fmt.Sprintf("%.4f", e.EnergyWh),
			fmt.Sprintf("%.2f", e.SharePct),
		})
	}
	w.Flush()
	return b.String()
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

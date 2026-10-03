package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"batrat/internal/daemon"
	"batrat/internal/report"
	"batrat/internal/sampler"
	"batrat/internal/ui"
)

func main() {
	var (
		daemonMode = flag.Bool("d", false, "daemon mode: detach from terminal and write report to file")
		duration   = flag.String("t", "15m", "daemon duration: minutes (15) or Go duration (15m, 1h30m)")
		outPath    = flag.String("o", "", "report output path (daemon default: batrat-<timestamp>.txt)")
		format     = flag.String("f", "text", "report format: text or csv")
		interval   = flag.Duration("interval", time.Second, "sampling interval")
		idleW      = flag.Float64("idle-w", 8, "system idle power in watts")
		maxW       = flag.Float64("max-w", 105, "system max power in watts")
		top        = flag.Int("top", 15, "number of processes shown")
		noRAPL     = flag.Bool("no-rapl", false, "force heuristic power estimate, ignore RAPL")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `batrat — lightweight terminal power profiler for Linux

Samples CPU time per process, converts it to estimated power draw, and shows
a live dashboard of what's draining your machine — or runs headless and
writes a report.

Usage:
  batrat                              interactive live TUI (q to quit, prints report)
  batrat -d -t 15                     daemon: sample 15 min, write report to file
  batrat -d -t 1h30m -o report.csv -f csv

Interactive keys:
  q / ctrl+c   quit and print report
  w            write report to file (default: batrat-<timestamp>.<ext>)
  space        pause
  r            reset counters
  s            toggle ranking: power (per-tick) / energy (cumulative)
  + / -        adjust sampling interval

Flags:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg := sampler.Config{Interval: *interval, IdleWatts: *idleW, MaxWatts: *maxW, NoRAPL: *noRAPL}

	if *daemonMode {
		d, err := parseDuration(*duration)
		if err != nil {
			fmt.Fprintln(os.Stderr, "batrat: invalid -t:", err)
			os.Exit(1)
		}
		out := *outPath
		if out == "" {
			out = fmt.Sprintf("batrat-%s.txt", time.Now().Format("20060102-150405"))
		}
		if err := daemon.Start(daemon.Options{
			Config: cfg, Duration: d, OutPath: out, Format: *format, Top: *top,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "batrat:", err)
			os.Exit(1)
		}
		return
	}

	s := sampler.New(cfg)
	m := ui.New(s, *interval, *top, *outPath, *format)
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "batrat:", err)
		os.Exit(1)
	}

	rep := report.Build(s.ReportData(), *top)
	fmt.Println(rep.Render(*format))
	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(rep.Render(*format)), 0644); err != nil {
			fmt.Fprintln(os.Stderr, "batrat: writing report:", err)
		}
	}
}

func parseDuration(s string) (time.Duration, error) {
	if m, err := strconv.Atoi(s); err == nil {
		return time.Duration(m) * time.Minute, nil
	}
	return time.ParseDuration(s)
}

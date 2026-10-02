package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"batrat/internal/report"
	"batrat/internal/sampler"
)

type Options struct {
	Config  sampler.Config
	Duration time.Duration
	OutPath string
	Format  string
	Top     int
}

const envKey = "BATRAT_DETACHED"

func Start(o Options) error {
	if os.Getenv(envKey) == "1" {
		return run(o)
	}
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), envKey+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer devnull.Close()
	cmd.Stdout, cmd.Stderr = devnull, devnull
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Printf("batrat: sampling for %s, report will be written to %s\n", o.Duration, o.OutPath)
	return nil
}

func run(o Options) error {
	s := sampler.New(o.Config)
	ticker := time.NewTicker(o.Config.Interval)
	defer ticker.Stop()
	deadline := time.Now().Add(o.Duration)
	for {
		s.Tick()
		if !time.Now().Before(deadline) {
			break
		}
		<-ticker.C
	}
	rep := report.Build(s.ReportData(), o.Top)
	return os.WriteFile(o.OutPath, []byte(rep.Render(o.Format)), 0644)
}

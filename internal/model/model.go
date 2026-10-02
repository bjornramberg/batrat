package model

import (
	"fmt"
	"time"
)

type ProcSample struct {
	PID      int
	Name     string
	CPUDelta float64
	Power    float64
	EnergyJ  float64
	CPUPct   float64
}

type Sample struct {
	Time      time.Time
	SysPower  float64
	SysEnergy float64
	Procs     []ProcSample
}

type ProcStat struct {
	PID     int
	Name    string
	CPUPct  float64
	EnergyJ float64
}

type ReportData struct {
	Start        time.Time
	Interval     time.Duration
	IdleW        float64
	MaxW         float64
	TotalEnergyJ float64
	Procs        []ProcStat
}

func FormatWh(wh float64) string {
	if wh >= 1 {
		return fmt.Sprintf("%.2f Wh", wh)
	}
	return fmt.Sprintf("%.0f mWh", wh*1000)
}

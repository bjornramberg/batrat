package sampler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"batrat/internal/model"
	"batrat/internal/power"
)

const (
	historyLen    = 120
	sysHistoryLen = 3600
)

type Config struct {
	Interval  time.Duration
	IdleWatts float64
	MaxWatts  float64
	NoRAPL    bool
}

type Sampler struct {
	cfg     Config
	power   *power.Manager
	clkTck  float64
	started time.Time
	last    time.Time
	hasPrev bool

	prevTotal uint64
	prevIdle  uint64
	prevProcs map[int]uint64

	sysEnergy  float64
	energy     map[int]float64
	cpuTotal   map[int]uint64
	names      map[int]string
	history    map[int][]float64
	sysHistory []float64

	lastSample model.Sample
}

func New(cfg Config) *Sampler {
	return &Sampler{
		cfg:      cfg,
		power:    power.NewManager(cfg.IdleWatts, cfg.MaxWatts, cfg.NoRAPL),
		clkTck:   100.0,
		prevProcs: make(map[int]uint64),
		energy:    make(map[int]float64),
		cpuTotal:  make(map[int]uint64),
		names:     make(map[int]string),
		history:   make(map[int][]float64),
	}
}

func (s *Sampler) Tick() model.Sample {
	now := time.Now()
	total, idle := readProcStat()
	cur := readProcCPUs(s)

	sample := model.Sample{Time: now}
	if s.hasPrev {
		dt := now.Sub(s.last).Seconds()
		var totalDelta, idleDelta uint64
		if total > s.prevTotal {
			totalDelta = total - s.prevTotal
		}
		if idle > s.prevIdle {
			idleDelta = idle - s.prevIdle
		}
		util := 0.0
		if totalDelta > 0 {
			util = 1.0 - float64(idleDelta)/float64(totalDelta)
			if util < 0 {
				util = 0
			}
			if util > 1 {
				util = 1
			}
		}
		sysPower := s.power.SysPower(util)
		sample.SysPower = sysPower
		s.sysEnergy += sysPower * dt
		s.sysHistory = append(s.sysHistory, sysPower)
		if len(s.sysHistory) > sysHistoryLen {
			s.sysHistory = s.sysHistory[len(s.sysHistory)-sysHistoryLen:]
		}

		procs := make([]model.ProcSample, 0, len(cur))
		for pid, j := range cur {
			var delta uint64
			if prev, ok := s.prevProcs[pid]; ok && j >= prev {
				delta = j - prev
			}
			var power, pct float64
			if totalDelta > 0 {
				pct = float64(delta) / float64(totalDelta) * 100
				power = float64(delta) / float64(totalDelta) * sysPower
			}
			s.energy[pid] += power * dt
			s.cpuTotal[pid] += delta
			s.history[pid] = append(s.history[pid], power)
			if len(s.history[pid]) > historyLen {
				s.history[pid] = s.history[pid][len(s.history[pid])-historyLen:]
			}
			procs = append(procs, model.ProcSample{
				PID:      pid,
				Name:     s.names[pid],
				CPUDelta: float64(delta),
				Power:    power,
				EnergyJ:  s.energy[pid],
				CPUPct:   pct,
			})
		}
		sort.Slice(procs, func(i, j int) bool { return procs[i].Power > procs[j].Power })
		sample.Procs = procs
	}

	s.prevTotal, s.prevIdle = total, idle
	s.prevProcs = cur
	s.last = now
	if !s.hasPrev {
		s.started = now
	}
	s.hasPrev = true
	s.lastSample = sample
	return sample
}

func (s *Sampler) Reset() {
	s.energy = make(map[int]float64)
	s.cpuTotal = make(map[int]uint64)
	s.history = make(map[int][]float64)
	s.sysHistory = nil
	s.sysEnergy = 0
	s.started = time.Now()
	s.lastSample = model.Sample{}
}

func (s *Sampler) StartTime() time.Time  { return s.started }
func (s *Sampler) TotalEnergyJ() float64 { return s.sysEnergy }
func (s *Sampler) SysPower() float64     { return s.lastSample.SysPower }
func (s *Sampler) LastSample() model.Sample { return s.lastSample }
func (s *Sampler) History(pid int) []float64 { return s.history[pid] }
func (s *Sampler) SysHistory() []float64    { return s.sysHistory }
func (s *Sampler) Source() string           { return s.power.Source() }

func (s *Sampler) ReportData() model.ReportData {
	procs := make([]model.ProcStat, 0, len(s.energy))
	for pid, e := range s.energy {
		if e <= 0 {
			continue
		}
		procs = append(procs, model.ProcStat{
			PID:     pid,
			Name:    s.names[pid],
			EnergyJ: e,
		})
	}
	var totalCPU uint64
	for _, v := range s.cpuTotal {
		totalCPU += v
	}
	if totalCPU > 0 {
		for i := range procs {
			procs[i].CPUPct = float64(s.cpuTotal[procs[i].PID]) / float64(totalCPU) * 100
		}
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].EnergyJ > procs[j].EnergyJ })
	return model.ReportData{
		Start:        s.started,
		Interval:     s.cfg.Interval,
		IdleW:        s.cfg.IdleWatts,
		MaxW:         s.cfg.MaxWatts,
		TotalEnergyJ: s.sysEnergy,
		Procs:        procs,
	}
}

func readProcStat() (total, idle uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		if len(fields) < 7 {
			return 0, 0
		}
		vals := make([]uint64, len(fields))
		for i, f := range fields {
			vals[i], _ = strconv.ParseUint(f, 10, 64)
		}
		total = vals[0] + vals[1] + vals[2] + vals[3] + vals[4] + vals[5] + vals[6]
		if len(vals) > 7 {
			total += vals[7]
		}
		idle = vals[3] + vals[4]
		return total, idle
	}
	return 0, 0
}

func readProcCPUs(s *Sampler) map[int]uint64 {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	cur := make(map[int]uint64)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if j, comm, ok := readProcCPU(pid); ok {
			cur[pid] = j
			if _, known := s.names[pid]; !known {
				s.names[pid] = resolveName(pid, comm)
			}
		}
	}
	return cur
}

func readProcCPU(pid int) (jiffies uint64, comm string, ok bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, "", false
	}
	str := string(data)
	l := strings.IndexByte(str, '(')
	r := strings.LastIndexByte(str, ')')
	if l < 0 || r < l {
		return 0, "", false
	}
	comm = str[l+1 : r]
	rest := strings.Fields(str[r+2:])
	if len(rest) < 13 {
		return 0, "", false
	}
	utime, _ := strconv.ParseUint(rest[11], 10, 64)
	stime, _ := strconv.ParseUint(rest[12], 10, 64)
	return utime + stime, comm, true
}

func resolveName(pid int, comm string) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err == nil {
		if fields := strings.Split(string(data), "\x00"); len(fields) > 0 && fields[0] != "" {
			return filepath.Base(fields[0])
		}
	}
	return comm
}

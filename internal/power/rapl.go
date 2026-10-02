package power

import (
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultSysfs = "/sys/class/powercap"

type RAPLSource struct {
	path     string
	energy   *os.File
	maxRange float64
	prev     float64
	prevTime time.Time
	hasPrev  bool
	alive    bool
}

func NewRAPLSource(sysfs string) *RAPLSource {
	if sysfs == "" {
		sysfs = defaultSysfs
	}
	entries, err := os.ReadDir(sysfs)
	if err != nil {
		return &RAPLSource{}
	}
	var pkgPath string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "intel-rapl:") && !strings.Contains(name, ":") {
			pkgPath = sysfs + "/" + name
			break
		}
	}
	if pkgPath == "" {
		return &RAPLSource{}
	}
	f, err := os.Open(pkgPath + "/energy_uj")
	if err != nil {
		return &RAPLSource{}
	}
	maxRange := 0.0
	if data, err := os.ReadFile(pkgPath + "/max_energy_range_uj"); err == nil {
		maxRange, _ = strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	}
	return &RAPLSource{path: pkgPath, energy: f, maxRange: maxRange, alive: true}
}

func (r *RAPLSource) Alive() bool { return r.alive }

func (r *RAPLSource) Power() (float64, bool) {
	if !r.alive {
		return 0, false
	}
	if _, err := r.energy.Seek(0, io.SeekStart); err != nil {
		r.alive = false
		return 0, false
	}
	buf := make([]byte, 64)
	n, err := r.energy.Read(buf)
	if err != nil {
		r.alive = false
		return 0, false
	}
	val, err := strconv.ParseFloat(strings.TrimSpace(string(buf[:n])), 64)
	if err != nil {
		r.alive = false
		return 0, false
	}
	now := time.Now()
	if !r.hasPrev {
		r.prev = val
		r.prevTime = now
		r.hasPrev = true
		return 0, false
	}
	dt := now.Sub(r.prevTime).Seconds()
	if dt <= 0 {
		return 0, false
	}
	var delta float64
	if val >= r.prev {
		delta = val - r.prev
	} else if r.maxRange > 0 {
		delta = val + r.maxRange - r.prev
	} else {
		delta = val - r.prev
	}
	r.prev = val
	r.prevTime = now
	if delta < 0 {
		delta = 0
	}
	return delta / 1e6 / dt, true
}

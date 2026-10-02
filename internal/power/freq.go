package power

import (
	"os"
	"strconv"
	"strings"
)

type FreqReader struct {
	cpufreqDir  string
	procStat    string
	cores       []string
	maxKHz      float64
}

func NewFreqReader() *FreqReader {
	fr := &FreqReader{
		cpufreqDir: "/sys/devices/system/cpu",
		procStat:   "/proc/stat",
	}
	fr.init()
	return fr
}

func (f *FreqReader) init() {
	entries, err := os.ReadDir(f.cpufreqDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "cpu") || !isNumeric(name[3:]) {
			continue
		}
		p := f.cpufreqDir + "/" + name + "/cpufreq/scaling_cur_freq"
		if _, err := os.Stat(p); err == nil {
			f.cores = append(f.cores, name)
		}
	}
	for _, c := range f.cores {
		if khz := readKHz(f.cpufreqDir + "/" + c + "/cpufreq/cpuinfo_max_freq"); khz > f.maxKHz {
			f.maxKHz = khz
		}
	}
}

func (f *FreqReader) Ratio() float64 {
	if len(f.cores) == 0 || f.maxKHz <= 0 {
		return 1
	}
	busy := f.perCoreBusy()
	var sumW, sumB float64
	for _, c := range f.cores {
		n, _ := strconv.Atoi(c[3:])
		b := busy[n]
		sumW += readKHz(f.cpufreqDir+"/"+c+"/cpufreq/scaling_cur_freq") * b
		sumB += b
	}
	var avg float64
	if sumB > 0 {
		avg = sumW / sumB
	} else {
		var sum float64
		for _, c := range f.cores {
			sum += readKHz(f.cpufreqDir + "/" + c + "/cpufreq/scaling_cur_freq")
		}
		avg = sum / float64(len(f.cores))
	}
	r := avg / f.maxKHz
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	return r
}

func (f *FreqReader) perCoreBusy() map[int]float64 {
	data, err := os.ReadFile(f.procStat)
	if err != nil {
		return nil
	}
	busy := make(map[int]float64)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || !strings.HasPrefix(fields[0], "cpu") {
			continue
		}
		name := fields[0][3:]
		if !isNumeric(name) {
			continue
		}
		n, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		var total float64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseFloat(fields[i], 64)
			total += v
		}
		idle := parseFloat(fields[4]) + parseFloat(fields[5])
		busy[n] = total - idle
	}
	return busy
}

func readKHz(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	return v
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

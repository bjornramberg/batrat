package power

type Manager struct {
	rapl   *RAPLSource
	freq   *FreqReader
	idleW  float64
	maxW   float64
	noRAPL bool
}

func NewManager(idleW, maxW float64, noRAPL bool) *Manager {
	m := &Manager{
		freq:   NewFreqReader(),
		idleW:  idleW,
		maxW:   maxW,
		noRAPL: noRAPL,
	}
	if !noRAPL {
		m.rapl = NewRAPLSource("")
	}
	return m
}

func (m *Manager) SysPower(util float64) float64 {
	if m.rapl != nil {
		if w, ok := m.rapl.Power(); ok {
			return w
		}
	}
	return m.heuristic(util)
}

func (m *Manager) heuristic(util float64) float64 {
	r := m.freq.Ratio()
	return m.idleW + util*(m.maxW-m.idleW)*r
}

func (m *Manager) Source() string {
	if m.rapl != nil && m.rapl.Alive() {
		return "rapl"
	}
	return "est"
}

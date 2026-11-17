package numeral

import "testing"

// fastGate summarizes the owner flags checkOwnerReady reads (and the
// synchronized fused scope); smallFastReady must answer as if it read
// every flag, for each flag set alone, and a raw-scope mark must clear
// itself once no raw local is pending.
func TestRationalFastGateMatchesOwnerFlags(t *testing.T) {
	if !smallFastBuild() {
		t.Skip("fast paths are compiled out of g3trace/g3debug builds")
	}
	flags := []struct {
		name string
		p    func(m *RationalManager) *bool
	}{
		{"Closing", func(m *RationalManager) *bool { return &m.Closing }},
		{"SourceFinalizing", func(m *RationalManager) *bool { return &m.SourceFinalizing }},
		{"SourceFinalized", func(m *RationalManager) *bool { return &m.SourceFinalized }},
		{"parseOuterPending", func(m *RationalManager) *bool { return &m.parseOuterPending }},
		{"parseSuffixPending", func(m *RationalManager) *bool { return &m.parseSuffixPending }},
		{"parseB", func(m *RationalManager) *bool { return &m.parseB.pending }},
		{"parseExp", func(m *RationalManager) *bool { return &m.parseExp.pending }},
		{"parseTen", func(m *RationalManager) *bool { return &m.parseTen.pending }},
		{"prevPowerPending", func(m *RationalManager) *bool { return &m.prevPowerPending }},
		{"addIntPending", func(m *RationalManager) *bool { return &m.addIntPending }},
		{"fusedPending", func(m *RationalManager) *bool { return &m.fusedPending }},
		{"fusedExecuting", func(m *RationalManager) *bool { return &m.fusedExecuting }},
		{"hwfPending", func(m *RationalManager) *bool { return &m.hwfPending }},
		{"hwfExecuting", func(m *RationalManager) *bool { return &m.hwfExecuting }},
	}
	for _, synch := range []bool{false, true} {
		m, _ := rationalManager(t, synch)
		if m.fastGate != 0 || !m.smallFastReady() {
			t.Fatalf("synch=%v: fresh manager not ready (gate %d)", synch, m.fastGate)
		}
		for _, f := range flags {
			*f.p(m) = true
			m.refreshFastGate()
			if got, want := m.smallFastReady(), m.smallFastReadyFull(); got != want {
				t.Fatalf("synch=%v %s: smallFastReady %v, full %v", synch, f.name, got, want)
			}
			*f.p(m) = false
			m.refreshFastGate()
			if m.fastGate != 0 || !m.smallFastReady() {
				t.Fatalf("synch=%v %s: gate %d after clearing", synch, f.name, m.fastGate)
			}
		}
		m.RawArithmetic.NormActive = true
		m.fastGate |= fastGateRaw
		if m.smallFastReady() {
			t.Fatalf("synch=%v: ready with a pending raw local", synch)
		}
		m.RawArithmetic.NormActive = false
		if !m.smallFastReady() || m.fastGate != 0 {
			t.Fatalf("synch=%v: raw mark did not clear (gate %d)", synch, m.fastGate)
		}
	}
}

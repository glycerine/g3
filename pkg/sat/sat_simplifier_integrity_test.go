package sat

// Source: src/sat/sat_simplifier.cpp:148,161,169,222; sat_solver.cpp:711–716.

import (
	"testing"
)

func TestSATSimplifierActualEnabledIntegritySites(t *testing.T) {
	d, g := satDecisionFixture(t, 0)
	l := g.lifecycle()
	p := l.propagation
	params, e := p.config.context.NewValue()
	satOK(t, e)
	s, e := newSATSimplifier(l, g.cleaner, d, &params)
	satOK(t, e)
	satOK(t, p.config.context.Del(&params))
	t.Cleanup(func() { satOK(t, s.Close()) })
	satOK(t, l.bindIntegrity(g))
	debug, _, _ := satIntegrityActionFixture(t, "")
	satOK(t, debug.EnableDebug("sat_solver"))
	before := p.limit.Count()
	satOK(t, s.Run(false))
	want := uint64(1) // subsume unconditionally checkpoints before its empty-todo exit.
	if satStorageSourceDebug {
		want += 4
	}
	if got := p.limit.Count() - before; got != want {
		t.Fatal("four actual enabled source invariant sites", got, want)
	}
}

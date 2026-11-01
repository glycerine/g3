package sat

// Source: src/sat/sat_asymm_branch.cpp:62–119; src/sat/sat_solver.cpp:711–716.

import (
	"testing"
)

func TestSATIntegrityAsymmActualEnabledWrapperSites(t *testing.T) {
	g, _ := satGCFixture(t, 3)
	l := g.lifecycle()
	p := l.propagation
	ctx := p.config.context
	params, e := ctx.NewValue()
	satOK(t, e)
	t.Cleanup(func() { satOK(t, ctx.Del(&params)) })
	a, e := newSATAsymmBranch(l, &params)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, a.Close()) })
	debug, _, _ := satIntegrityActionFixture(t, "")
	satOK(t, debug.EnableDebug("sat_propagate"))
	satOK(t, debug.EnableDebug("sat_missed_prop"))
	satOK(t, debug.EnableDebug("asymm_branch"))
	before := p.limit.Count()
	satOK(t, a.run(true))
	want := uint64(0)
	if satStorageSourceDebug {
		want = 3
	} // propagate checker, entry checker, exit checker.
	if p.limit.Count()-before != want {
		t.Fatal("actual three enabled source checker sites", p.limit.Count()-before, want)
	}
	if a.reportActive || a.savedPhase != nil {
		t.Fatal("completed source locals")
	}
}

func TestSATAsymmInvariantUsesSourceConditionalAssertionTag(t *testing.T) {
	g, _ := satGCFixture(t, 3)
	l := g.lifecycle()
	params, err := l.propagation.config.context.NewValue()
	satOK(t, err)
	a, err := newSATAsymmBranch(l, &params)
	satOK(t, l.propagation.config.context.Del(&params))
	satOK(t, err)
	t.Cleanup(func() { satOK(t, a.Close()) })
	debug, _, _ := satIntegrityActionFixture(t, "")
	before := l.propagation.limit.Steps
	satOK(t, a.run(true))
	want := uint64(0)
	if satStorageSourceDebug {
		want = 0 // The source tag is disabled by default.
	}
	if got := l.propagation.limit.Steps - before; got != want {
		t.Fatalf("disabled process assertion consumed %d checks, want %d", got, want)
	}
	satOK(t, debug.EnableDebug("asymm_branch"))
	before = l.propagation.limit.Steps
	satOK(t, a.run(true))
	want = 0
	if satStorageSourceDebug {
		want = 2 // CASSERT at asymmetric-branch entry and normal exit.
	}
	if got := l.propagation.limit.Steps - before; got != want {
		t.Fatalf("enabled process assertion consumed %d checks, want %d", got, want)
	}
}

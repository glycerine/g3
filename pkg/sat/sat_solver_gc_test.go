package sat

// Source: src/sat/sat_solver.cpp:1169–1190,1347–1670; src/sat/sat_solver.h:380–389; src/util/luby.cpp.

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func satGCFixture(t *testing.T, n uint32) (*satSolverGC, *satSolverConflictResolution) {
	t.Helper()
	s := satConflictFixture(t, n)
	l := s.dynamic.probing.lifecycle
	c, e := newSATCleaner(l, l.budget)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, c.Close()) })
	g, e := newSATSolverGC(s, c)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, g.Close()) })
	satOK(t, g.initSearch())
	return g, s
}

func TestSATSolverRestartAndGCCASSERTUseSourceTags(t *testing.T) {
	g, s := satGCFixture(t, 2)
	debug, _, _ := satIntegrityActionFixture(t, "")
	p := s.analysis.propagation
	before := p.limit.Steps
	satOK(t, g.restart())
	if got := p.limit.Steps - before; got != 0 {
		t.Fatal("disabled restart tag reached checker", got)
	}
	satOK(t, debug.EnableDebug("sat_restart"))
	before = p.limit.Steps
	satOK(t, g.restart())
	want := uint64(0)
	if satStorageSourceDebug {
		want = 1
	}
	if got := p.limit.Steps - before; got != want {
		t.Fatal("restart CASSERT source tag", got, want)
	}
	s.conflictsSinceGC = g.gcThreshold + 1
	before = p.limit.Steps
	satOK(t, g.gc())
	if got := p.limit.Steps - before; got != 0 {
		t.Fatal("disabled GC tag reached checker", got)
	}
	satOK(t, debug.EnableDebug("sat_gc_bug"))
	s.conflictsSinceGC = g.gcThreshold + 1
	before = p.limit.Steps
	satOK(t, g.gc())
	want = 0
	if satStorageSourceDebug {
		want = 2
	}
	if got := p.limit.Steps - before; got != want {
		t.Fatal("GC entry and exit CASSERT source tags", got, want)
	}
}

func satGCClause(t *testing.T, g *satSolverGC, x []Literal, glue uint32) StoredClauseRef {
	t.Helper()
	l := g.lifecycle()
	r, e := l.propagation.clauses.MkClause(x, true)
	satOK(t, e)
	satOK(t, l.pushClause(true, r))
	satOK(t, l.propagation.clauses.SetGlue(r, glue))
	satOK(t, l.attachClause(r))
	return r
}
func TestSATSolverGCSearchInitAndRestartPolicies(t *testing.T) {
	g, s := satGCFixture(t, 2)
	p := s.analysis.propagation
	cfg, e := p.config.State()
	satOK(t, e)
	if g.gcThreshold != cfg.GCInitial || g.restartThreshold != cfg.RestartInitial || g.lubyIndex != 1 || g.minDTk != 1 || s.phaseCounter != 0 || s.phaseCacheOn {
		t.Fatal("init_search order/state")
	}
	satOK(t, s.analysis.lemma.Push(1))
	satOK(t, s.core.Push(1))
	satOK(t, s.minCore.Push(3))
	s.minCoreValid = true
	s.phaseCounter = 55
	s.phaseCacheOn = true
	s.conflictsSinceRestart = 9
	p.variables.modelIsCurrent = true
	satOK(t, g.initSearch())
	a, e := s.core.Size()
	satOK(t, e)
	b, e := s.minCore.Size()
	satOK(t, e)
	if a != 0 || b != 0 || s.minCoreValid || s.phaseCacheOn || p.variables.modelIsCurrent || !g.watch.Running() {
		t.Fatal("source init resets")
	}
	p.config.state.Restart = satRestartLuby
	p.config.state.RestartInitial = 3
	want := []uint32{3, 6, 3, 3, 6, 12}
	for _, v := range want {
		satOK(t, g.restart())
		if g.restartThreshold != v {
			t.Fatal(g.lubyIndex, g.restartThreshold, v)
		}
	}
	p.config.state.Restart = satRestartGeometric
	p.config.state.RestartFactor = 1.5
	g.restartThreshold = 3
	s.conflictsSinceRestart = 7
	satOK(t, g.restart())
	if g.restartThreshold != 4 || s.conflictsSinceRestart != 0 || g.restarts != 7 || g.restartStat != 7 {
		t.Fatal("literal restart prefix")
	}
}
func TestSATSolverGCAllSortStrategiesStableAndProtected(t *testing.T) {
	for _, strategy := range []satGCStrategy{satGCGlue, satGCPSM, satGCGluePSM, satGCPSMGlue} {
		t.Run(fmt.Sprint(strategy), func(t *testing.T) {
			g, s := satGCFixture(t, 6)
			p := s.analysis.propagation
			p.config.state.GCStrategy = strategy
			refs := []StoredClauseRef{}
			for _, glue := range []uint32{4, 1, 3, 2} {
				refs = append(refs, satGCClause(t, g, []Literal{0, 2, 4}, glue))
			}
			// Phase unavailable yields equal PSM: exact stable original order matters.
			s.conflictsSinceGC = g.gcThreshold + 1
			satOK(t, g.gc())
			want := []StoredClauseRef{refs[1], refs[3]}
			if strategy == satGCPSM || strategy == satGCPSMGlue {
				if strategy == satGCPSM {
					want = []StoredClauseRef{refs[0], refs[1]}
				}
			}
			if !reflect.DeepEqual(g.lifecycle().learned, want) || g.gcClauseStat != 2 || s.conflictsSinceGC != 0 {
				t.Fatal("stable selected source strategy", g.lifecycle().learned, want)
			}
		})
	}
	g, _ := satGCFixture(t, 6)
	l := g.lifecycle()
	p := l.propagation
	r := satGCClause(t, g, []Literal{0, 2, 4, 6}, 5)
	satOK(t, l.push())
	head, e := p.clauses.Literal(r, 0)
	satOK(t, e)
	satOK(t, p.assign(head, newSATClauseJustification(func() satClauseOffset { o, e := p.clauses.Offset(r); satOK(t, e); return o }())))
	yes, e := g.canDelete(r)
	satOK(t, e)
	if yes {
		t.Fatal("actual clause justification protects deletion")
	}
	satOK(t, p.variables.justification.write(uint32(head.variable()), satJustification{}))
	yes, e = g.canDelete(r)
	satOK(t, e)
	if !yes {
		t.Fatal("decision permits delete")
	}
	satOK(t, p.clauses.SetReinitStack(r, true))
	yes, e = g.canDelete(r)
	satOK(t, e)
	if yes {
		t.Fatal("reinit protects")
	}
	tr := satGCClause(t, g, []Literal{2, 4, 6}, 6)
	yes, e = g.canDelete(tr)
	satOK(t, e)
	if !yes {
		t.Fatal("ternary never retained as reason")
	}
}
func TestSATSolverGCDynamicFreezeActivateAndInactivity(t *testing.T) {
	g, s := satGCFixture(t, 4)
	p := s.analysis.propagation
	l := g.lifecycle()
	cfg := &p.config.state
	cfg.GCStrategy = satGCDynPSM
	cfg.GCSmallLBD = 1
	cfg.GCK = 3
	cfg.Initialized |= satConfigGCSmallLBDMask | satConfigGCKMask
	r := satGCClause(t, g, []Literal{0, 2, 4}, 5)
	for i := uint32(0); i < 4; i++ {
		satOK(t, p.variables.setPhase(BoolVar(i), 0))
		satOK(t, p.variables.setPreviousPhase(BoolVar(i), 0))
		satOK(t, p.variables.setGCAssigned(BoolVar(i), true))
	}
	s.conflictsSinceGC = g.gcThreshold + 1
	satOK(t, g.gc())
	f, e := p.clauses.Frozen(r)
	satOK(t, e)
	if !f || g.numFrozen != 1 || g.minDTk != 0 {
		t.Fatal("actual d_tk and freeze")
	}
	// PSM becomes zero; source unfreezes and actually reattaches.
	for i := uint32(0); i < 4; i++ {
		satOK(t, p.variables.setPhase(BoolVar(i), 1))
		satOK(t, p.variables.setPreviousPhase(BoolVar(i), 1))
		satOK(t, p.variables.setGCAssigned(BoolVar(i), true))
	}
	s.conflictsSinceGC = g.gcThreshold + 1
	satOK(t, g.gc())
	f, e = p.clauses.Frozen(r)
	satOK(t, e)
	if f || g.numFrozen != 0 || len(l.learned) != 1 {
		t.Fatal("source reactivation")
	}
	// Active small-LBD bypasses inactivity and used clearing.
	satOK(t, p.clauses.SetGlue(r, 1))
	satOK(t, p.clauses.MarkUsed(r))
	s.conflictsSinceGC = g.gcThreshold + 1
	satOK(t, g.gc())
	used, e := p.clauses.Used(r)
	satOK(t, e)
	if !used {
		t.Fatal("small LBD skips actions")
	}
}
func TestSATSolverGCThresholdScopeAndOwners(t *testing.T) {
	g, s := satGCFixture(t, 1)
	p := s.analysis.propagation
	p.config.state.GCStrategy = satGCDynPSM
	satOK(t, g.lifecycle().push())
	s.conflictsSinceGC = g.gcThreshold + 1
	old := g.gcThreshold
	satOK(t, g.gc())
	if g.gcThreshold != old || s.conflictsSinceGC != old+1 {
		t.Fatal("dyn higher scope early return")
	}
	satOK(t, g.lifecycle().pop(1))
	s.conflictsSinceGC = old
	satOK(t, g.gc())
	if g.gcThreshold != old {
		t.Fatal("strict threshold")
	}
	copy := *g
	if e := copy.Close(); !errors.Is(e, ErrSATOwner) {
		t.Fatal(e)
	}
	satOK(t, g.Close())
	if e := g.gc(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
}

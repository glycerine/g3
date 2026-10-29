package sat

// Source: src/sat/sat_solver.cpp:2781-2801,3716-3751; src/sat/sat_solver.h:48-70.

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func satSolverStatisticsFixture(t *testing.T) (*statistics.Statistics, satSolverStatisticsKeys) {
	st := statFixture(t)
	var keys satSolverStatisticsKeys
	names := [15]string{"mk bool var", "mk binary clause", "mk ternary clause", "mk clause", "gc clause", "del clause", "conflicts", "propagations", "decisions", "binary propagations", "ternary propagations", "restarts", "minimized lits", "dyn subsumption resolution", "blocked correction sets"}
	for i, n := range names {
		keys.main[i] = statKey(n)
	}
	for i, n := range [2]string{"elim clauses", "elim literals"} {
		keys.cleaner[i] = statKey(n)
	}
	for i, n := range [5]string{"subsumed", "subsumption resolution", "elim literals", "elim bool vars", "elim blocked clauses"} {
		keys.simplifier[i] = statKey(n)
	}
	keys.scc[0] = statKey("sat scc elim vars")
	keys.scc[1] = statKey("sat scc elim binary")
	keys.asymm = statKey("elim literals")
	keys.probing = statKey("probing assigned")
	return st, keys
}
func satSolverSeedStatistics(s *Solver) {
	s.Variables.mkVarCount = 1
	s.Lifecycle.mkBinary = 2
	s.Lifecycle.mkTernary = 3
	s.Lifecycle.mkNary = 4
	s.gc.gcClauseStat = 5
	s.Lifecycle.deleted = 6
	s.conflict.conflictStat = 7
	s.propagation.clausePropagations = 8
	s.decision.decisions = 9
	s.propagation.binaryPropagations = 10
	s.propagation.ternaryPropagations = 11
	s.gc.restartStat = 12
	s.analysis.minimizedLits = 13
	s.dynamic.eliminated = 14
	s.blockedCorrectionSets = 15
	s.cleaner.elimClauses = 16
	s.cleaner.elimLiterals = 17
	s.simplifier.numSubsumed = 18
	s.simplifier.numSubRes = 19
	s.simplifier.numElimLits = 20
	s.simplifier.numElimVars = 21
	s.simplifier.numBlocked = 22
	s.scc.numElim = 23
	s.scc.numElimBinary = 24
	s.asymm.elimLiterals = 25
	s.probing.numAssigned = 26
}
func TestSATSolverStatisticsActualSourceOrderAndResetKeepsSCC(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	st, keys := satSolverStatisticsFixture(t)
	satSolverSeedStatistics(s)
	s.Lifecycle.nonLearnedGeneration = 26
	s.conflict.conflicts = 99
	s.conflict.conflictsSinceRestart = 77
	s.gc.restarts = 44
	s.propagation.cleanerCounter = -17
	satOK(t, s.collectStatistics(st, keys))
	if st.Size() != 26 {
		t.Fatal(st.Size())
	}
	for i := 0; i < 26; i++ {
		v := st.UintValue(i)
		if v != uint32(i+1) {
			t.Fatal(i, v)
		}
	}
	satOK(t, s.resetStatistics())
	if s.Lifecycle.nonLearnedGeneration != 0 || s.scc.numElim != 23 || s.scc.numElimBinary != 24 || s.conflict.conflicts != 99 || s.conflict.conflictsSinceRestart != 77 || s.gc.restarts != 44 || s.propagation.cleanerCounter != -17 {
		t.Fatal("source reset boundaries")
	}
	st.Reset()
	satOK(t, s.collectStatistics(st, keys))
	if st.Size() != 2 {
		t.Fatal("SCC is intentionally retained", st.Size())
	}
	if st.UintValue(0) != 23 || st.UintValue(1) != 24 {
		t.Fatal(st.UintValue(0), st.UintValue(1))
	}
	satFullSolverRuntimeAudit(t, s)
}
func TestSATSolverStatisticsZeroSkipAndSliceReuse(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	st, keys := satSolverStatisticsFixture(t)
	satOK(t, s.collectStatistics(st, keys))
	if st.Size() != 0 {
		t.Fatal("all real constructor counters initially zero", st.Size())
	}
	satSolverSeedStatistics(s)
	for pass := 0; pass < 6; pass++ {
		st.Reset()
		satOK(t, s.collectStatistics(st, keys))
		if st.Size() != 26 {
			t.Fatal("reused output", st.Size())
		}
	}
	satFullSolverRuntimeAudit(t, s)
}
func TestSATSolverStatisticsPreservesUint32AndDuplicateKeyContributions(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	st, k := satSolverStatisticsFixture(t)
	s.Variables.mkVarCount = ^uint32(0)
	s.cleaner.elimLiterals = 3
	s.simplifier.numElimLits = 5
	s.asymm.elimLiterals = 7
	// Source update appends contributions rather than coalescing key spellings.
	satOK(t, s.collectStatistics(st, k))
	want := []uint32{^uint32(0), 3, 5, 7}
	if st.Size() != len(want) {
		t.Fatal(st.Size())
	}
	for i, w := range want {
		v := st.UintValue(i)
		if v != w {
			t.Fatal(i, v, w)
		}
	}
	satOK(t, s.collectStatistics(st, k))
	if st.Size() != 8 {
		t.Fatal(st.Size())
	}
	for i, w := range want {
		v := st.UintValue(i + 4)
		if v != w {
			t.Fatal(i, v, w)
		}
	}
}
func TestSATSolverStatisticsCopiedClosedAndHeaderCharge(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	st, k := satSolverStatisticsFixture(t)
	if s.ownerBytes != uint64(reflect.TypeOf(Solver{}).Size()) || s.blockedCorrectionSets != 0 {
		t.Fatal("actual source field/header")
	}
	copied := *s
	if e := copied.collectStatistics(st, k); e != ErrSATOwner {
		t.Fatal(e)
	}
	if e := copied.resetStatistics(); e != ErrSATOwner {
		t.Fatal(e)
	}
	satOK(t, s.Close())
	if e := s.collectStatistics(st, k); e != arena.ErrClosed {
		t.Fatal(e)
	}
	if e := s.resetStatistics(); e != arena.ErrClosed {
		t.Fatal(e)
	}
}
func TestSATSolverStatisticsIndependentSparseCounterMasks(t *testing.T) {
	for mask := uint32(0); mask < 64; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			s, _, _ := satSolverCheckFixture(t)
			st, k := satSolverStatisticsFixture(t)
			satSolverSeedStatistics(s)
			positions := [6]int{0, 15, 17, 22, 23, 25}
			var expected [26]uint32
			for i := range expected {
				expected[i] = uint32(i + 1)
			}
			for j, pos := range positions {
				if mask&(1<<uint32(j)) == 0 {
					expected[pos] = 0
				}
			}
			s.Variables.mkVarCount = expected[0]
			s.cleaner.elimClauses = expected[15]
			s.simplifier.numSubsumed = expected[17]
			s.scc.numElim = expected[22]
			s.scc.numElimBinary = expected[23]
			s.probing.numAssigned = expected[25]
			satOK(t, s.collectStatistics(st, k))
			index := 0
			for _, want := range expected {
				if want == 0 {
					continue
				}
				v := st.UintValue(index)
				if v != want {
					t.Fatal(index, v, want, mask)
				}
				index++
			}
			if st.Size() != index {
				t.Fatal(st.Size(), index)
			}
			satFullSolverRuntimeAudit(t, s)
		})
	}
}

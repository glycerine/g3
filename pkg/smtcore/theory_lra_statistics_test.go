package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func lraCollectedStatistics(t *testing.T, a *SMTLRATheory) map[statistics.Key]uint32 {
	t.Helper()
	st := statistics.New()
	astTestMust(t, a.CollectStatistics(st))
	out := make(map[statistics.Key]uint32)
	for i := 0; i < st.Size(); i++ {
		if !st.IsUint(i) {
			t.Fatal("native unsigned statistic became floating point", st.Key(i))
		}
		if _, exists := out[st.Key(i)]; exists {
			t.Fatal("duplicate LRA contribution", st.Key(i))
		}
		out[st.Key(i)] = st.UintValue(i)
	}
	return out
}

func TestSMTLRAStatisticsActualRowsScopesAndSearchLifetime(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	if f.a.Name() != "lra" || len(lraCollectedStatistics(t, f.a)) != 0 {
		t.Fatal("fresh LRA statistics")
	}
	x, y := f.variable("lra-stats-x"), f.variable("lra-stats-y")
	for _, h := range []ast.ASTHandle{
		f.app(smtArithOpGE, x, f.number(0)), f.app(smtArithOpLE, x, f.number(4)),
		f.app(smtArithOpLE, y, f.number(2)), f.app(smtArithOpGE, f.app(smtArithOpAdd, x, y), f.number(5)),
	} {
		f.assert(h, true)
	}
	lraFinalCheckAssignments(t, f)
	status, err := f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone {
		t.Fatal("initial feasibility", status)
	}
	first := lraCollectedStatistics(t, f.a)
	for key, want := range map[statistics.Key]uint32{"arith-lower": 2, "arith-upper": 2, "arith-rows": 4, "arith-make-feasible": 1, "arith-max-columns": 3, "arith-max-rows": 1} {
		if first[key] != want {
			t.Fatal(key, first[key], want)
		}
	}
	astTestMust(t, f.s.PushScope())
	f.assert(f.app(smtArithOpLE, x, f.number(2)), true)
	lraFinalCheckAssignments(t, f)
	status, err = f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || !f.s.inconsistent {
		t.Fatal("missing numerical conflict", status)
	}
	conflict := lraCollectedStatistics(t, f.a)
	astTestMust(t, f.s.PopScopes(1))
	astTestMust(t, f.a.InitializeSearch())
	if f.a.conflictCount != 0 {
		t.Fatal("search counter was not reset")
	}
	after := lraCollectedStatistics(t, f.a)
	for key, want := range map[statistics.Key]uint32{"arith-lower": 3, "arith-upper": 2, "arith-rows": 5, "arith-conflicts": 1, "arith-make-feasible": 2, "arith-max-columns": 3, "arith-max-rows": 1} {
		if after[key] != want || conflict[key] != want {
			t.Fatal("cumulative statistic changed at pop/search", key, after[key], conflict[key], want)
		}
	}
	if _, exists := after["arith-plateau-iterations"]; exists {
		t.Fatal("unwritten native counter fabricated")
	}
	// LP counters are sampled by make_feasible, not observed afresh at collect.
	f.a.numeric.solver.Settings.Stats.TotalIterations += 17
	f.a.numeric.solver.Settings.Stats.Factorizations += 19
	f.a.numeric.solver.Settings.Stats.NeedToSolveInf += 23
	again := lraCollectedStatistics(t, f.a)
	for _, key := range []statistics.Key{"arith-iterations", "arith-factorizations", "arith-pivots"} {
		if again[key] != after[key] {
			t.Fatal("collector sampled LP state outside make_feasible", key, again[key], after[key])
		}
	}
}

func TestSMTLRAStatisticsBasicVersusLPAndEqualityCallbacks(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		f := newLRAFrontendFixture(t, true)
		x := f.variable("lra-stats-basic-x")
		lower, strong := f.app(smtArithOpGE, x, f.number(1)), f.app(smtArithOpGE, x, f.number(2))
		astTestMust(t, f.s.Internalize(lower, true))
		astTestMust(t, f.s.Internalize(strong, true))
		bv, _ := f.s.BoolVar(strong)
		astTestMust(t, f.a.propagateBasicBound(f.a.boundByBoolean[bv], true))
		st := lraCollectedStatistics(t, f.a)
		if st["arith-propagations"] != 1 || st["arith-bound-propagations-cheap"] != 1 || st["arith-bound-propagations-lp"] != 0 {
			t.Fatal("basic propagation counters", st)
		}
	})
	t.Run("lp", func(t *testing.T) {
		f := newLRAFrontendFixture(t, true)
		target := prepareLRALPPropagation(t, f)
		astTestMust(t, f.a.propagateLPBounds())
		lit, err := f.s.Literal(target)
		astTestMust(t, err)
		value, err := f.s.Assignment(lit)
		astTestMust(t, err)
		st := lraCollectedStatistics(t, f.a)
		if value != lbool.True || st["arith-bound-propagations-lp"] == 0 || st["arith-bound-propagations-cheap"] != 0 || st["arith-propagations"] != 0 {
			t.Fatal("LP propagation counters conflated", st, value)
		}
	})
	t.Run("equality", func(t *testing.T) {
		f := newLRAFrontendFixture(t, true)
		x, y := f.variable("lra-stats-eq-x"), f.variable("lra-stats-eq-y")
		lraFix(t, f, x, 2)
		lraFix(t, f, y, 2)
		f.propagate(true)
		if lraCollectedStatistics(t, f.a)["arith-fixed-eqs"] != 1 {
			t.Fatal("fixed equality omitted")
		}
		xv, yv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID()), f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID())
		astTestMust(t, f.a.NewDisequality(xv, yv))
		st := lraCollectedStatistics(t, f.a)
		if st["arith-diseq"] != 1 || st["arith eq adapter"] == 0 {
			t.Fatal("equality adapter/disequality stats", st)
		}
	})
}

func TestSMTLRAStatisticsUnsignedWidthAndGuards(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	f.a.numeric.assertLower = uint64(1) << 32
	f.a.numeric.assertUpper = (uint64(1) << 32) + 3
	f.a.basicPropagations = (uint64(1) << 32) + 5
	f.a.lpPropagations = (uint64(1) << 32) + 7
	f.a.fixedEqualities = (uint64(1) << 32) + 11
	st := lraCollectedStatistics(t, f.a)
	for key, want := range map[statistics.Key]uint32{"arith-lower": 0, "arith-upper": 3, "arith-bound-propagations-cheap": 5, "arith-bound-propagations-lp": 7, "arith-fixed-eqs": 11} {
		if st[key] != want {
			t.Fatal("source unsigned width", key, st[key], want)
		}
	}
	if err := f.a.CollectStatistics(nil); err != ast.ErrASTHandle {
		t.Fatal("nil statistics target", err)
	}
	var absent *SMTLRATheory
	if err := absent.CollectStatistics(statistics.New()); err != ast.ErrASTHandle {
		t.Fatal("nil theory", err)
	}
}

func TestSMTLRAStatisticsSolverCollectionAndResetNoOp(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, cfg.Params, "QF_UFLRA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	a, err := NewSMTLRATheory(family)
	astTestMust(t, err)
	astTestMust(t, a.ConfigureArithmetic(s.kernel.arithState))
	astTestMust(t, s.kernel.replaceUnusedSetupArithmetic(a))
	s.kernel.preprocessingSetupDone = true
	s.kernel.copiedArithmeticPlugin = true
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	f := &lraFrontendFixture{t, m, s.kernel.internalizer, a, sort}
	x := f.variable("lra-solver-stats-x")
	astTestMust(t, s.Assert(f.app(smtArithOpGE, x, f.number(1))))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("direct-installed LRA solver", status)
	}
	collected := statistics.New()
	astTestMust(t, s.CollectStatistics(collected))
	found := false
	for i := 0; i < collected.Size(); i++ {
		if collected.Key(i) == "arith-make-feasible" && collected.IsUint(i) && collected.UintValue(i) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("solver omitted concrete LRA statistics")
	}
	before := lraCollectedStatistics(t, a)
	astTestMust(t, s.kernel.ResetStatistics())
	after := lraCollectedStatistics(t, a)
	for key, value := range before {
		if after[key] != value {
			t.Fatal("source kernel reset must be no-op", key)
		}
	}
	v := s.kernel.internalizer.context.enodes.get(x).TheoryVar(family)
	value, err := a.numeric.value(v)
	astTestMust(t, err)
	if value.Main.Cmp(big.NewRat(1, 1)) < 0 {
		t.Fatal("statistics collection changed numerical model", value)
	}
}

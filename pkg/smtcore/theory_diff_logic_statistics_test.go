package smtcore

// Source: src/smt/theory_diff_logic_def.h:298-319 and
// src/smt/diff_logic.h:123-144.

import (
	"math"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func smtDiffCollectedStatistics(t *testing.T, theory interface {
	CollectStatistics(*statistics.Statistics) error
}) map[statistics.Key]uint32 {
	t.Helper()
	out := statistics.New()
	astTestMust(t, theory.CollectStatistics(out))
	result := make(map[statistics.Key]uint32, out.Size())
	for i := 0; i < out.Size(); i++ {
		if !out.IsUint(i) {
			t.Fatal("difference statistic is not unsigned", out.Key(i))
		}
		result[out.Key(i)] = out.UintValue(i)
	}
	return result
}

func TestSMTDifferenceStatisticsActualAssertionsConflictsAndGuards(t *testing.T) {
	t.Run("integer", func(t *testing.T) {
		f := newSMTIntegerDiffFixture(t)
		bounds := []ast.ASTHandle{
			f.bound(t, smtArithOpLE, f.x, f.y, 0),
			f.bound(t, smtArithOpLE, f.y, f.x, -1),
		}
		for _, bound := range bounds {
			astTestMust(t, f.state.Internalize(bound, true))
			literal, err := f.state.Literal(bound)
			astTestMust(t, err)
			astTestMust(t, f.theory.Assign(literal.Var(), true))
		}
		astTestMust(t, f.theory.Propagate())
		got := smtDiffCollectedStatistics(t, f.theory)
		if got["dl asserts"] != 2 || got["dl conflicts"] != 1 || got["dl prop steps"] == 0 {
			t.Fatal("integer difference statistics", got)
		}
		if err := f.theory.CollectStatistics(nil); err != ast.ErrASTHandle {
			t.Fatal("nil target", err)
		}
		f.theory.statistics.assertions = math.MaxUint32
		literal, err := f.state.Literal(bounds[0])
		astTestMust(t, err)
		astTestMust(t, f.theory.Assign(literal.Var(), true))
		if f.theory.statistics.assertions != 0 {
			t.Fatal("source uint32 assertion counter did not wrap", f.theory.statistics.assertions)
		}
	})

	t.Run("real", func(t *testing.T) {
		f := newSMTRealDiffFixture(t)
		bound := f.bound(t, smtArithOpLE, f.x, f.y, 1, 2)
		astTestMust(t, f.state.Internalize(bound, true))
		literal, err := f.state.Literal(bound)
		astTestMust(t, err)
		astTestMust(t, f.theory.Assign(literal.Var(), true))
		astTestMust(t, f.theory.Propagate())
		got := smtDiffCollectedStatistics(t, f.theory)
		if got["dl asserts"] != 1 || got["dl conflicts"] != 0 {
			t.Fatal("real difference statistics", got)
		}
		if err = (&SMTRealDiffTheory{}).CollectStatistics(statistics.New()); err != ast.ErrASTHandle {
			t.Fatal("uninitialized theory", err)
		}
	})

	t.Run("solver", func(t *testing.T) {
		m, cfg := smtModelGeneratorFixture(t)
		params, err := cfg.Context.NewValue()
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
		name := "auto_config"
		astTestMust(t, cfg.Context.SetBoolName(&params, &name, false))
		name = "arith.solver"
		astTestMust(t, cfg.Context.SetUIntName(&params, &name, 1))
		solver, err := NewSMTSolver(m, &params, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, solver.Close()) })
		family, err := m.RegisterArithmeticPlugin()
		astTestMust(t, err)
		sort, err := m.MkFamilySort(family, 1, nil)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "difference-statistics-solver-x"), sort)
		astTestMust(t, err)
		one, err := bigTestArithNumeral(m, big.NewRat(1, 1), true)
		astTestMust(t, err)
		bound, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{x, one})
		astTestMust(t, err)
		astTestMust(t, solver.Assert(bound))
		status, err := smtSetupProfileCheck(solver)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("difference statistics solver check", status)
		}
		out := statistics.New()
		astTestMust(t, solver.CollectStatistics(out))
		found := false
		for i := 0; i < out.Size(); i++ {
			found = found || out.Key(i) == "dl asserts" && out.IsUint(i) && out.UintValue(i) == 1
		}
		if !found {
			t.Fatal("solver omitted difference statistics")
		}
	})
}

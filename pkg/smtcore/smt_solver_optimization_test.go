package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func optimizationSMTFixture(t *testing.T, optimize bool) (*ast.ASTManager, *SMTSolver) {
	t.Helper()
	m, config := smtModelGeneratorFixture(t)
	var s *SMTSolver
	var err error
	cfg := SMTSolverConfig{Params: config.Context, ModelEvaluator: config}
	if optimize {
		s, err = NewOptimizationSMTSolver(m, nil, "AUFLIA", cfg)
	} else {
		s, err = NewSMTSolver(m, nil, "AUFLIA", cfg)
	}
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.SetProduceModels(true))
	return m, s
}

func TestOptimizationSolverQuantifierUnknownAdjustmentAndStickyFlag(t *testing.T) {
	for _, optimize := range []bool{false, true} {
		m, s := optimizationSMTFixture(t, optimize)
		local, err := s.config.Params.NewValue()
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, s.config.Params.Del(&local)) })
		for _, name := range []string{"mbqi", "preprocess", "auto_config"} {
			astTestMust(t, s.config.Params.SetBoolName(&local, &name, false))
		}
		name := "relevancy"
		astTestMust(t, s.config.Params.SetUIntName(&local, &name, 0))
		astTestMust(t, s.UpdateParams(&local))
		sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "opt-validation-sort"), nil)
		astTestMust(t, err)
		boolean, err := m.MkBoolSort()
		astTestMust(t, err)
		predicate := smtQuickPredicate(t, m, "opt-validation-p", sort, boolean)
		variable, err := m.MkVar(0, sort)
		astTestMust(t, err)
		body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
		astTestMust(t, err)
		pattern, err := m.MkPattern([]ast.ASTHandle{body})
		astTestMust(t, err)
		quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body, Patterns: []ast.ASTHandle{pattern}})
		astTestMust(t, err)
		astTestMust(t, s.Push())
		astTestMust(t, s.Assert(quantifier))
		status, err := s.CheckSat(nil)
		astTestMust(t, err)
		want := lbool.Undef
		if optimize {
			want = lbool.True
		}
		unknown, err := s.OptimizationWasUnknown()
		astTestMust(t, err)
		if status != want || unknown != optimize || s.kernel.lastFailure != SMTFailureQuantifiers {
			t.Fatal("quantifier adjustment", optimize, status, unknown, s.kernel.lastFailure)
		}
		reason, err := s.ReasonUnknown()
		astTestMust(t, err)
		if reason != "(incomplete quantifiers)" {
			t.Fatal(reason)
		}
		md, err := s.AcquireModel()
		astTestMust(t, err)
		if optimize && md == nil {
			t.Fatal("adjusted satisfiable check omitted candidate model")
		}
		if md != nil {
			astTestMust(t, md.DecRef())
		}
		astTestMust(t, s.Pop(1))
		status, err = s.CheckSat(nil)
		astTestMust(t, err)
		unknown, err = s.OptimizationWasUnknown()
		astTestMust(t, err)
		if status != lbool.True || unknown != optimize {
			t.Fatal("sticky flag after later complete check", status, unknown)
		}
		astTestMust(t, s.Push())
		falsity, err := m.MkFalse()
		astTestMust(t, err)
		astTestMust(t, s.Assert(falsity))
		status, err = s.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("unsat was adjusted", status)
		}
	}
}

func TestOptimizationSolverModelFallbackAndCacheReplacement(t *testing.T) {
	m, s := optimizationSMTFixture(t, true)
	md, err := s.AcquireModel()
	astTestMust(t, err)
	if md != nil {
		t.Fatal("fresh cache not empty")
	}
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "cached-opt-p"), boolean)
	astTestMust(t, err)
	astTestPin(t, m, p)
	astTestMust(t, s.Assert(p))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	first, err := s.AcquireModel()
	astTestMust(t, err)
	if first == nil {
		t.Fatal("missing first model")
	}
	t.Cleanup(func() { astTestMust(t, first.DecRef()) })
	astTestMust(t, s.Push())
	negative, err := m.MkNot(p)
	astTestMust(t, err)
	astTestMust(t, s.Assert(negative))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	fallback, err := s.AcquireModel()
	astTestMust(t, err)
	if fallback != first {
		t.Fatal("unsat lost cached model")
	}
	astTestMust(t, fallback.DecRef())
	astTestMust(t, s.Pop(1))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	next, err := s.AcquireModel()
	astTestMust(t, err)
	if next == nil || next == first {
		t.Fatal("cache did not refresh")
	}
	t.Cleanup(func() { astTestMust(t, next.DecRef()) })
	astTestMust(t, s.Close())
	// Both callers' references survive cache replacement and solver teardown.
	for _, model := range []*model.ModelCore{first, next} {
		d, err := m.GetDecl(p)
		astTestMust(t, err)
		v, err := model.ConstantInterpretation(d)
		astTestMust(t, err)
		if !m.IsTrue(v) {
			t.Fatal("caller model lost interpretation")
		}
	}
}

func TestOptimizationSolverCancellationDoesNotBecomeSatisfiable(t *testing.T) {
	m, s := optimizationSMTFixture(t, true)
	m.ResourceLimit().Cancel()
	defer m.ResourceLimit().ResetCancel()
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	unknown, err := s.OptimizationWasUnknown()
	astTestMust(t, err)
	if status != lbool.Undef || unknown || s.optimization.first {
		t.Fatal("cancellation adjustment", status, unknown, s.optimization.first)
	}
	md, err := s.AcquireModel()
	astTestMust(t, err)
	if md != nil {
		t.Fatal("cancellation constructed a candidate")
	}
}

func TestOptimizationSolverFirstCheckHonorsAssumptionsAndScope(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		m, s := optimizationSMTFixture(t, true)
		boolean, err := m.MkBoolSort()
		astTestMust(t, err)
		p, err := m.MkConst(astTestSymbol(t, m, "initial-opt-assumption"), boolean)
		astTestMust(t, err)
		astTestPin(t, m, p)
		negative, err := m.MkNot(p)
		astTestMust(t, err)
		astTestMust(t, s.Assert(negative))
		assumptions := []ast.ASTHandle{p}
		if scoped {
			astTestMust(t, s.Push())
			astTestMust(t, s.Assert(p))
			assumptions = nil
		}
		status, err := s.CheckSat(assumptions)
		astTestMust(t, err)
		if status != lbool.False || s.optimization.first {
			t.Fatal("first check ignored source selection gate", scoped, status)
		}
		if scoped {
			astTestMust(t, s.Pop(1))
		}
		status, err = s.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("later check lost original assertions", scoped, status)
		}
	}
}

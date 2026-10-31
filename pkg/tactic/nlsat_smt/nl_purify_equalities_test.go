package nlsatsmt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestNLPurifyEnforcesModelEqualitiesInBothSolvers(t *testing.T) {
	m := astTestManager(t)
	md, ev, config := nlModelEvaluatorFixture(t, m)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x := analysisConst(t, m, real, "eq-x")
	y := analysisConst(t, m, real, "eq-y")
	z := analysisConst(t, m, real, "eq-z")
	h, err := m.ArithmeticRealNumeralText("1")
	one := analysisKeep(t, m, h, err)
	h, err = m.ArithmeticRealNumeralText("2")
	two := analysisKeep(t, m, h, err)
	for i, variable := range []ast.ASTHandle{x, y, z} {
		decl, err := m.GetDecl(variable)
		astTestMust(t, err)
		value := one
		if i == 2 {
			value = two
		}
		astTestMust(t, md.Core().RegisterConstant(decl, value))
	}
	smt, err := smtcore.NewSMTSolver(m, nil, "QF_UF", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config})
	astTestMust(t, err)
	defer func() { astTestMust(t, smt.Close()) }()
	nonlinear, err := tactic.NewGoal(m, false, true, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, nonlinear.Close()) }()
	filter, err := tactic.NewFilterModel(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, filter.Close()) }()
	state, err := newNLEqualityState(m, filter)
	astTestMust(t, err)
	defer func() { astTestMust(t, state.Close()) }()
	changed, err := state.enforce(ev, []ast.ASTHandle{x, y, z}, nonlinear, smt)
	astTestMust(t, err)
	if !changed || len(state.predicates) != 1 || state.values[0] != lbool.True {
		t.Fatal("new equality", changed, state.predicates, state.values)
	}
	if n, err := nonlinear.Size(); err != nil || n != 2 {
		t.Fatal("nonlinear equality clauses", n, err)
	}
	if n, err := smt.NumAssertions(); err != nil || n != 1 {
		t.Fatal("SMT equality assertion", n, err)
	}
	changed, err = state.enforce(ev, []ast.ASTHandle{x, y, z}, nonlinear, smt)
	astTestMust(t, err)
	if changed || len(state.predicates) != 1 {
		t.Fatal("repeated equality", changed, len(state.predicates))
	}
	eq, err := m.MkEq(x, y)
	astTestMust(t, err)
	astTestMust(t, smt.AssertFormula(eq))
	neg, err := m.MkNot(state.predicates[0])
	astTestMust(t, err)
	status, err := smt.CheckSat([]ast.ASTHandle{neg})
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("predicate did not enforce equality", status)
	}
	filteredModel, err := model.NewModel(m)
	astTestMust(t, err)
	var ref tactic.ModelRef
	astTestMust(t, ref.Set(filteredModel.Core()))
	defer func() { astTestMust(t, ref.Close()) }()
	decl, err := m.GetDecl(state.predicates[0])
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	astTestMust(t, filteredModel.Core().RegisterConstant(decl, truth))
	astTestMust(t, filter.ApplyUnindexed(&ref))
	filtered, err := ref.Get()
	astTestMust(t, err)
	if value, err := filtered.ConstantInterpretation(decl); err != nil || !value.IsNull() {
		t.Fatal("equality predicate remained in model", value, err)
	}
}

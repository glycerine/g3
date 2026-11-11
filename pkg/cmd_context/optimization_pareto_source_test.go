package cmd_context

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"testing"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestParetoModelEvaluationLeavesUninterpretedObjectiveIncomplete(t *testing.T) {
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "pareto-uninterpreted-objective")
	md, err := model.NewModelCore(c.manager)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, md.Close()) })
	contextMust(t, c.AddObjective(x, true))
	values, err := c.evaluateParetoModel(md)
	contextMust(t, err)
	if len(values) != 1 || values[0].comparable {
		t.Fatal("uninterpreted objective was completed", values)
	}
}

func TestParetoIncomparableObjectiveUsesNeutralComparison(t *testing.T) {
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "pareto-neutral-objective")
	contextMust(t, c.AddObjective(x, true))
	for _, dominates := range []bool{true, false} {
		backend, err := c.newParetoBackend(nil)
		contextMust(t, err)
		md, err := model.NewModelCore(c.manager)
		contextMust(t, err)
		contextMust(t, c.assertParetoFormula(backend, md, dominates))
		contextMust(t, md.Close())
		status, err := backend.CheckSat(nil)
		contextMust(t, err)
		if status != lbool.False {
			t.Fatal("neutral Pareto comparison status", dominates, status)
		}
		contextMust(t, backend.Close())
	}
}

func TestParetoDominanceFormulaKeepsSourceArgumentOrder(t *testing.T) {
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "pareto-flat-x")
	y := optimizationComparisonInteger(t, c, "pareto-flat-y")
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.AddObjective(y, false))

	md, err := model.NewModelCore(c.manager)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, md.Close()) })
	for i, term := range []ast.ASTHandle{x, y} {
		value, err := c.manager.ArithmeticIntegerNumeralText(fmt.Sprint(i + 2))
		contextMust(t, err)
		d, err := c.manager.GetDecl(term)
		contextMust(t, err)
		contextMust(t, md.RegisterConstant(d, value))
	}
	ref, err := c.makeParetoFormula(md, true)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, c.manager.DelRef(&ref)) })
	formula, err := c.manager.RefGet(ref)
	contextMust(t, err)
	count, err := c.manager.GetNumArgs(formula)
	contextMust(t, err)
	if count != 3 {
		t.Fatalf("source dominance conjunction arity = %d, want GE(x), GE(y), OR(GT(x),GT(y))", count)
	}
	for i, term := range []ast.ASTHandle{x, y} {
		ge, err := c.manager.GetArg(formula, uint32(i))
		contextMust(t, err)
		kind, err := c.manager.GetDeclKind(ge)
		contextMust(t, err)
		if kind != 3 && kind != 2 {
			t.Fatal("source comparison is nested under an extra gate")
		}
		left, err := c.manager.GetArg(ge, 0)
		contextMust(t, err)
		right, err := c.manager.GetArg(ge, 1)
		contextMust(t, err)
		if (i == 0 && left != term) || (i == 1 && right != term) {
			t.Fatal("source objective order or preference direction changed")
		}
	}
	strict, err := c.manager.GetArg(formula, 2)
	contextMust(t, err)
	kind, err := c.manager.GetDeclKind(strict)
	contextMust(t, err)
	if kind != ast.ASTOpOr {
		t.Fatal("source final dominance argument is not strict-improvement disjunction")
	}
	for index, term := range []ast.ASTHandle{x, y} {
		negation, err := c.manager.GetArg(strict, uint32(index))
		contextMust(t, err)
		op, err := c.manager.GetDeclKind(negation)
		contextMust(t, err)
		if op != ast.ASTOpNot {
			t.Fatal("strict source comparison is not negated LE")
		}
		le, err := c.manager.GetArg(negation, 0)
		contextMust(t, err)
		op, err = c.manager.GetDeclKind(le)
		contextMust(t, err)
		if op != 3 {
			t.Fatal("source LE preference must use arithmetic GE with swapped operands")
		}
		left, err := c.manager.GetArg(le, 0)
		contextMust(t, err)
		right, err := c.manager.GetArg(le, 1)
		contextMust(t, err)
		if (index == 0 && right != term) || (index == 1 && left != term) {
			t.Fatal("strict source objective order or preference direction changed")
		}
	}

}

func TestParetoCanceledImprovementDropsBackendAndAllowsFreshSearch(t *testing.T) {
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "pareto-canceled-x")
	m := c.manager
	zero, err := m.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	lower, err := m.MkArithmeticGE(x, zero)
	contextMust(t, err)
	contextMust(t, c.Assert(lower, ""))
	contextMust(t, c.AddObjective(x, true))
	func() {
		// An unbounded integer objective keeps producing improvements until
		// the source work limit interrupts its nested solver scope.
		// (optimize() checks twice before gia_pareto's loop.)
		contextMust(t, m.ResourceLimit().Push(3000))
		defer m.ResourceLimit().Pop()
		result, err := c.checkOptimizePareto(nil, false)
		contextMust(t, err)
		defer func() { contextMust(t, result.Close()) }()
		if result.Status != lbool.Undef || result.Model == nil {
			t.Fatal("expected cancellation after entering improvement loop", result.Status, result.Model)
		}
		if c.optimizationParetoBackend != nil || c.optimizationParetoActive {
			t.Fatal("canceled Pareto solver stayed active")
		}
	}()
	two, err := m.ArithmeticIntegerNumeralText("2")
	contextMust(t, err)
	upper, err := m.MkArithmeticLE(x, two)
	contextMust(t, err)
	contextMust(t, c.Assert(upper, ""))
	fresh, err := c.checkOptimizePareto(nil, false)
	contextMust(t, err)
	defer func() { contextMust(t, fresh.Close()) }()
	if fresh.Status != lbool.True || fresh.Model == nil {
		t.Fatal("fresh bounded Pareto search failed", fresh.Status)
	}
	values, err := c.evaluateParetoModel(fresh.Model)
	contextMust(t, err)
	if len(values) != 1 || values[0].measure.RatString() != "2" {
		t.Fatal("canceled dominance constraints leaked into fresh search", values)
	}
}

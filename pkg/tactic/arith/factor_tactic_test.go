package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestFactorTacticSplitsPolynomialEqualityWithProofAndCore(t *testing.T) {
	base, m := fixDLFixture(t, true)
	runner, err := NewFactorTactic(m, base.context, base.globals, nil, base.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, runner.Close()) })
	x := fixDLConst(t, m, false, "factor-x")
	one := fixDLNumber(t, m, false, "1")
	zero := fixDLNumber(t, m, false, "0")
	left := fixDLApp(t, m, 7, x, one)
	right := fixDLApp(t, m, 6, x, one)
	product := fixDLApp(t, m, 9, left, right)
	formula, err := m.MkEq(product, zero)
	equationMust(t, err)
	fixDLPin(t, m, formula)
	proof, err := m.MkProofAsserted(formula)
	equationMust(t, err)
	dependency, err := m.DependencyLeaf(formula)
	equationMust(t, err)
	goal, err := tactic.NewGoal(m, true, false, true)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.AssertWithProof(formula, proof, dependency))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	rewritten, err := goal.Form(0)
	equationMust(t, err)
	if family, err := m.GetFamilyID(rewritten); err != nil || family != m.BasicFamilyID() {
		t.Fatal("factor split family", family, err)
	}
	if op, err := m.GetDeclKind(rewritten); err != nil || op != ast.ASTOpOr {
		t.Fatal("factor split did not produce a disjunction", op, err)
	}
	if count, err := m.GetNumArgs(rewritten); err != nil || count != 2 {
		t.Fatal("factor split arity", count, err)
	}
	rewrittenProof, err := goal.Proof(0)
	equationMust(t, err)
	fact, err := m.GetProofFact(rewrittenProof)
	equationMust(t, err)
	if fact != rewritten {
		t.Fatal("factor rewrite proof conclusion")
	}
	gotDependency, err := goal.Dependency(0)
	equationMust(t, err)
	leaves, err := m.DependencyLinearize(gotDependency, nil)
	equationMust(t, err)
	if len(leaves) != 1 || leaves[0] != formula {
		t.Fatal("factor rewrite changed dependency", leaves)
	}
}

func TestFactorTacticStrictEvenPowerAndParameterUpdate(t *testing.T) {
	base, m := fixDLFixture(t, false)
	runner, err := NewFactorTactic(m, base.context, base.globals, nil, base.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, runner.Close()) })
	x := fixDLConst(t, m, false, "factor-even-x")
	zero := fixDLNumber(t, m, false, "0")
	square := fixDLApp(t, m, 9, x, x)
	strict := fixDLApp(t, m, factorLT, square, zero)
	goal, err := tactic.NewGoal(m, false, false, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.Assert(strict))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	if !goal.IsDecidedUnsat() {
		t.Fatal("square < 0 was not reduced to false")
	}
	depth, err := goal.Depth()
	equationMust(t, err)
	if depth != 1 {
		t.Fatal("factor tactic depth", depth)
	}
	params, err := base.context.NewValue()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, base.context.Del(&params)) })
	name := "split_factors"
	equationMust(t, base.context.SetBoolName(&params, &name, false))
	equationMust(t, runner.UpdateParams(&params))
	if runner.splitFactors {
		t.Fatal("split_factors parameter was ignored")
	}
}

// AUDIT.md tactic_arith_02-F1: C++ reduce_app factors an equality only when
// one side is an arithmetic application (is_arith_expr), so (= c b) never
// reaches expr2polynomial and does not fix the variable order.
func TestFactorTacticSkipsNonArithEqualityVariableOrder(t *testing.T) {
	base, m := fixDLFixture(t, false)
	runner, err := NewFactorTactic(m, base.context, base.globals, nil, base.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, runner.Close()) })
	a := fixDLConst(t, m, true, "factor-order-a")
	b := fixDLConst(t, m, true, "factor-order-b")
	c := fixDLConst(t, m, true, "factor-order-c")
	zero := fixDLNumber(t, m, true, "0")
	one := fixDLNumber(t, m, true, "1")
	two := fixDLNumber(t, m, true, "2")
	three := fixDLNumber(t, m, true, "3")
	product := fixDLApp(t, m, 9, fixDLApp(t, m, 7, a, one), fixDLApp(t, m, 7, b, two), fixDLApp(t, m, 7, c, three))
	positive := fixDLApp(t, m, factorGT, product, zero)
	eq, err := m.MkEq(c, b)
	equationMust(t, err)
	fixDLPin(t, m, eq)
	goal, err := tactic.NewGoal(m, false, false, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.Assert(eq))
	equationMust(t, goal.Assert(positive))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	if f, err := goal.Form(0); err != nil || f != eq {
		t.Fatal("equality changed", err)
	}
	rewritten, err := goal.Form(1)
	equationMust(t, err)
	prod, err := m.GetArg(rewritten, 0)
	equationMust(t, err)
	first, err := m.GetArg(prod, 0)
	equationMust(t, err)
	head, err := m.GetArg(first, 0)
	equationMust(t, err)
	if head != c {
		t.Fatal("first factor is not (+ c (- 3)): expr2polynomial saw (= c b)")
	}
}

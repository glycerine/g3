package qe

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierTacticQuantifierFreeGoalAndRefusal(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	port, err := NewQuantifierTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	p := f.constant("qe-free-p", f.boolean)
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(p))
	out := f.result()
	m.ResourceLimit().Cancel()
	err = port.Apply(goal, out)
	m.ResourceLimit().ResetCancel()
	if err == nil {
		t.Fatal("canceled goal succeeded")
	}
	fmMust(t, port.Apply(goal, out))
	depth, err := goal.Depth()
	fmMust(t, err)
	if depth != 1 {
		t.Fatal("goal depth", depth)
	}
	size, err := goal.Size()
	fmMust(t, err)
	if size != 1 {
		t.Fatal("quantifier-free goal changed")
	}
	h, err := goal.Form(0)
	fmMust(t, err)
	if h != p {
		t.Fatal("formula changed")
	}
	integerVar := f.own(m.MkVar(0, f.integer))
	freeInteger := f.constant("qe-unsupported-bound", f.integer)
	square := f.own(m.MkArithmeticMul(integerVar, integerVar))
	inequality := f.own(m.MkArithmeticGT(square, freeInteger))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: inequality}))
	quantified := f.goal(false, false)
	fmMust(t, quantified.Assert(q))
	second := f.result()
	if err = port.Apply(quantified, second); err == nil {
		t.Fatal("unfinished quantifier elimination accepted")
	}
	depth, err = quantified.Depth()
	fmMust(t, err)
	if depth != 0 {
		t.Fatal("refused goal was modified")
	}
}

func TestQuantifierTacticBooleanUnitsAndAtomicRefusal(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	port, err := NewQuantifierTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	v := f.own(m.MkVar(0, f.boolean))
	r := f.constant("qe-unit-free", f.boolean)
	conjunction := f.own(m.MkAnd(v, r))
	exists := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: conjunction}))
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(exists))
	out := f.result()
	fmMust(t, port.Apply(goal, out))
	value, err := goal.Form(0)
	fmMust(t, err)
	if value != r {
		t.Fatal("existential unit did not preserve its free condition")
	}
	xor := f.own(m.MkXor(v, r))
	xorQuantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: xor}))
	xorGoal := f.goal(false, false)
	fmMust(t, xorGoal.Assert(xorQuantifier))
	fmMust(t, port.Apply(xorGoal, f.result()))
	xorSize, err := xorGoal.Size()
	fmMust(t, err)
	if xorSize != 0 {
		t.Fatal("existential XOR did not reduce to true")
	}
	label := f.constant("qe-unit-label", f.boolean)
	dep, err := m.DependencyLeaf(label)
	fmMust(t, err)
	fmMust(t, m.DependencyInc(dep))
	defer func() { fmMust(t, m.DependencyDec(dep)) }()
	coreGoal := f.goal(false, true)
	fmMust(t, coreGoal.AssertWithProof(exists, ast.ASTHandle{}, dep))
	fmMust(t, port.Apply(coreGoal, f.result()))
	actual, err := coreGoal.Dependency(0)
	fmMust(t, err)
	if actual != dep {
		t.Fatal("quantifier rewrite lost its assertion dependency")
	}
	forall := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: v}))
	unsatGoal := f.goal(false, false)
	fmMust(t, unsatGoal.Assert(forall))
	fmMust(t, port.Apply(unsatGoal, f.result()))
	inconsistent, err := unsatGoal.Inconsistent()
	fmMust(t, err)
	if !inconsistent {
		t.Fatal("universal Boolean variable should be false")
	}
	integerVar := f.own(m.MkVar(0, f.integer))
	freeInteger := f.constant("qe-atomic-unsupported-bound", f.integer)
	square := f.own(m.MkArithmeticMul(integerVar, integerVar))
	positive := f.own(m.MkArithmeticGT(square, freeInteger))
	unsupported := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: positive}))
	refused := f.goal(false, false)
	fmMust(t, refused.Assert(exists))
	fmMust(t, refused.Assert(unsupported))
	if err = port.Apply(refused, f.result()); err == nil {
		t.Fatal("unsupported arithmetic quantifier was accepted")
	}
	unchanged, err := refused.Form(0)
	fmMust(t, err)
	if unchanged != exists {
		t.Fatal("refusal partially changed the goal")
	}
}

func TestQuantifierTacticBooleanPredicateBranches(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	port, err := NewQuantifierTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	v := f.own(m.MkVar(0, f.boolean))
	body := f.predicate("qe-branch-predicate", v)
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(quantifier))
	fmMust(t, port.Apply(goal, f.result()))
	value, err := goal.Form(0)
	fmMust(t, err)
	truth := f.own(m.MkTrue())
	falsity := f.own(m.MkFalse())
	positive := f.predicate("qe-branch-predicate", truth)
	negative := f.predicate("qe-branch-predicate", falsity)
	expected := f.own(m.MkOr(positive, negative))
	iff := f.own(m.MkIff(value, expected))
	different := f.own(m.MkNot(iff))
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("QE result differs from the two Boolean predicate branches")
	}
}

func TestQuantifierTacticUniversalIndependentPredicates(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	port, err := NewQuantifierTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	x := f.own(m.MkVar(0, f.boolean))
	y := f.own(m.MkVar(1, f.boolean))
	left := f.predicate("qe-forall-left", x)
	right := f.predicate("qe-forall-right", y)
	body := f.own(m.MkOr(left, right))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean, f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body}))
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(quantifier))
	fmMust(t, port.Apply(goal, f.result()))
	count, err := goal.Size()
	fmMust(t, err)
	forms := make([]ast.ASTHandle, count)
	for i := range forms {
		forms[i], err = goal.Form(uint32(i))
		fmMust(t, err)
	}
	value := f.own(m.MkAndArray(forms))
	truth := f.own(m.MkTrue())
	falsity := f.own(m.MkFalse())
	leftTrue := f.predicate("qe-forall-left", truth)
	leftFalse := f.predicate("qe-forall-left", falsity)
	rightTrue := f.predicate("qe-forall-right", truth)
	rightFalse := f.predicate("qe-forall-right", falsity)
	leftBoth := f.own(m.MkAnd(leftTrue, leftFalse))
	rightBoth := f.own(m.MkAnd(rightTrue, rightFalse))
	expected := f.own(m.MkOr(leftBoth, rightBoth))
	iff := f.own(m.MkIff(value, expected))
	different := f.own(m.MkNot(iff))
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("universal partition result differs from Boolean expansion")
	}
}

func TestQuantifierTacticAlternatingBooleanQuantifiers(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	port, err := NewQuantifierTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	x := f.own(m.MkVar(1, f.boolean))
	y := f.own(m.MkVar(0, f.boolean))
	equality := f.own(m.MkIff(x, y))
	inner := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(1)}, Body: equality}))
	outer := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: inner}))
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(outer))
	fmMust(t, port.Apply(goal, f.result()))
	size, err := goal.Size()
	fmMust(t, err)
	if size != 0 {
		t.Fatal("forall x exists y (x = y) should be true")
	}
}

func TestQuantifierTacticArithmeticEquation(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	port, err := NewQuantifierTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("qe-equation-y", f.integer)
	zero := f.number("0")
	equality := f.own(m.MkEq(x, y))
	positive := f.own(m.MkArithmeticGT(x, zero))
	body := f.own(m.MkAnd(equality, positive))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(quantifier))
	fmMust(t, port.Apply(goal, f.result()))
	value, err := goal.Form(0)
	fmMust(t, err)
	expected := f.own(m.MkArithmeticGT(y, zero))
	iff := f.own(m.MkIff(value, expected))
	different := f.own(m.MkNot(iff))
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("arithmetic QE result differs from substitution")
	}
}

func TestQuantifierTacticUnsupportedDivisibilityIsExplicit(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	port, err := NewQuantifierTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("qe-unsupported-divisibility", f.integer)
	twoX := f.own(m.MkArithmeticMul(f.number("2"), x))
	body := f.own(m.MkEq(twoX, y))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(quantifier))
	err = port.Apply(goal, f.result())
	if err == nil || !strings.Contains(err.Error(), "arithmetic branch resolution is not implemented") {
		t.Fatal("unsupported branch did not report its limitation", err)
	}
	unchanged, err := goal.Form(0)
	fmMust(t, err)
	if unchanged != quantifier {
		t.Fatal("unsupported branch changed the goal")
	}
}

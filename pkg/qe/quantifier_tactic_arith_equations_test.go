package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierArithmeticUnitEquationElimination(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("arith-equation-free", f.integer)
	zero := f.number("0")
	equality := f.own(m.MkEq(x, y))
	positive := f.own(m.MkArithmeticGT(x, zero))
	body := f.own(m.MkAnd(equality, positive))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	m.ResourceLimit().Cancel()
	_, _, canceled := eliminateQuantifierArithmeticEquations(m, quantifier, f.r.config)
	m.ResourceLimit().ResetCancel()
	if canceled == nil {
		t.Fatal("canceled arithmetic elimination succeeded")
	}
	result, complete, err := eliminateQuantifierArithmeticEquations(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("integer unit equation was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
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
		t.Fatal("integer equation result differs from substitution")
	}
	notEquality := f.own(m.MkNot(equality))
	forall := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: notEquality}))
	falseResult, complete, err := eliminateQuantifierArithmeticEquations(m, forall, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("universal disequality was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&falseResult)) }()
	falseValue, err := m.RefGet(falseResult)
	fmMust(t, err)
	if !m.IsFalse(falseValue) {
		t.Fatal("universal disequality must be false")
	}
}

func TestQuantifierArithmeticRealEquationElimination(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.real))
	y := f.constant("arith-real-equation-free", f.real)
	two := f.own(m.ArithmeticRealNumeralText("2"))
	product := f.own(m.MkArithmeticMul(two, x))
	equality := f.own(m.MkEq(product, y))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.real}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: equality}))
	result, complete, err := eliminateQuantifierArithmeticEquations(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("real equation was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("a nonzero real coefficient should be solvable")
	}
}

func TestQuantifierArithmeticEquationFreshVariable(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.integer))
	y := f.own(m.MkVar(1, f.integer))
	twoX := f.own(m.MkArithmeticMul(f.number("2"), x))
	threeY := f.own(m.MkArithmeticMul(f.number("3"), y))
	linear := f.own(m.MkArithmeticAdd(twoX, threeY))
	left := f.own(m.MkArithmeticAdd(linear, f.number("1")))
	body := f.own(m.MkEq(left, f.number("0")))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer, f.integer}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body}))
	result, complete, err := eliminateQuantifierArithmeticEquations(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("integer equation with a fresh variable was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("equation 2x + 3y + 1 = 0 should be satisfiable")
	}
}

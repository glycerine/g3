package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierMixedBooleanArithmeticGroups(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	b := f.own(m.MkVar(1, f.boolean))
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("mixed-qe-y", f.integer)
	zero := f.number("0")
	equality := f.own(m.MkEq(x, y))
	positive := f.own(m.MkArithmeticGT(x, zero))
	body := f.own(m.MkAndArray([]ast.ASTHandle{b, equality, positive}))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean, f.integer}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body}))
	result, complete, err := eliminateQuantifierMixed(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("mixed Boolean and integer block was not eliminated")
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
		t.Fatal("mixed result differs from the free arithmetic condition")
	}
	independentBody := f.own(m.MkAnd(b, positive))
	independent := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean, f.integer}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: independentBody}))
	independentResult, complete, err := eliminateQuantifierMixed(m, independent, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("independent mixed inequality was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&independentResult)) }()
	independentValue, err := m.RefGet(independentResult)
	fmMust(t, err)
	if !m.IsTrue(independentValue) {
		t.Fatal("mixed Boolean and positive integer witnesses should exist")
	}
	notEq := f.own(m.MkNot(equality))
	implies := f.own(m.MkImplies(b, notEq))
	forall := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean, f.integer}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: implies}))
	falseResult, complete, err := eliminateQuantifierMixed(m, forall, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("universal mixed block was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&falseResult)) }()
	falseValue, err := m.RefGet(falseResult)
	fmMust(t, err)
	if !m.IsFalse(falseValue) {
		t.Fatal("universal mixed formula should be false")
	}
}

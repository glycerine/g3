package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalRelationToFormula(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, m, h); return h }
	x, y := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	zero, two := own(m.ArithmeticRealNumeralText("0")), own(m.ArithmeticRealNumeralText("2"))
	r := NewIntervalFull(2)
	astTestMust(t, r.FilterInterpreted(m, own(m.MkArithmeticLT(zero, x))))
	astTestMust(t, r.FilterInterpreted(m, own(m.MkArithmeticLE(x, two))))
	astTestMust(t, r.FilterIdentical(0, 1))
	formula, err := r.ToFormula(m, []ast.ASTHandle{real, real})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&formula)) }()
	got, err := m.RefGet(formula)
	astTestMust(t, err)
	want := own(m.MkAndArray([]ast.ASTHandle{own(m.MkEq(x, y)), own(m.MkArithmeticLT(zero, y)), own(m.MkArithmeticLE(y, two))}))
	if got != want {
		t.Fatal("interval relation formula differs from source order")
	}
	if _, err = r.ToFormula(m, []ast.ASTHandle{real}); err == nil {
		t.Fatal("wrong formula arity accepted")
	}
	full, err := NewIntervalFull(1).ToFormula(m, []ast.ASTHandle{real})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&full)) }()
	h, err := m.RefGet(full)
	astTestMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("full interval should produce true")
	}
	emptyFormula, err := NewIntervalEmpty(1).ToFormula(m, []ast.ASTHandle{real})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&emptyFormula)) }()
	emptyHandle, err := m.RefGet(emptyFormula)
	astTestMust(t, err)
	if !m.IsTrue(emptyHandle) {
		t.Fatal("source empty flag must not alter the reset interval payload formula")
	}
}

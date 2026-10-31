package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBoundRelationToFormula(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, m, h); return h }
	x, y, z := own(m.MkVar(0, integer)), own(m.MkVar(1, integer)), own(m.MkVar(2, integer))
	r := NewBoundRelation(3, false)
	astTestMust(t, r.AddLess(0, 2))
	astTestMust(t, r.Equate(0, 1))
	formula, err := r.ToFormula(m, []ast.ASTHandle{integer, integer, integer})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&formula)) }()
	got, err := m.RefGet(formula)
	astTestMust(t, err)
	want := own(m.MkAnd(own(m.MkEq(x, y)), own(m.MkArithmeticLT(y, z))))
	if got != want {
		t.Fatal("bound relation formula differs from source order")
	}
	if _, err = r.ToFormula(m, []ast.ASTHandle{integer}); err == nil {
		t.Fatal("wrong formula arity accepted")
	}
	empty := NewBoundRelation(0, false)
	trueRef, err := empty.ToFormula(m, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&trueRef)) }()
	h, err := m.RefGet(trueRef)
	astTestMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("empty conjunction must be true")
	}
}

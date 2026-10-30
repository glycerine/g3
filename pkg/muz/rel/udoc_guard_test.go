package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestUDocGuardRecognition(t *testing.T) {
	docManager, _ := newTestDocManager(t, 2)
	relation, err := NewUDocRelation(docManager, []uint32{2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	m := docASTManager(t)
	if _, err = m.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		t.Fatal(err)
	}
	bv, err := m.BitVectorSort(4)
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkVar(0, bv)
	if err != nil {
		t.Fatal(err)
	}
	part, err := m.MkBitVectorExtract(2, 1, x)
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.MkVar(1, boolean)
	if err != nil {
		t.Fatal(err)
	}
	neg, err := m.MkNot(b)
	if err != nil {
		t.Fatal(err)
	}
	compound, err := m.MkAnd(b, neg)
	if err != nil {
		t.Fatal(err)
	}
	truth, err := m.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	falsity, err := m.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	constant, err := m.BitVectorNumeralAllOnes(2)
	if err != nil {
		t.Fatal(err)
	}
	equality, err := m.MkEq(part, constant)
	if err != nil {
		t.Fatal(err)
	}
	for _, expression := range []ast.ASTHandle{truth, falsity, b, neg, compound, equality} {
		ok, e := relation.IsGuard(m, expression)
		if e != nil || !ok {
			t.Fatalf("expected guard %v: %v", expression, e)
		}
	}
	i, err := m.MkVar(2, bv)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := relation.IsGuard(m, i)
	if err != nil || !ok {
		t.Fatalf("source accepts any Boolean variable expression check as variable: %v", err)
	}
	if _, err = relation.IsGuard(m, ast.ASTHandle{}); err == nil {
		t.Fatal("zero handle accepted")
	}
}

package rewriter

// Source: src/ast/rewriter/der.cpp:98-132,153-165.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func derQuantifier(t *testing.T, m *ast.ASTManager, forall bool, sorts []ast.ASTHandle, body ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	names := make([]usymbol.Symbol, len(sorts))
	for i := range names {
		names[i] = astTestSymbol(t, m, "der-bound-"+string(rune('a'+i)))
	}
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: body})
	astTestMust(t, err)
	astTestPin(t, m, q)
	return q
}

func derReduce(t *testing.T, d *DER, m *ast.ASTManager, q ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	proof, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, m.DelRef(&proof))
		astTestMust(t, m.DelRef(&out))
	})
	astTestMust(t, d.ReduceQuantifier(q, out, proof))
	result, err := m.RefGet(out)
	astTestMust(t, err)
	return result
}

func TestDERExistentialUnitEqualityAndBooleanDuals(t *testing.T) {
	f := newFactorFixture(t)
	d, err := NewDER(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	v, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	equality, err := f.m.MkEq(v, f.numeral(7))
	astTestMust(t, err)
	truth, err := f.m.MkTrue()
	astTestMust(t, err)
	if got := derReduce(t, d, f.m, derQuantifier(t, f.m, false, []ast.ASTHandle{f.sort}, equality)); got != truth {
		t.Fatal("existential equality", got, truth)
	}
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	bv, err := f.m.MkVar(0, boolean)
	astTestMust(t, err)
	notBV, err := f.m.MkNot(bv)
	astTestMust(t, err)
	for _, body := range []ast.ASTHandle{bv, notBV} {
		if got := derReduce(t, d, f.m, derQuantifier(t, f.m, false, []ast.ASTHandle{boolean}, body)); got != truth {
			t.Fatal("existential Boolean unit", body, got)
		}
	}
}

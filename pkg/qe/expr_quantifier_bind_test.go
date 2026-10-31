package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBindExprQuantifierVariablesOnlyWhenUsed(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x := f.constant("bind-x", f.boolean)
	unused := f.constant("bind-unused", f.boolean)
	y := f.constant("bind-y", f.boolean)
	formula := f.own(m.MkOr(x, y))
	result, err := bindExprQuantifierVariables(m, []ast.ASTHandle{x, unused, y}, formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	q, err := m.RefGet(result)
	fmMust(t, err)
	forall, err := m.IsForall(q)
	fmMust(t, err)
	if forall {
		t.Fatal("expected existential quantifier")
	}
	n, err := m.GetNumDeclarations(q)
	fmMust(t, err)
	if n != 2 {
		t.Fatalf("got %d declarations", n)
	}
	w, err := m.GetQuantifierWeight(q)
	fmMust(t, err)
	if w != 1 {
		t.Fatal("wrong quantifier weight", w)
	}
	for i, v := range []ast.ASTHandle{x, y} {
		decl, e := m.GetDecl(v)
		fmMust(t, e)
		want, e := m.GetASTName(decl)
		fmMust(t, e)
		got, e := m.GetDeclarationName(q, uint32(i))
		fmMust(t, e)
		if got != want {
			t.Fatal("declaration order", i)
		}
	}
	body, err := m.GetQuantifierBody(q)
	fmMust(t, err)
	for i, want := range []uint32{1, 0} {
		v, e := m.GetArg(body, uint32(i))
		fmMust(t, e)
		idx, e := m.GetVarIndex(v)
		fmMust(t, e)
		if idx != want {
			t.Fatal("variable index", idx, want)
		}
	}
	unchanged, err := bindExprQuantifierVariables(m, []ast.ASTHandle{unused}, formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&unchanged)) }()
	h, err := m.RefGet(unchanged)
	fmMust(t, err)
	if h != formula {
		t.Fatal("unused variable introduced quantifier")
	}
}

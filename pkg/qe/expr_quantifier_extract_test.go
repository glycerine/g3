package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExtractExprQuantifierVariableOrderAndFreeShift(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	sym, err := m.Symbols.Intern("extract-predicate")
	fmMust(t, err)
	decl, err := m.MkFuncDecl(sym, []ast.ASTHandle{f.boolean, f.boolean, f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	fmMust(t, err)
	v0 := f.own(m.MkVar(0, f.boolean))
	v1 := f.own(m.MkVar(1, f.boolean))
	v2 := f.own(m.MkVar(2, f.boolean))
	body := f.own(m.MkApp(decl, []ast.ASTHandle{v1, v0, v2}))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean, f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body}))
	extracted, err := extractExprQuantifier(m, q)
	fmMust(t, err)
	defer func() { fmMust(t, extracted.Close()) }()
	if len(extracted.variables) != 2 {
		t.Fatal("wrong number of fresh variables")
	}
	h, err := m.RefGet(extracted.body)
	fmMust(t, err)
	for i := uint32(0); i < 2; i++ {
		arg, e := m.GetArg(h, i)
		fmMust(t, e)
		fresh, e := m.RefGet(extracted.variables[i])
		fmMust(t, e)
		if arg != fresh {
			t.Fatal("bound variable order", i)
		}
	}
	free, err := m.GetArg(h, 2)
	fmMust(t, err)
	idx, err := m.GetVarIndex(free)
	fmMust(t, err)
	if idx != 0 {
		t.Fatalf("free variable index %d, want zero", idx)
	}
}

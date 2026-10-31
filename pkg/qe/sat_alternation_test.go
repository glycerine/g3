package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSATAlternationSkolemPrefixAndPolarity(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	sym := func(name string) usymbol.Symbol { s, e := m.Symbols.Intern(name); fmMust(t, e); return s }
	decl, err := m.MkFuncDecl(sym("sat-alt-p"), []ast.ASTHandle{f.boolean, f.boolean, f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	fmMust(t, err)
	x := f.own(m.MkVar(2, f.boolean))
	y := f.own(m.MkVar(1, f.boolean))
	z := f.own(m.MkVar(0, f.boolean))
	body := f.own(m.MkApp(decl, []ast.ASTHandle{x, y, z}))
	quantify := func(forall bool, name string, h ast.ASTHandle) ast.ASTHandle {
		return f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: []ast.ASTHandle{f.boolean}, Names: []usymbol.Symbol{sym(name)}, Body: h}))
	}
	inner := quantify(false, "sat-alt-z", body)
	middle := quantify(true, "sat-alt-y", inner)
	outer := quantify(false, "sat-alt-x", middle)
	alt, err := extractSATAlternation(m, f.r.config, outer)
	fmMust(t, err)
	defer func() { fmMust(t, alt.Close()) }()
	if len(alt.variables) != 2 || len(alt.variables[0]) != 1 || len(alt.variables[1]) != 1 {
		t.Fatal("alternating blocks", len(alt.variables))
	}
	final, err := m.RefGet(alt.formula)
	fmMust(t, err)
	quantified, err := m.HasQuantifiers(final)
	fmMust(t, err)
	if quantified {
		t.Fatal("quantifier survived extraction")
	}
	op, err := m.GetDeclKind(final)
	fmMust(t, err)
	if op == ast.ASTOpNot {
		t.Fatal("even alternation unexpectedly negated")
	}
	unary, err := m.MkFuncDecl(sym("sat-alt-unary"), []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	fmMust(t, err)
	atom := f.own(m.MkApp(unary, []ast.ASTHandle{z}))
	q := quantify(true, "sat-alt-only", atom)
	single, err := extractSATAlternation(m, f.r.config, q)
	fmMust(t, err)
	defer func() { fmMust(t, single.Close()) }()
	if len(single.variables) != 1 {
		t.Fatal("single universal block", len(single.variables))
	}
	final, err = m.RefGet(single.formula)
	fmMust(t, err)
	op, err = m.GetDeclKind(final)
	fmMust(t, err)
	if op != ast.ASTOpNot {
		t.Fatal("odd alternation not negated", op)
	}
}

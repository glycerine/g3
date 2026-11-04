package duality_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQueryClauseSparseAndGroundVariables(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("duality-query-p")
	must(err)
	pred, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	x, err := m.MkVar(2, integer)
	must(err)
	query, err := m.MkApp(pred, []ast.ASTHandle{x})
	must(err)
	clause, err := duality.QueryClause(m, query)
	must(err)
	defer func() { must(m.DelRef(&clause)) }()
	h, err := m.RefGet(clause)
	must(err)
	forall, err := m.IsForall(h)
	must(err)
	if !forall {
		t.Fatal("expected forall")
	}
	count, err := m.GetNumDeclarations(h)
	must(err)
	if count != 3 {
		t.Fatal("sparse binder count", count)
	}
	for i, want := range []ast.ASTHandle{integer, boolean, boolean} {
		got, e := m.GetDeclarationSort(h, uint32(i))
		must(e)
		if got != want {
			t.Fatal("binder sort", i, got, want)
		}
		name, e := m.GetDeclarationName(h, uint32(i))
		must(e)
		if name != symbol.Numerical(uint32(2-i)) {
			t.Fatal("binder name", i, name)
		}
	}
	body, err := m.GetQuantifierBody(h)
	must(err)
	op, err := m.GetDeclKind(body)
	must(err)
	if op != ast.ASTOpImplies {
		t.Fatal("query clause operator", op)
	}
	lhs, err := m.GetArg(body, 0)
	must(err)
	rhs, err := m.GetArg(body, 1)
	must(err)
	if lhs != query || !m.IsFalse(rhs) {
		t.Fatal("query implication", lhs, rhs)
	}
	ground, err := m.MkApp(pred, []ast.ASTHandle{zero})
	must(err)
	groundClause, err := duality.QueryClause(m, ground)
	must(err)
	defer func() { must(m.DelRef(&groundClause)) }()
	body, err = m.RefGet(groundClause)
	must(err)
	kind, err := m.GetKind(body)
	must(err)
	if kind != ast.ASTApp {
		t.Fatal("ground clause introduced a quantifier", kind)
	}
	op, err = m.GetDeclKind(body)
	must(err)
	if op != ast.ASTOpImplies {
		t.Fatal("ground clause operator", op)
	}
}

// Source: duality_dl_interface.cpp computes used_vars over the complete query
// before binding its free variables. Inner quantifier variables do not become
// new outer variables, and their binder depth shifts free de Bruijn indices.
func TestQueryClauseNestedBinderAndSparseFreeVariable(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("duality-nested-query")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	bound, err := m.MkVar(0, boolean)
	must(err)
	free, err := m.MkVar(2, integer) // Outer variable 1 under one binder.
	must(err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{bound, free})
	must(err)
	query, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(9)}, Body: body})
	must(err)
	queryRef, err := m.NewRef(query)
	must(err)
	defer func() { must(m.DelRef(&queryRef)) }()
	clause, err := duality.QueryClause(m, query)
	must(err)
	defer func() { must(m.DelRef(&clause)) }()
	h, err := m.RefGet(clause)
	must(err)
	forall, err := m.IsForall(h)
	must(err)
	count, err := m.GetNumDeclarations(h)
	must(err)
	if !forall || count != 2 {
		t.Fatal("outer closure included an inner binder or omitted a free slot", forall, count)
	}
	for i, want := range []ast.ASTHandle{integer, boolean} {
		sort, err := m.GetDeclarationSort(h, uint32(i))
		must(err)
		name, err := m.GetDeclarationName(h, uint32(i))
		must(err)
		if sort != want || name != symbol.Numerical(uint32(1-i)) {
			t.Fatal("outer binder", i, sort, name)
		}
	}
	implication, err := m.GetQuantifierBody(h)
	must(err)
	left, err := m.GetArg(implication, 0)
	must(err)
	right, err := m.GetArg(implication, 1)
	must(err)
	if left != query || !m.IsFalse(right) {
		t.Fatal("nested query was altered during closure")
	}
}

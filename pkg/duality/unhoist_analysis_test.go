package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityUnhoistRelationLookupAndVariableOrder(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	p, err := ctx.BoolConst("analysis-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("analysis-q")
	must(err)
	closeExpr(t, q)
	hidden, err := ctx.BoolConst("analysis-pattern-only")
	must(err)
	closeExpr(t, hidden)
	boolean, err := m.GetSort(raw(p))
	must(err)
	v := wrap(m.MkVar(0, boolean))
	name, err := m.Symbols.Intern("analysis-R")
	must(err)
	decl, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	relation, err := NewFuncDecl(ctx, decl)
	must(err)
	defer relation.Close()
	rp := wrap(m.MkApp(decl, []ast.ASTHandle{raw(p)}))
	rq := wrap(m.MkApp(decl, []ast.ASTHandle{raw(q)}))
	annotation := wrap(m.MkApp(decl, []ast.ASTHandle{raw(hidden)}))
	pattern := wrap(m.MkPattern([]ast.ASTHandle{raw(annotation)}))
	body := wrap(m.MkAndArgs([]ast.ASTHandle{raw(q), raw(v), raw(rp), raw(q)}))
	quant := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(body), Patterns: []ast.ASTHandle{raw(pattern)}}))
	root := wrap(m.MkAnd(raw(quant), raw(rq)))
	vars, err := GetVars(root)
	must(err)
	defer func() {
		for _, e := range vars {
			must(e.Close())
		}
	}()
	if len(vars) != 2 || raw(vars[0]) != raw(q) || raw(vars[1]) != raw(p) {
		t.Fatal("variable order, deduplication or annotation exclusion changed")
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer g.Close()
	parent, err := g.CreateNode(p)
	must(err)
	child, err := g.CreateNode(q)
	must(err)
	trans, err := NewTransformer(ctx, []*FuncDecl{relation}, nil, root)
	must(err)
	defer trans.Close()
	edge, err := g.CreateEdge(parent, trans, []*Node{child})
	must(err)
	found, err := g.GetRel(edge, 0)
	must(err)
	if found == nil {
		t.Fatal("relation missing")
	}
	defer found.Close()
	if raw(found) != raw(rp) {
		t.Fatal("relation lookup did not follow source depth-first body order")
	}
	absent, err := NewTransformer(ctx, []*FuncDecl{relation}, nil, p)
	must(err)
	defer absent.Close()
	absentEdge, err := g.CreateEdge(parent, absent, []*Node{child})
	must(err)
	got, err := g.GetRel(absentEdge, 0)
	must(err)
	if got != nil {
		got.Close()
		t.Fatal("missing relation unexpectedly found")
	}
	// A relation appearing only in a quantifier annotation must stay invisible.
	annotated, err := NewTransformer(ctx, []*FuncDecl{relation}, nil, wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(v), Patterns: []ast.ASTHandle{raw(pattern)}})))
	must(err)
	defer annotated.Close()
	annotationEdge, err := g.CreateEdge(parent, annotated, []*Node{child})
	must(err)
	got, err = g.GetRel(annotationEdge, 0)
	must(err)
	if got != nil {
		got.Close()
		t.Fatal("lookup traversed pattern")
	}
	if _, err := g.GetRel(edge, -1); err == nil {
		t.Fatal("negative child accepted")
	}
	if _, err := g.GetRel(edge, 1); err == nil {
		t.Fatal("out of range child accepted")
	}
	must(g.Close())
	if raw(found) != raw(rp) {
		t.Fatal("lookup result lost ownership after graph closure")
	}
}

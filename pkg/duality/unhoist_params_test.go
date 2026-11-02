package duality

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDualityUnhoistParameterExtensionOwnershipAndSignatures(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := ctx.BoolConst("params-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("params-q")
	must(err)
	closeExpr(t, q)
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	g, err := NewRPFP(ctx)
	must(err)
	defer g.Close()
	node, err := g.CreateNode(p)
	must(err)
	old, err := m.RefGet(node.Name)
	must(err)
	before := []ast.ASTHandle{}
	for _, relation := range []*Transformer{node.Annotation, node.Bound, node.Underapprox} {
		h, err := m.RefGet(relation.formula)
		must(err)
		before = append(before, h)
	}
	// Repeated appended expressions are positional, not deduplicated.
	must(g.AddParamsToNode(node, []*Expr{q, q}))
	fresh, err := m.RefGet(node.Name)
	must(err)
	if fresh == old {
		t.Fatal("extended predicate was not fresh")
	}
	arity, err := m.GetArity(fresh)
	must(err)
	if arity != 2 {
		t.Fatal("extended arity mismatch")
	}
	qsort, err := m.GetSort(raw(q))
	must(err)
	for i := uint32(0); i < 2; i++ {
		sort, err := m.GetDomain(fresh, i)
		must(err)
		if sort != qsort {
			t.Fatal("parameter sort changed")
		}
	}
	for i, relation := range []*Transformer{node.Annotation, node.Bound, node.Underapprox} {
		if len(relation.individualParams) != 2 {
			t.Fatal("node relations disagree on arity")
		}
		h, err := m.RefGet(relation.formula)
		must(err)
		if h != before[i] {
			t.Fatal("extension substituted formula")
		}
		for _, ref := range relation.individualParams {
			h, err := m.RefGet(ref)
			must(err)
			if h != raw(q) {
				t.Fatal("parameter order mismatch")
			}
		}
	}
	decl, err := NewFuncDecl(ctx, fresh)
	must(err)
	defer decl.Close()
	app, err := AddParamsToApp(p, decl, []*Expr{q, q})
	must(err)
	defer app.Close()
	for i := uint32(0); i < 2; i++ {
		h, err := m.GetArg(raw(app), i)
		must(err)
		if h != raw(q) {
			t.Fatal("app extension changed argument")
		}
	}
	// Extend an existing non-nullary application with a different sort.
	hzero, err := m.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	must(err)
	zero, err := NewExpr(ctx, hzero)
	must(err)
	closeExpr(t, zero)
	// Preserve the original argument prefix.
	must(g.AddParamsToNode(node, []*Expr{zero}))
	third, err := m.RefGet(node.Name)
	must(err)
	nextDecl, err := NewFuncDecl(ctx, third)
	must(err)
	defer nextDecl.Close()
	expanded, err := AddParamsToApp(app, nextDecl, []*Expr{zero})
	must(err)
	defer expanded.Close()
	for i, want := range []ast.ASTHandle{raw(q), raw(q), raw(zero)} {
		h, err := m.GetArg(raw(expanded), uint32(i))
		must(err)
		if h != want {
			t.Fatal("existing arguments not preserved")
		}
	}
	// Appending nothing still freshens a node declaration, as the source does.
	must(g.AddParamsToNode(node, nil))
	fourth, err := m.RefGet(node.Name)
	must(err)
	if fourth == third {
		t.Fatal("empty extension failed to freshen name")
	}
	clone, err := node.Annotation.Clone()
	must(err)
	defer clone.Close()
	must(clone.AddParams(nil))
	must(g.Close())
	if _, err := expanded.Raw(); err != nil {
		t.Fatal("application lost ownership after graph closure", err)
	}
	for _, ref := range clone.individualParams {
		if _, err := m.RefGet(ref); err != nil {
			t.Fatal("clone parameter lost ownership", err)
		}
	}
	if err := g.AddParamsToNode(node, nil); err == nil {
		t.Fatal("closed graph accepted extension")
	}
}

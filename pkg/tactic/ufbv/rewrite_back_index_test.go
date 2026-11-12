package ufbv

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRewriteBackIndexReschedulesProcessedAcrossQuantifierAnnotations(t *testing.T) {
	m := cacheFixture(t)
	b, e := m.MkBoolSort()
	cacheMust(t, e)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		cacheMust(t, m.IncRef(h))
		t.Cleanup(func() { cacheMust(t, m.DecRef(h)) })
		return h
	}
	makeDecl := func(name uint32, arity int) ast.ASTHandle {
		t.Helper()
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = b
		}
		h, e := m.MkFuncDecl(symbol.Numerical(name), domain, b, ast.ASTFuncDeclInfo{})
		cacheMust(t, e)
		return pin(h)
	}
	f, g, constant := makeDecl(61, 1), makeDecl(62, 1), makeDecl(63, 0)
	a, e := m.MkApp(constant, nil)
	cacheMust(t, e)
	pin(a)
	fa, e := m.MkApp(f, []ast.ASTHandle{a})
	cacheMust(t, e)
	pin(fa)
	ga, e := m.MkApp(g, []ast.ASTHandle{a})
	cacheMust(t, e)
	pin(ga)
	both, e := m.MkAnd(fa, ga)
	cacheMust(t, e)
	pin(both)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{b},
		Names: []symbol.Symbol{symbol.Numerical(64)}, Body: fa,
		NoPatterns: []ast.ASTHandle{ga},
	})
	cacheMust(t, e)
	pin(q)
	base := make(map[ast.ASTHandle]uint32)
	for _, h := range []ast.ASTHandle{both, q, ga} {
		base[h], e = m.GetRefCount(h)
		cacheMust(t, e)
	}
	r, e := newRewriteBackIndex(m)
	cacheMust(t, e)
	t.Cleanup(func() { cacheMust(t, r.close()) })
	order, e := r.declarations(q)
	cacheMust(t, e)
	if len(order) != 2 || order[0] != f || order[1] != g {
		t.Fatal("quantifier child visitation must reach body before no-pattern", order)
	}
	for _, h := range []ast.ASTHandle{both, q, ga} {
		cacheMust(t, r.addProcessed(h))
	}
	if r.byDecl[f].values.Len() != 2 || r.byDecl[g].values.Len() != 3 || r.byDecl[constant] != nil || r.processed.Len() != 3 {
		t.Fatal("back index must visit body and no-pattern, but skip constants and interpreted conjunctions")
	}
	cacheMust(t, r.rescheduleProcessed(f))
	if r.processed.Len() != 1 || len(r.todo) != 2 || r.byDecl[f].values.Len() != 0 || r.byDecl[g].values.Len() != 1 {
		t.Fatal("reschedule must snapshot f occurrences then erase all their back-index memberships")
	}
	if found, e := r.processed.Contains(ga); e != nil || !found {
		t.Fatal("unrelated g-only expression was rescheduled")
	}
	queued := map[ast.ASTHandle]bool{}
	for _, h := range r.todo {
		queued[h] = true
	}
	if !queued[both] || !queued[q] {
		t.Fatal("reschedule lost processed formula or quantified annotation owner", r.todo)
	}
	for h, refs := range base {
		got, e := m.GetRefCount(h)
		cacheMust(t, e)
		if got != refs+1 {
			t.Fatal("todo transfer and remaining processed roots must retain one reference", h, got, refs)
		}
	}
	cacheMust(t, r.close())
	for h, refs := range base {
		got, e := m.GetRefCount(h)
		cacheMust(t, e)
		if got != refs {
			t.Fatal("back-index owner leaked expression", h, got, refs)
		}
	}
}

func TestRewriteBackIndexSetAllocationRefusalCanRetry(t *testing.T) {
	m := cacheFixture(t)
	b, e := m.MkBoolSort()
	cacheMust(t, e)
	x, e := m.MkVar(0, b)
	cacheMust(t, e)
	f, e := m.MkFuncDecl(symbol.Numerical(65), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	cacheMust(t, e)
	fx, e := m.MkApp(f, []ast.ASTHandle{x})
	cacheMust(t, e)
	cacheMust(t, m.IncRef(fx))
	t.Cleanup(func() { cacheMust(t, m.DecRef(fx)) })
	r, e := newRewriteBackIndex(m)
	cacheMust(t, e)
	t.Cleanup(func() { cacheMust(t, r.close()) })
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	if e := r.addProcessed(fx); !errors.Is(e, arena.ErrInjectedFailure) || len(r.byDecl) != 0 || r.processed.Len() != 0 || len(r.owned) != 0 || m.Budget().Used() != before {
		t.Fatal("refused expr-set allocation published an incomplete back index", e, r.byDecl, r.processed, r.owned)
	}
	m.Budget().FailAfter(-1)
	cacheMust(t, r.addProcessed(fx))
	if r.byDecl[f].values.Len() != 1 {
		t.Fatal("retry did not index the expression", r.byDecl[f])
	}
}

func TestRewriteBackIndexChildFirstAllocationRefusalPrefix(t *testing.T) {
	m := cacheFixture(t)
	b, e := m.MkBoolSort()
	cacheMust(t, e)
	x, e := m.MkVar(0, b)
	cacheMust(t, e)
	g, e := m.MkFuncDecl(symbol.Numerical(66), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	cacheMust(t, e)
	f, e := m.MkFuncDecl(symbol.Numerical(67), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	cacheMust(t, e)
	gx, e := m.MkApp(g, []ast.ASTHandle{x})
	cacheMust(t, e)
	fgx, e := m.MkApp(f, []ast.ASTHandle{gx})
	cacheMust(t, e)
	cacheMust(t, m.IncRef(fgx))
	t.Cleanup(func() { cacheMust(t, m.DecRef(fgx)) })
	r, e := newRewriteBackIndex(m)
	cacheMust(t, e)
	t.Cleanup(func() { cacheMust(t, r.close()) })
	order, e := r.declarations(fgx)
	cacheMust(t, e)
	if len(order) != 2 || order[0] != g || order[1] != f {
		t.Fatal("for_each_expr must process child declaration first", order)
	}
	before := m.Budget().Used()
	m.Budget().FailAfter(1)
	if e := r.addProcessed(fgx); !errors.Is(e, arena.ErrInjectedFailure) {
		t.Fatal("second declaration allocation refusal", e)
	}
	if r.byDecl[g] == nil || r.byDecl[f] != nil || r.processed.Len() != 0 || len(r.owned) != 0 || m.Budget().Used() != before+r.byDecl[g].header {
		t.Fatal("source child-first allocation prefix", r.byDecl, r.processed, r.owned, m.Budget().Used())
	}
	m.Budget().FailAfter(-1)
	cacheMust(t, r.addProcessed(fgx))
	if r.byDecl[g].values.Len() != 1 || r.byDecl[f].values.Len() != 1 {
		t.Fatal("retry did not complete both declaration memberships")
	}
}

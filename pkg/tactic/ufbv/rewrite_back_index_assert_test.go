package ufbv

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRewriteBackIndexPresentNilSetSourceGuards(t *testing.T) {
	m := cacheFixture(t)
	if diagnostic.DebugBuild {
		budget := arena.NewBudget(1 << 20)
		region, e := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: budget}})
		cacheMust(t, e)
		debug, e := diagnostic.NewDebugContext(region, budget)
		cacheMust(t, e)
		previous, e := diagnostic.BindProcessDebugContext(debug)
		cacheMust(t, e)
		t.Cleanup(func() {
			_, e := diagnostic.BindProcessDebugContext(previous)
			cacheMust(t, e)
			cacheMust(t, debug.Close())
			cacheMust(t, region.Close())
		})
	}
	b, e := m.MkBoolSort()
	cacheMust(t, e)
	x, e := m.MkVar(0, b)
	cacheMust(t, e)
	f, e := m.MkFuncDecl(symbol.Numerical(994), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	cacheMust(t, e)
	g, e := m.MkFuncDecl(symbol.Numerical(995), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	cacheMust(t, e)
	fx, e := m.MkApp(f, []ast.ASTHandle{x})
	cacheMust(t, e)
	gfx, e := m.MkApp(g, []ast.ASTHandle{fx})
	cacheMust(t, e)
	cacheMust(t, m.IncRef(gfx))
	t.Cleanup(func() { cacheMust(t, m.DecRef(gfx)) })
	want := func(t *testing.T, err error) {
		t.Helper()
		target := ast.ErrASTDomain
		if diagnostic.DebugBuild {
			target = diagnostic.ErrDebugAssertion
		}
		if !errors.Is(err, target) {
			t.Fatalf("present nil set did not follow source assertion/guard: %v", err)
		}
	}
	t.Run("insert-482", func(t *testing.T) {
		r, e := newRewriteBackIndex(m)
		cacheMust(t, e)
		r.byDecl[f] = nil
		want(t, r.addProcessed(fx))
		delete(r.byDecl, f)
		cacheMust(t, r.close())
	})
	t.Run("reschedule-518", func(t *testing.T) {
		r, e := newRewriteBackIndex(m)
		cacheMust(t, e)
		r.byDecl[f] = nil
		want(t, r.rescheduleProcessed(f))
		delete(r.byDecl, f)
		cacheMust(t, r.close())
	})
	t.Run("remove-507", func(t *testing.T) {
		r, e := newRewriteBackIndex(m)
		cacheMust(t, e)
		cacheMust(t, r.addProcessed(gfx))
		old := r.byDecl[f]
		r.byDecl[f] = nil
		want(t, r.rescheduleProcessed(g))
		r.byDecl[f] = old
		cacheMust(t, r.close())
	})
	t.Run("demodulator-reschedule-607", func(t *testing.T) {
		r, e := newRewriteBackIndex(m)
		cacheMust(t, e)
		forward, e := newRewriteRuleIndex(m)
		cacheMust(t, e)
		r.byDecl[f] = nil
		want(t, r.rescheduleDemodulators(forward, f, fx))
		delete(r.byDecl, f)
		cacheMust(t, r.close())
		cacheMust(t, forward.close())
	})
}

func TestRewriteBackIndexTypedRootRejectsInvalidSourcePointer(t *testing.T) {
	m := cacheFixture(t)
	r, e := newRewriteBackIndex(m)
	cacheMust(t, e)
	t.Cleanup(func() { cacheMust(t, r.close()) })
	before := m.Budget().Used()
	if e := r.addProcessed(ast.ASTHandle{}); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("invalid typed root reached the source back-index visitor", e)
	}
	if len(r.byDecl) != 0 || r.processed.Len() != 0 || len(r.owned) != 0 || m.Budget().Used() != before {
		t.Fatal("invalid root published back-index state", r.byDecl, r.processed, r.owned, m.Budget().Used(), before)
	}
}

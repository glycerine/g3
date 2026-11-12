package ufbv

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRewriteRuleIndexTriesRulesAndOwnsDemodulators(t *testing.T) {
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
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		h, e := m.MkApp(d, args)
		cacheMust(t, e)
		return pin(h)
	}
	x, e := m.MkVar(0, b)
	cacheMust(t, e)
	pin(x)
	y, e := m.MkVar(1, b)
	cacheMust(t, e)
	pin(y)
	f, g, ad, bd := makeDecl(51, 2), makeDecl(52, 1), makeDecl(53, 0), makeDecl(54, 0)
	a, c := app(ad), app(bd)
	firstLHS, secondLHS := app(f, x, x), app(f, x, y)
	firstRHS, secondRHS := app(g, x), app(g, y)
	want := app(g, c)
	q := func(name uint32) ast.ASTHandle {
		h, e := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{b},
			Names: []symbol.Symbol{symbol.Numerical(name)}, Body: x,
		})
		cacheMust(t, e)
		return pin(h)
	}
	q1, q2 := q(55), q(56)
	base := make(map[ast.ASTHandle]uint32)
	for _, h := range []ast.ASTHandle{q1, firstLHS, firstRHS, q2, secondLHS, secondRHS} {
		base[h], e = m.GetRefCount(h)
		cacheMust(t, e)
	}
	r, e := newRewriteRuleIndex(m)
	cacheMust(t, e)
	t.Cleanup(func() { cacheMust(t, r.close()) })
	beforeSet := m.Budget().Used()
	m.Budget().FailAfter(0)
	if e = r.insert(q1, firstLHS, firstRHS); !errors.Is(e, arena.ErrInjectedFailure) || len(r.byFunc) != 0 || len(r.owned) != 0 || m.Budget().Used() != beforeSet {
		t.Fatal("refused source quantifier-set allocation published an index or reference", e, r.byFunc, r.owned)
	}
	m.Budget().FailAfter(-1)
	cacheMust(t, r.insert(q1, firstLHS, firstRHS))
	set := r.byFunc[f]
	if set == nil || set.header == 0 || m.Budget().Used() != beforeSet+set.header {
		t.Fatal("first source quantifier-set allocation", set, m.Budget().Used(), beforeSet)
	}
	cacheMust(t, r.insert(q2, secondLHS, secondRHS))
	if r.byFunc[f] != set || m.Budget().Used() != beforeSet+set.header {
		t.Fatal("same function must reuse its source set", r.byFunc[f], m.Budget().Used())
	}
	for h, refs := range base {
		got, e := m.GetRefCount(h)
		cacheMust(t, e)
		if got != refs+1 {
			t.Fatal("indexed rule must retain quantifier and both expressions", h, got, refs)
		}
	}
	if r.byFunc[f].values.Len() != 2 {
		t.Fatal("forward index lost a rule")
	}
	if !ruleIndexConsistent(r) {
		t.Fatal("forward index lacks a demodulator mapping")
	}
	out, matched, e := r.tryRewrite(f, []ast.ASTHandle{a, c})
	cacheMust(t, e)
	if !matched {
		t.Fatal("second demodulator should match after first repeated-variable refusal")
	}
	got, e := m.RefGet(out)
	cacheMust(t, e)
	if got != want {
		t.Fatal("rewrite1 selected wrong RHS", got, want)
	}
	miss, matched, e := r.tryRewrite(f, []ast.ASTHandle{a})
	cacheMust(t, e)
	if matched || miss != (ast.ASTRef{}) {
		t.Fatal("arity mismatch should skip indexed rules")
	}
	cacheMust(t, r.remove(f, q1))
	if !ruleIndexConsistent(r) || r.byFunc[f].values.Len() != 1 || objSetKeys(r.byFunc[f].values)[0] != q2 {
		t.Fatal("remove_fwd_idx must erase only the selected quantifier", r.byFunc[f].values)
	}
	for _, h := range []ast.ASTHandle{q1, firstLHS, firstRHS} {
		refs, e := m.GetRefCount(h)
		cacheMust(t, e)
		if refs != base[h] {
			t.Fatal("remove_fwd_idx leaked a removed rule reference", h, refs, base[h])
		}
	}
	again, matched, e := r.tryRewrite(f, []ast.ASTHandle{a, c})
	cacheMust(t, e)
	if !matched {
		t.Fatal("remaining rule must still rewrite")
	}
	cacheMust(t, m.DelRef(&again))
	cacheMust(t, r.remove(f, q2))
	if !ruleIndexConsistent(r) || r.byFunc[f].values.Len() != 0 {
		t.Fatal("source keeps an empty forward function entry", r.byFunc[f].values)
	}
	miss, matched, e = r.tryRewrite(f, []ast.ASTHandle{a, c})
	cacheMust(t, e)
	if matched || miss != (ast.ASTRef{}) {
		t.Fatal("removed rules must no longer match")
	}
	cacheMust(t, r.close())
	for h, refs := range base {
		got, e := m.GetRefCount(h)
		cacheMust(t, e)
		if got != refs {
			t.Fatal("index close leaked a rule reference", h, got, refs)
		}
	}
	got, e = m.RefGet(out)
	cacheMust(t, e)
	if got != want {
		t.Fatal("caller result must outlive rule index")
	}
	cacheMust(t, m.DelRef(&out))
}

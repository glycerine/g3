package ufbv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMatchSubstConcretePairCacheAndVariableBindings(t *testing.T) {
	m := cacheFixture(t)
	b, e := m.MkBoolSort()
	cacheMust(t, e)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		cacheMust(t, m.IncRef(h))
		t.Cleanup(func() { cacheMust(t, m.DecRef(h)) })
		return h
	}
	declare := func(name string, arity int) ast.ASTHandle {
		t.Helper()
		n, e := m.Symbols.Intern(name)
		cacheMust(t, e)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = b
		}
		d, e := m.MkFuncDecl(n, domain, b, ast.ASTFuncDeclInfo{})
		cacheMust(t, e)
		return pin(d)
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
	f, g, h := declare("match_f", 2), declare("match_g", 1), declare("match_h", 1)
	a, c := app(declare("match_a", 0)), app(declare("match_c", 0))
	gx, gy, ga, gc := app(g, x), app(g, y), app(g, a), app(g, c)
	pattern := app(f, gx, gx)
	instance := app(f, ga, ga)
	s, e := newMatchSubst(m)
	cacheMust(t, e)
	t.Cleanup(func() { cacheMust(t, s.close()) })
	baseA, e := m.GetRefCount(a)
	cacheMust(t, e)
	baseC, e := m.GetRefCount(c)
	cacheMust(t, e)
	ok, e := s.isInstance(pattern, instance)
	cacheMust(t, e)
	if !ok || s.bindings[x] != a || len(s.cache) != 1 {
		t.Fatal("shared expression pair must be cached and bind x once", ok, s.bindings, s.cache)
	}
	refs, e := m.GetRefCount(a)
	cacheMust(t, e)
	if refs != baseA+1 {
		t.Fatal("substitution binding must retain its instance", refs, baseA)
	}

	ok, e = s.isInstance(pattern, app(f, ga, gc))
	cacheMust(t, e)
	if ok {
		t.Fatal("repeated variable bound to different arguments")
	}
	refsA, e := m.GetRefCount(a)
	cacheMust(t, e)
	refsC, e := m.GetRefCount(c)
	cacheMust(t, e)
	if refsA != baseA || refsC != baseC+1 {
		t.Fatal("reset must release old binding; failed match retains partial new binding", refsA, refsC)
	}
	ok, e = s.isInstance(pattern, app(f, ga, app(h, a)))
	cacheMust(t, e)
	if ok {
		t.Fatal("quick declaration mismatch")
	}

	// The source processes the last argument first. It binds x to a in
	// (g x, g a), then the equal non-ground (g x, g x) must reject that
	// non-self binding.
	ok, e = s.isInstance(pattern, app(f, gx, ga))
	cacheMust(t, e)
	if ok {
		t.Fatal("identical non-ground branch ignored earlier non-self binding")
	}
	ok, e = s.isInstance(app(f, x, gy), app(f, a, gy))
	cacheMust(t, e)
	if !ok || s.bindings[x] != a || s.bindings[y] != y {
		t.Fatal("identical non-ground branch did not bind unbound y to itself", ok, s.bindings)
	}
	// The later variable pair makes the overall match nontrivial, so the
	// earlier equal ground app reaches the source cpp844 assertion.
	ok, e = s.isInstance(app(f, a, x), app(f, a, c))
	cacheMust(t, e)
	if !ok || s.bindings[x] != c {
		t.Fatal("equal ground app did not preserve the variable binding", ok, s.bindings)
	}

	ok, e = s.isInstance(app(f, a, a), app(f, x, x))
	cacheMust(t, e)
	if ok {
		t.Fatal("ground pattern may not match a non-ground instance")
	}
	ok, e = s.isInstance(x, app(h, a))
	cacheMust(t, e)
	if !ok {
		t.Fatal("source direct variable instance check")
	}
	ok, e = s.isInstance(app(f, a, a), app(f, a, a))
	cacheMust(t, e)
	if !ok || !s.allEqual {
		t.Fatal("source equal-arguments fast path", ok, s.allEqual)
	}
	cacheMust(t, s.close())
	if len(s.owned) != 0 || len(s.bindings) != 0 {
		t.Fatal("matcher close retained substitution bindings")
	}
}

// Pattern f(x, g(x)) must not match f(c, g(x)): the identical non-ground
// pair (g(x), g(x)) binds x to itself, so the pair (x, c) fails. z3 4.7.1's
// match_args_aux_proc left x unbound and bound x := c, an unsound demodulation
// (AUDIT.md tactic_sls_ufbv_01-F1; fixed upstream in b76ed6a47).
func TestMatchSubstIdenticalSubtermBindsUnboundVariable(t *testing.T) {
	m := cacheFixture(t)
	b, e := m.MkBoolSort()
	cacheMust(t, e)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		cacheMust(t, m.IncRef(h))
		t.Cleanup(func() { cacheMust(t, m.DecRef(h)) })
		return h
	}
	declare := func(name string, arity int) ast.ASTHandle {
		t.Helper()
		n, e := m.Symbols.Intern(name)
		cacheMust(t, e)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = b
		}
		d, e := m.MkFuncDecl(n, domain, b, ast.ASTFuncDeclInfo{})
		cacheMust(t, e)
		return pin(d)
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
	f, g := declare("match_f", 2), declare("match_g", 1)
	c := app(declare("match_c", 0))
	gx := app(g, x)
	pattern := app(f, x, gx)
	s, e := newMatchSubst(m)
	cacheMust(t, e)
	t.Cleanup(func() { cacheMust(t, s.close()) })
	ok, e := s.matchArgs(pattern, []ast.ASTHandle{c, gx})
	cacheMust(t, e)
	if ok {
		t.Fatal("f(x, g(x)) matched f(c, g(x)) by binding x := c", s.bindings)
	}
	ok, e = s.isInstance(pattern, app(f, x, gx))
	cacheMust(t, e)
	if !ok {
		t.Fatal("f(x, g(x)) must match itself")
	}
}

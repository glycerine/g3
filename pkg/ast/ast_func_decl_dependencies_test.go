package ast

// Source: src/ast/func_decl_dependencies.cpp;
// src/ast/func_decl_dependencies.h.

import "testing"

func TestASTCollectFuncDeclsConcreteTraversalAndNonGroundFilter(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	decl := func(name string) ASTHandle {
		h, err := m.MkFuncDecl(astTestSymbol(t, m, name), []ASTHandle{b}, b, ASTFuncDeclInfo{})
		astTestMust(t, err)
		return h
	}
	f, g := decl("deps-f"), decl("deps-g")
	c, e := m.MkConst(astTestSymbol(t, m, "deps-c"), b)
	astTestMust(t, e)
	x, e := m.MkVar(0, b)
	astTestMust(t, e)
	fc, e := m.MkApp(f, []ASTHandle{c})
	astTestMust(t, e)
	gx, e := m.MkApp(g, []ASTHandle{x})
	astTestMust(t, e)
	root, e := m.MkAnd(fc, gx)
	astTestMust(t, e)
	astTestPin(t, m, root)

	all := NewFuncDeclSet(m)
	astTestMust(t, m.CollectFuncDecls(root, all, false))
	if !all.Contains(f) {
		t.Fatal("ground uninterpreted application declaration missing")
	}
	if !all.Contains(g) || all.Len() != 2 {
		t.Fatal("non-ground declaration collection", all)
	}
	nonGround := NewFuncDeclSet(m)
	astTestMust(t, m.CollectFuncDecls(root, nonGround, true))
	if nonGround.Contains(f) {
		t.Fatal("ground application survived non-ground filter")
	}
	if !nonGround.Contains(g) || nonGround.Len() != 1 {
		t.Fatal("non-ground application declaration missing", nonGround)
	}
}

func TestASTFuncDeclDependenciesCyclesOwnershipAndErase(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	decl := func(name string) ASTHandle {
		h, err := m.MkFuncDecl(astTestSymbol(t, m, name), []ASTHandle{b}, b, ASTFuncDeclInfo{})
		astTestMust(t, err)
		astTestPin(t, m, h)
		return h
	}
	a, bDecl, c := decl("deps-a"), decl("deps-b"), decl("deps-cyclic-c")
	d, e := NewFuncDeclDependencies(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	refs := func(h ASTHandle) uint32 { n, err := m.GetRefCount(h); astTestMust(t, err); return n }
	set := func(hs ...ASTHandle) *FuncDeclSet {
		s := NewFuncDeclSet(m)
		for _, h := range hs {
			astTestMust(t, s.Insert(h))
		}
		return s
	}
	baseA, baseB, baseC := refs(a), refs(bDecl), refs(c)
	ok, e := d.Insert(a, set(bDecl))
	astTestMust(t, e)
	if !ok || refs(a) != baseA+1 || refs(bDecl) != baseB+1 {
		t.Fatal("first dependency ownership")
	}
	ok, e = d.Insert(bDecl, set(c))
	astTestMust(t, e)
	if !ok || refs(bDecl) != baseB+2 || refs(c) != baseC+1 {
		t.Fatal("second dependency ownership")
	}
	ok, e = d.Insert(c, set(a))
	astTestMust(t, e)
	if ok || d.Contains(c) || refs(c) != baseC+1 || refs(a) != baseA+1 {
		t.Fatal("cycle insertion must be rejected without pins")
	}
	ok, e = d.Insert(a, set())
	astTestMust(t, e)
	if ok {
		t.Fatal("duplicate insertion accepted")
	}
	deps := d.GetDependencies(a)
	astTestMust(t, deps.table.Erase(bDecl))
	if d.GetDependencies(a).Len() != 1 {
		t.Fatal("dependency result exposed manager storage")
	}
	astTestMust(t, d.Erase(a))
	if d.Contains(a) || refs(a) != baseA || refs(bDecl) != baseB+1 {
		t.Fatal("erase ownership", refs(a), refs(bDecl))
	}
	astTestMust(t, d.Reset())
	if refs(bDecl) != baseB || refs(c) != baseC {
		t.Fatal("reset ownership", refs(bDecl), refs(c))
	}
}

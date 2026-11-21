package ast

import (
	"errors"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestExprAbstractorNativePatternAndNestedBinder(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	constantDecl, e := m.MkFuncDecl(astTestSymbol(t, m, "abstract-c"), nil, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	constant, e := m.MkApp(constantDecl, nil)
	astTestMust(t, e)
	fn, e := m.MkFuncDecl(astTestSymbol(t, m, "abstract-f"), []ASTHandle{b}, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	body, e := m.MkApp(fn, []ASTHandle{constant})
	astTestMust(t, e)
	pattern, e := m.MkFamilyApp(2, 0, nil, []ASTHandle{body})
	astTestMust(t, e)
	q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{b}, Names: []usymbol.Symbol{astTestSymbol(t, m, "inner")}, Body: body, Patterns: []ASTHandle{pattern}})
	astTestMust(t, e)
	astTestPin(t, m, q)
	out, e := m.NewRef(ASTHandle{})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&out)) })
	a, e := NewExprAbstractor(m)
	astTestMust(t, e)
	if e = a.Abstract(0, []ASTHandle{constant}, q, out); e != nil {
		t.Fatal(e)
	}
	result, e := m.RefGet(out)
	astTestMust(t, e)
	if result == q {
		t.Fatal("quantifier was not rebuilt")
	}
	rewrittenBody, e := m.GetQuantifierBody(result)
	astTestMust(t, e)
	arg, e := m.GetArg(rewrittenBody, 0)
	astTestMust(t, e)
	index, e := m.GetVarIndex(arg)
	astTestMust(t, e)
	if index != 1 {
		t.Fatal("outer binder index under inner quantifier", index)
	}
	rewrittenPattern, e := m.GetPattern(result, 0)
	astTestMust(t, e)
	patternBody, e := m.GetArg(rewrittenPattern, 0)
	astTestMust(t, e)
	if patternBody != rewrittenBody {
		t.Fatal("pattern and body did not share the rewritten application")
	}
	if len(a.pinned) != 0 {
		t.Fatal("temporary expression references were not released")
	}
	if e = a.Abstract(0, nil, constant, out); e != nil {
		t.Fatal("reused abstractor", e)
	}
	if got, e := m.RefGet(out); e != nil || got != constant {
		t.Fatal("zero-bound source shortcut", got, e)
	}
	if e = a.Abstract(0, []ASTHandle{constant}, q, out); e != nil {
		t.Fatal("reused abstractor with binder", e)
	}
	result, e = m.RefGet(out)
	astTestMust(t, e)
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	if got, e := m.RefGet(out); e != nil || got != result {
		t.Fatal("result lost after temporary pins released", got, e)
	}
}

func TestExprAbstractorRetainsInterruptedTemporaryUntilClose(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "abstract-fault-c"), nil, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	c, e := m.MkApp(d, nil)
	astTestMust(t, e)
	astTestPin(t, m, c)
	out, e := m.NewRef(c)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&out)) })
	a, e := NewExprAbstractor(m)
	astTestMust(t, e)
	if e = a.Abstract(0, []ASTHandle{c, {}}, c, out); !errors.Is(e, ErrASTHandle) {
		t.Fatal("invalid second bound constant", e)
	}
	if len(a.pinned) != 1 {
		t.Fatal("first generated variable was not retained for cleanup")
	}
	if got, e := m.RefGet(out); e != nil || got != c {
		t.Fatal("failed abstraction changed output", got, e)
	}
	copied := *a
	if e := copied.Close(); !errors.Is(e, ErrASTHandle) {
		t.Fatal("copied owner released original references", e)
	}
	if len(a.pinned) != 1 {
		t.Fatal("copied owner changed original pending references")
	}
	if e = a.Close(); e != nil {
		t.Fatal("temporary reference cleanup", e)
	}
	if len(a.pinned) != 0 || !a.closed {
		t.Fatal("cleanup did not finish")
	}
}

func TestExprAbstractorForallExistsConstantOrderAndEmpty(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	makeConstant := func(name string) ASTHandle {
		d, e := m.MkFuncDecl(astTestSymbol(t, m, name), nil, b, ASTFuncDeclInfo{})
		astTestMust(t, e)
		c, e := m.MkApp(d, nil)
		astTestMust(t, e)
		return c
	}
	x, y := makeConstant("abstract-x"), makeConstant("abstract-y")
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "abstract-pair"), []ASTHandle{b, b}, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	body, e := m.MkApp(d, []ASTHandle{x, y})
	astTestMust(t, e)
	astTestPin(t, m, body)
	out, e := m.NewRef(ASTHandle{})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&out)) })
	a, e := NewExprAbstractor(m)
	astTestMust(t, e)
	for _, tc := range []struct {
		forall bool
		call   func([]ASTHandle, ASTHandle, ASTRef) error
	}{{true, a.MkForall}, {false, a.MkExists}} {
		if e = tc.call([]ASTHandle{x, y}, body, out); e != nil {
			t.Fatal(e)
		}
		q, e := m.RefGet(out)
		astTestMust(t, e)
		forall, e := m.IsForall(q)
		astTestMust(t, e)
		if forall != tc.forall {
			t.Fatal("quantifier kind", forall, tc.forall)
		}
		count, e := m.GetNumDeclarations(q)
		astTestMust(t, e)
		if count != 2 {
			t.Fatal("declaration count", count)
		}
		for i, want := range []string{"abstract-x", "abstract-y"} {
			name, e := m.GetDeclarationName(q, uint32(i))
			astTestMust(t, e)
			spelling, e := m.Symbols.String(name)
			astTestMust(t, e)
			if spelling != want {
				t.Fatal("bound name order", i, spelling, want)
			}
		}
		qb, e := m.GetQuantifierBody(q)
		astTestMust(t, e)
		for i, want := range []uint32{1, 0} {
			arg, e := m.GetArg(qb, uint32(i))
			astTestMust(t, e)
			index, e := m.GetVarIndex(arg)
			astTestMust(t, e)
			if index != want {
				t.Fatal("de Bruijn order", i, index, want)
			}
		}
	}
	if e = a.MkForall(nil, body, out); e != nil {
		t.Fatal(e)
	}
	if got, e := m.RefGet(out); e != nil || got != body {
		t.Fatal("zero-bound wrapper created a quantifier", got, e)
	}
	astTestMust(t, a.Close())
}

func TestExprAbstractorSourceRootReferenceGuard(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "abstract-unpinned"), nil, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	c, e := m.MkApp(d, nil)
	astTestMust(t, e)
	refs, e := m.GetRefCount(c)
	astTestMust(t, e)
	if refs != 0 {
		t.Fatal("fixture needs an unreferenced source root", refs)
	}
	out, e := m.NewRef(ASTHandle{})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&out)) })
	a, e := NewExprAbstractor(m)
	astTestMust(t, e)
	e = a.Abstract(0, []ASTHandle{c}, c, out)
	if ASTSourceDebug {
		if !errors.Is(e, ErrASTDomain) || len(a.pinned) != 0 {
			t.Fatal("debug root-reference guard preceded generated pin", e, len(a.pinned))
		}
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if !got.IsNull() {
			t.Fatal("failed debug assertion changed output", got)
		}
	} else {
		astTestMust(t, e)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		index, e := m.GetVarIndex(got)
		astTestMust(t, e)
		if index != 0 {
			t.Fatal("release profile did not continue", index)
		}
	}
	if e = a.Abstract(0, nil, c, out); e != nil {
		t.Fatal("zero-bound shortcut must bypass the source root assertion", e)
	}
	if got, e := m.RefGet(out); e != nil || got != c {
		t.Fatal("zero-bound shortcut output", got, e)
	}
	astTestMust(t, a.Close())
}

func TestExprAbstractorReleasesUnusedVariableAtNestedBinder(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	c, err := m.MkConst(astTestSymbol(t, m, "outer-constant"), boolean)
	astTestMust(t, err)
	fn, err := m.MkFuncDecl(astTestSymbol(t, m, "use-outer"), []ASTHandle{boolean}, boolean, ASTFuncDeclInfo{})
	astTestMust(t, err)
	application, err := m.MkApp(fn, []ASTHandle{c})
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	inner, err := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolean}, Names: []usymbol.Symbol{astTestSymbol(t, m, "inner")}, Body: truth})
	astTestMust(t, err)
	root, err := m.MkAnd(application, inner)
	astTestMust(t, err)
	astTestPin(t, m, root)
	outerVar, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	astTestPin(t, m, outerVar)
	// Seed the free ID that the nested abstraction takes for its unused
	// variable. The right operand is visited first on the source work stack.
	v, err := m.MkVar(1, boolean)
	astTestMust(t, err)
	id, err := m.GetASTID(v)
	astTestMust(t, err)
	ref, err := m.NewRef(v)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&ref))
	out := astTestPin(t, m, root)
	a, err := NewExprAbstractor(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, a.Close()) })
	astTestMust(t, a.Abstract(0, []ASTHandle{c}, root, out))
	result, err := m.RefGet(out)
	astTestMust(t, err)
	child, err := m.GetArg(result, 0)
	astTestMust(t, err)
	got, err := m.GetASTID(child)
	astTestMust(t, err)
	if got != id {
		t.Fatalf("rewritten sibling ID %d, want the nested variable's released ID %d", got, id)
	}
}

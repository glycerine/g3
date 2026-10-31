package base_test

// Source: src/muz/base/bind_variables.cpp abstraction, binder ordering,
// duplicate registration, empty binding, and forall/exists branches.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestVariableBinderAbstractsImplicitConstantsInSourceOrder(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	manager, err := context.Manager()
	ruleMust(t, err)
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	constant := func(name string) ast.ASTHandle {
		symbol, e := manager.Symbols.Intern(name)
		ruleMust(t, e)
		declaration, e := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		value, e := manager.MkApp(declaration, nil)
		ruleMust(t, e)
		return value
	}
	x, y := constant("bind-x"), constant("bind-y")
	pSymbol, err := manager.Symbols.Intern("bind-p")
	ruleMust(t, err)
	pDeclaration, err := manager.MkFuncDecl(pSymbol, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	body, err := manager.MkApp(pDeclaration, []ast.ASTHandle{x, y})
	ruleMust(t, err)
	bodyRef, err := manager.NewRef(body)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, manager.DelRef(&bodyRef)) })
	binder, err := base.NewVariableBinder(manager)
	ruleMust(t, err)
	ruleMust(t, binder.AddVariable(x))
	ruleMust(t, binder.AddVariable(y))
	ruleMust(t, binder.AddVariable(x))

	for _, forall := range []bool{true, false} {
		result, e := binder.Bind(body, forall)
		ruleMust(t, e)
		value, e := manager.RefGet(result)
		ruleMust(t, e)
		isForall, e := manager.IsForall(value)
		ruleMust(t, e)
		if isForall != forall {
			t.Fatal("quantifier kind", isForall, forall)
		}
		if count, e := manager.GetNumDeclarations(value); e != nil || count != 2 {
			t.Fatal("binder count", count, e)
		}
		for i, want := range []string{"bind-x", "bind-y"} {
			name, e := manager.GetDeclarationName(value, uint32(i))
			ruleMust(t, e)
			spelling, e := manager.Symbols.String(name)
			ruleMust(t, e)
			if spelling != want {
				t.Fatal("binder name", i, spelling, want)
			}
		}
		quantifiedBody, e := manager.GetQuantifierBody(value)
		ruleMust(t, e)
		for i, want := range []uint32{1, 0} {
			argument, x := manager.GetArg(quantifiedBody, uint32(i))
			ruleMust(t, x)
			index, x := manager.GetVarIndex(argument)
			ruleMust(t, x)
			if index != want {
				t.Fatal("bound index", i, index, want)
			}
		}
		ruleMust(t, manager.DelRef(&result))
	}
	ruleMust(t, binder.Close())
	if _, err = binder.Bind(body, true); !errors.Is(err, base.ErrBindVariablesOwner) {
		t.Fatal("closed binder accepted", err)
	}
}

func TestVariableBinderWithoutVariablesReturnsOwnedIdentity(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	manager, err := context.Manager()
	ruleMust(t, err)
	truth, err := manager.MkTrue()
	ruleMust(t, err)
	truthRef, err := manager.NewRef(truth)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, manager.DelRef(&truthRef)) })
	binder, err := base.NewVariableBinder(manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, binder.Close()) })
	result, err := binder.Bind(truth, true)
	ruleMust(t, err)
	defer func() { ruleMust(t, manager.DelRef(&result)) }()
	value, err := manager.RefGet(result)
	ruleMust(t, err)
	if value != truth {
		t.Fatal("empty binder changed expression", value, truth)
	}
}

func TestVariableBinderUsesExpressionOrderAndSkipsUnused(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	defer func() { ruleMust(t, c.Close()) }()
	m, err := c.Manager()
	ruleMust(t, err)
	sort, err := m.MkBoolSort()
	ruleMust(t, err)
	makeConst := func(name string) ast.ASTHandle {
		sym, e := m.Symbols.Intern(name)
		ruleMust(t, e)
		d, e := m.MkFuncDecl(sym, nil, sort, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		a, e := m.MkApp(d, nil)
		ruleMust(t, e)
		return a
	}
	x, y, unused := makeConst("used-x"), makeConst("used-y"), makeConst("unused")
	sym, err := m.Symbols.Intern("used-p")
	ruleMust(t, err)
	decl, err := m.MkFuncDecl(sym, []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	body, err := m.MkApp(decl, []ast.ASTHandle{y, x})
	ruleMust(t, err)
	pin, err := m.NewRef(body)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&pin)) }()
	b, err := base.NewVariableBinder(m)
	ruleMust(t, err)
	defer func() { ruleMust(t, b.Close()) }()
	ruleMust(t, b.AddVariable(x))
	ruleMust(t, b.AddVariable(y))
	ruleMust(t, b.AddVariable(unused))
	r, err := b.Bind(body, true)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&r)) }()
	q, err := m.RefGet(r)
	ruleMust(t, err)
	n, err := m.GetNumDeclarations(q)
	ruleMust(t, err)
	if n != 2 {
		t.Fatal("bound variables", n)
	}
	for i, want := range []string{"used-y", "used-x"} {
		name, e := m.GetDeclarationName(q, uint32(i))
		ruleMust(t, e)
		got, e := m.Symbols.String(name)
		ruleMust(t, e)
		if got != want {
			t.Fatalf("name %d: %s want %s", i, got, want)
		}
	}
	truth, err := m.MkTrue()
	ruleMust(t, err)
	identity, err := b.Bind(truth, true)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&identity)) }()
	got, err := m.RefGet(identity)
	ruleMust(t, err)
	if got != truth {
		t.Fatal("unused variables changed formula")
	}
}

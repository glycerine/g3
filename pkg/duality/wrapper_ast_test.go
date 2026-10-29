package duality

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func closeAST(t *testing.T, value *AST) {
	t.Helper()
	t.Cleanup(func() {
		if !value.closed {
			if err := value.Close(); err != nil {
				t.Error(err)
			}
		}
	})
}

func closeSort(t *testing.T, value *Sort) {
	t.Helper()
	t.Cleanup(func() {
		if value != nil && value.value != nil && !value.value.closed {
			if err := value.Close(); err != nil {
				t.Error(err)
			}
		}
	})
}

func closeDecl(t *testing.T, value *FuncDecl) {
	t.Helper()
	t.Cleanup(func() {
		if value != nil && value.value != nil && !value.value.closed {
			if err := value.Close(); err != nil {
				t.Error(err)
			}
		}
	})
}

func closeExpr(t *testing.T, value *Expr) {
	t.Helper()
	t.Cleanup(func() {
		if value != nil && value.value != nil && !value.value.closed {
			if err := value.Close(); err != nil {
				t.Error(err)
			}
		}
	})
}

func TestWrapperASTReferenceCopyAssignmentAndClose(t *testing.T) {
	context, _, manager := dualityContextFixture(t)
	handle, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	before, err := manager.GetRefCount(handle)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewAST(context, handle)
	if err != nil {
		t.Fatal(err)
	}
	closeAST(t, value)
	afterNew, err := manager.GetRefCount(handle)
	if err != nil || afterNew != before+1 {
		t.Fatal("AST constructor reference", before, afterNew, err)
	}
	copyValue, err := value.Clone()
	if err != nil {
		t.Fatal(err)
	}
	closeAST(t, copyValue)
	afterCopy, err := manager.GetRefCount(handle)
	if err != nil || afterCopy != afterNew+1 {
		t.Fatal("AST copy reference", afterNew, afterCopy, err)
	}
	if err := value.Assign(value); err != nil {
		t.Fatal("self assignment", err)
	}
	if got, err := manager.GetRefCount(handle); err != nil || got != afterCopy {
		t.Fatal("self assignment reference", got, err)
	}
	equal, err := value.Equal(copyValue)
	if err != nil || !equal {
		t.Fatal("copied identity", equal, err)
	}
	less, err := value.Less(copyValue)
	if err != nil || less {
		t.Fatal("equal AST ordering", less, err)
	}
	if err := copyValue.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := manager.GetRefCount(handle); err != nil || got != afterNew {
		t.Fatal("copy destructor reference", got, err)
	}
	if err := value.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := manager.GetRefCount(handle); err != nil || got != before {
		t.Fatal("AST destructor reference", got, err)
	}
	if _, err := value.Raw(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("closed AST", err)
	}
}

func TestWrapperASTNullAndCrossContextAssignment(t *testing.T) {
	context, config, manager := dualityContextFixture(t)
	null, err := NewAST(context, ast.ASTHandle{})
	if err != nil {
		t.Fatal(err)
	}
	closeAST(t, null)
	if yes, err := null.Null(); err != nil || !yes {
		t.Fatal("null AST", yes, err)
	}
	otherContext, err := NewContext(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	defer otherContext.Close()
	handle, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewAST(otherContext, handle)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := null.Assign(other); !errors.Is(err, ErrContextMismatch) {
		t.Fatal("cross-context AST assignment", err)
	}
	if yes, err := null.Null(); err != nil || !yes {
		t.Fatal("rejected assignment mutated receiver", yes, err)
	}
}

func TestWrapperBooleanSortDeclarationAndExpression(t *testing.T) {
	context, _, manager := dualityContextFixture(t)
	boolean, err := context.BoolSort()
	if err != nil {
		t.Fatal(err)
	}
	closeSort(t, boolean)
	if kind, err := boolean.Kind(); err != nil || kind != SortBool {
		t.Fatal("Boolean sort kind", kind, err)
	}
	if yes, err := boolean.IsBool(); err != nil || !yes {
		t.Fatal("Boolean sort predicate", yes, err)
	}
	name, err := context.StringSymbol("pair")
	if err != nil {
		t.Fatal(err)
	}
	decl, err := context.Function(name, []*Sort{boolean, boolean}, boolean)
	if err != nil {
		t.Fatal(err)
	}
	closeDecl(t, decl)
	if arity, err := decl.Arity(); err != nil || arity != 2 {
		t.Fatal("arity", arity, err)
	}
	for i := uint32(0); i < 2; i++ {
		domain, err := decl.Domain(i)
		if err != nil {
			t.Fatal(err)
		}
		if yes, err := domain.IsBool(); err != nil || !yes {
			t.Fatal("domain", i, yes, err)
		}
		if err := domain.Close(); err != nil {
			t.Fatal(err)
		}
	}
	rangeSort, err := decl.Range()
	if err != nil {
		t.Fatal(err)
	}
	if yes, err := rangeSort.IsBool(); err != nil || !yes {
		t.Fatal("range", yes, err)
	}
	if err := rangeSort.Close(); err != nil {
		t.Fatal(err)
	}
	declName, err := decl.Name()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := declName.String(); err != nil || got != "pair" {
		t.Fatal("declaration name", got, err)
	}
	if constant, err := decl.IsConst(); err != nil || constant {
		t.Fatal("nonconstant declaration", constant, err)
	}

	x, err := context.BoolConst("x")
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, x)
	y, err := context.BoolConst("y")
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, y)
	app, err := decl.Call(x, y)
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, app)
	if n, err := app.NumArgs(); err != nil || n != 2 {
		t.Fatal("application args", n, err)
	}
	for i, want := range []*Expr{x, y} {
		arg, err := app.Arg(uint32(i))
		if err != nil {
			t.Fatal(err)
		}
		gotHandle, _ := arg.Raw()
		wantHandle, _ := want.Raw()
		if gotHandle != wantHandle {
			t.Fatal("argument", i, gotHandle, wantHandle)
		}
		if err := arg.Close(); err != nil {
			t.Fatal(err)
		}
	}
	appDecl, err := app.Decl()
	if err != nil {
		t.Fatal(err)
	}
	appDeclHandle, _ := appDecl.Raw()
	declHandle, _ := decl.Raw()
	if appDeclHandle != declHandle {
		t.Fatal("application declaration", appDeclHandle, declHandle)
	}
	if err := appDecl.Close(); err != nil {
		t.Fatal(err)
	}
	if yes, err := app.IsBool(); err != nil || !yes {
		t.Fatal("application sort", yes, err)
	}
	if yes, err := app.IsGround(); err != nil || !yes {
		t.Fatal("ground application", yes, err)
	}
	if yes, err := app.HasQuantifiers(); err != nil || yes {
		t.Fatal("quantifier flag", yes, err)
	}

	// Typed constructors reject the adjacent wrapper families.
	appHandle, _ := app.Raw()
	if _, err := NewSort(context, appHandle); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("expression accepted as sort", err)
	}
	sortHandle, _ := boolean.Raw()
	if _, err := NewExpr(context, sortHandle); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("sort accepted as expression", err)
	}
	if _, err := NewFuncDecl(context, sortHandle); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("sort accepted as declaration", err)
	}
	if _, err := manager.GetKind(appHandle); err != nil {
		t.Fatal("typed rejection damaged AST", err)
	}
}

func TestWrapperBooleanOperatorsAndContextGuards(t *testing.T) {
	context, config, manager := dualityContextFixture(t)
	a, err := context.BoolConst("a")
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, a)
	b, err := context.BoolConst("b")
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, b)
	tt, err := context.BoolVal(true)
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, tt)
	ff, err := context.BoolVal(false)
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, ff)
	checks := []struct {
		kind int32
		make func() (*Expr, error)
	}{
		{ast.ASTOpNot, a.Not},
		{ast.ASTOpAnd, func() (*Expr, error) { return a.And(b) }},
		{ast.ASTOpOr, func() (*Expr, error) { return a.Or(b) }},
		{ast.ASTOpImplies, func() (*Expr, error) { return a.Implies(b) }},
		{ast.ASTOpIff, func() (*Expr, error) { return a.Eq(b) }},
		{ast.ASTOpDistinct, func() (*Expr, error) { return a.Distinct(b) }},
	}
	for _, check := range checks {
		result, err := check.make()
		if err != nil {
			t.Fatal(check.kind, err)
		}
		handle, _ := result.Raw()
		decl, err := manager.GetDecl(handle)
		if err != nil {
			t.Fatal(err)
		}
		kind, err := manager.GetDeclKind(decl)
		if err != nil || kind != check.kind {
			t.Fatal("operator kind", check.kind, kind, err)
		}
		if err := result.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for expr, want := range map[*Expr]int32{tt: ast.ASTOpTrue, ff: ast.ASTOpFalse} {
		handle, _ := expr.Raw()
		decl, err := manager.GetDecl(handle)
		if err != nil {
			t.Fatal(err)
		}
		kind, err := manager.GetDeclKind(decl)
		if err != nil || kind != want {
			t.Fatal("Boolean value", want, kind, err)
		}
	}
	otherContext, err := NewContext(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	defer otherContext.Close()
	foreign, err := otherContext.BoolConst("foreign")
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.Close()
	if _, err := a.And(foreign); !errors.Is(err, ErrContextMismatch) {
		t.Fatal("cross-context expression", err)
	}
	if _, err := a.Eq(foreign); !errors.Is(err, ErrContextMismatch) {
		t.Fatal("cross-context equality", err)
	}
}

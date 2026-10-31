package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestAPIASTSymbolConstructorsPreserveNumericalNullAndTextNames(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	text, err := f.c.StringSymbol("a b")
	astTestMust(t, err)
	for _, name := range []usymbol.Symbol{usymbol.Numerical(37), {}, text} {
		sort := f.pin(f.c.UninterpretedSortWithSymbol(name))
		got, err := f.c.SortSymbol(sort)
		if err != nil || got != name {
			t.Fatal("sort symbol", got, name, err)
		}
		declaration := f.pin(f.c.FunctionWithSymbol(name, []ast.ASTHandle{sort}, sort))
		got, err = f.c.DeclarationSymbol(declaration)
		if err != nil || got != name {
			t.Fatal("function symbol", got, name, err)
		}
		constant := f.pin(f.c.ConstantWithSymbol(name, sort))
		constantDecl := f.pin(f.c.ApplicationDeclaration(constant))
		got, err = f.c.DeclarationSymbol(constantDecl)
		if err != nil || got != name {
			t.Fatal("constant symbol", got, name, err)
		}
		parameter, err := f.m.NewParameterSymbol(name)
		astTestMust(t, err)
		info, err := f.m.NewFuncDeclInfo(ast.NullFamilyID, 991, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
		astTestMust(t, err)
		parameterized := f.pin(f.m.MkFuncDeclWithInfo(text, nil, sort, info))
		astTestMust(t, f.m.DelParameter(&parameter))
		astTestMust(t, f.m.DelFuncDeclInfo(&info))
		got, err = f.c.DeclarationParameterSymbol(parameterized, 0)
		if err != nil || got != name {
			t.Fatal("parameter symbol", got, name, err)
		}
	}
}

func TestAPISymbolConstructorsAndParametersRejectInvalidOwnersAndKinds(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	other := newSMTAPIFixture(t, "ALL", nil)
	foreign, err := other.c.StringSymbol("foreign-name")
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	if _, err := f.c.UninterpretedSortWithSymbol(foreign); err == nil {
		t.Fatal("foreign sort name accepted")
	}
	if _, err := f.c.FunctionWithSymbol(foreign, nil, boolean); err == nil {
		t.Fatal("foreign function name accepted")
	}
	if _, err := f.c.ConstantWithSymbol(foreign, boolean); err == nil {
		t.Fatal("foreign constant name accepted")
	}
	name, err := f.c.StringSymbol("owned-name")
	astTestMust(t, err)
	foreignSort := other.pin(other.c.BoolSort())
	if _, err := f.c.ConstantWithSymbol(name, foreignSort); err == nil {
		t.Fatal("foreign sort accepted")
	}
	integer, err := f.m.NewParameterInt(3)
	astTestMust(t, err)
	info, err := f.m.NewFuncDeclInfo(ast.NullFamilyID, 912, []ast.ASTParameter{integer}, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	declaration := f.pin(f.m.MkFuncDeclWithInfo(name, nil, boolean, info))
	astTestMust(t, f.m.DelFuncDeclInfo(&info))
	astTestMust(t, f.m.DelParameter(&integer))
	for _, index := range []uint32{0, 1} {
		if _, err := f.c.DeclarationParameterSymbol(declaration, index); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("parameter type/bounds", index, err)
		}
	}
	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err := closed.UninterpretedSortWithSymbol(name); err == nil {
		t.Fatal("closed context accepted sort")
	}
	if _, err := closed.FunctionWithSymbol(name, nil, boolean); err == nil {
		t.Fatal("closed context accepted function")
	}
	if _, err := closed.ConstantWithSymbol(name, boolean); err == nil {
		t.Fatal("closed context accepted constant")
	}
	if _, err := closed.DeclarationParameterSymbol(declaration, 0); err == nil {
		t.Fatal("closed context accepted parameter")
	}
}

func TestAPIASTSymbolEmptyAndCStringBehavior(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	for _, spelling := range []string{"", "\x00ignored"} {
		symbol, err := f.c.StringSymbol(spelling)
		astTestMust(t, err)
		if symbol != (usymbol.Symbol{}) {
			t.Fatal("empty symbol must be null", symbol)
		}
		if text, err := f.c.SymbolString(symbol); err != nil || text != "" {
			t.Fatal("null spelling", text, err)
		}
	}
	truncated, err := f.c.StringSymbol("prefix\x00ignored")
	astTestMust(t, err)
	if text, err := f.c.SymbolString(truncated); err != nil || text != "prefix" {
		t.Fatal("C string spelling", text, err)
	}
	sort := f.pin(f.c.UninterpretedSort(""))
	if name, err := f.c.SortSymbol(sort); err != nil || name != (usymbol.Symbol{}) {
		t.Fatal("empty sort name", name, err)
	}
	boolean := f.pin(f.c.BoolSort())
	declaration := f.pin(f.c.Function("", nil, boolean))
	if name, err := f.c.DeclarationSymbol(declaration); err != nil || name != (usymbol.Symbol{}) {
		t.Fatal("empty declaration name", name, err)
	}
	constant := f.pin(f.c.Constant("", boolean))
	if got := f.pin(f.c.ApplicationDeclaration(constant)); got != declaration {
		t.Fatal("empty constant name", got, declaration)
	}
}

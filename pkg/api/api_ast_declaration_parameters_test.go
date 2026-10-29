package api

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextASTDeclarationParameterKindsAndValues(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	expression := f.pin(f.c.Bool(true))
	nested := f.pin(f.c.Function("parameter-nested", nil, boolean))
	integer, err := f.m.NewParameterInt(-19)
	astTestMust(t, err)
	double, err := f.m.NewParameterDouble(1.25)
	astTestMust(t, err)
	rational, err := f.m.NewParameterRat(big.NewRat(-7, 3))
	astTestMust(t, err)
	symbol, err := f.m.NewParameterString("parameter-symbol")
	astTestMust(t, err)
	sortValue, err := f.m.NewParameterAST(boolean)
	astTestMust(t, err)
	expressionValue, err := f.m.NewParameterAST(expression)
	astTestMust(t, err)
	functionValue, err := f.m.NewParameterAST(nested)
	astTestMust(t, err)
	parameters := []ast.ASTParameter{integer, double, rational, symbol, sortValue, expressionValue, functionValue}
	defer func() {
		for i := range parameters {
			astTestMust(t, f.m.DelParameter(&parameters[i]))
		}
	}()
	info, err := f.m.NewFuncDeclInfo(ast.NullFamilyID, 707, parameters, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	name, err := f.m.Symbols.Intern("parameterized-declaration")
	astTestMust(t, err)
	declaration := f.pin(f.m.MkFuncDeclWithInfo(name, nil, boolean, info))
	astTestMust(t, f.m.DelFuncDeclInfo(&info))

	if count, e := f.c.DeclarationParameterCount(declaration); e != nil || count != uint32(len(parameters)) {
		t.Fatal("parameter count", count, e)
	}
	wantKinds := []APIDeclarationParameterKind{APIDeclarationParameterInteger, APIDeclarationParameterDouble, APIDeclarationParameterRational, APIDeclarationParameterSymbol, APIDeclarationParameterSort, APIDeclarationParameterAST, APIDeclarationParameterFunctionDeclaration}
	for i, want := range wantKinds {
		if got, e := f.c.DeclarationParameterKind(declaration, uint32(i)); e != nil || got != want {
			t.Fatal("parameter kind", i, got, want, e)
		}
	}
	if got, e := f.c.DeclarationIntegerParameter(declaration, 0); e != nil || got != -19 {
		t.Fatal("integer parameter", got, e)
	}
	if got, e := f.c.DeclarationDoubleParameter(declaration, 1); e != nil || got != 1.25 {
		t.Fatal("double parameter", got, e)
	}
	if got, e := f.c.DeclarationRationalParameter(declaration, 2); e != nil || got != "-7/3" {
		t.Fatal("rational parameter", got, e)
	}
	if got, e := f.c.DeclarationSymbolParameter(declaration, 3); e != nil || got != "parameter-symbol" {
		t.Fatal("symbol parameter", got, e)
	}
	if got, e := f.c.DeclarationSortParameter(declaration, 4); e != nil || got != boolean {
		t.Fatal("sort parameter", got, e)
	}
	if got, e := f.c.DeclarationASTParameter(declaration, 5); e != nil || got != expression {
		t.Fatal("AST parameter", got, e)
	}
	if got, e := f.c.DeclarationFunctionParameter(declaration, 6); e != nil || got != nested {
		t.Fatal("function parameter", got, e)
	}
}

func TestSMTContextASTDeclarationParameterErrors(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	parameter, err := f.m.NewParameterAST(boolean)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelParameter(&parameter)) }()
	info, err := f.m.NewFuncDeclInfo(ast.NullFamilyID, 708, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	name, err := f.m.Symbols.Intern("parameter-errors")
	astTestMust(t, err)
	declaration := f.pin(f.m.MkFuncDeclWithInfo(name, nil, boolean, info))
	astTestMust(t, f.m.DelFuncDeclInfo(&info))

	if _, err = f.c.DeclarationParameterKind(declaration, 1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("parameter bounds", err)
	}
	if _, err = f.c.DeclarationIntegerParameter(declaration, 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("typed parameter mismatch", err)
	}
	if _, err = f.c.DeclarationFunctionParameter(declaration, 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("AST subtype mismatch", err)
	}
}

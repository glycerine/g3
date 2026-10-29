package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func hnfNestedQuantifier(t *testing.T, f *ruleSetFixture, forall bool, body ast.ASTHandle, name string) ast.ASTHandle {
	t.Helper()
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	nameSymbol, err := f.manager.Symbols.Intern(name)
	ruleMust(t, err)
	result, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{nameSymbol}, Body: body})
	ruleMust(t, err)
	return result
}

func hnfNormalizeNested(t *testing.T, f *ruleSetFixture, formula ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	ref, err := f.manager.NewRef(formula)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, f.manager.DelRef(&ref)) })
	normalizer, err := base.NewHornNormalizer(f.manager, "nested")
	ruleMust(t, err)
	result, err := normalizer.Normalize(formula)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, result.Close()) })
	if len(result.Formulas) == 0 {
		t.Fatal("nested normalization produced no formulas")
	}
	value, err := f.manager.RefGet(result.Formulas[len(result.Formulas)-1])
	ruleMust(t, err)
	if kind, err := f.manager.GetKind(value); err != nil || kind != ast.ASTQuantifier {
		t.Fatal("nested quantifier was not rebuilt as a universal prefix", kind, err)
	}
	return value
}

func hnfRequireSinglePrefix(t *testing.T, f *ruleSetFixture, formula ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	if count, err := f.manager.GetNumDeclarations(formula); err != nil || count != 1 {
		t.Fatal("nested prefix declaration count", count, err)
	}
	forall, err := f.manager.IsForall(formula)
	ruleMust(t, err)
	if !forall {
		t.Fatal("nested compatible quantifier was not universally closed")
	}
	body, err := f.manager.GetQuantifierBody(formula)
	ruleMust(t, err)
	has, err := f.manager.HasQuantifiers(body)
	ruleMust(t, err)
	if has {
		t.Fatal("compatible nested quantifier remained below the prefix")
	}
	return body
}

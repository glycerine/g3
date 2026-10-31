package bmc_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBMCCompileQueryNamesAndSorts(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	real, err := m.MkFamilySort(family, 0, nil)
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	for _, tc := range []struct {
		name     symbol.Symbol
		sorts    []ast.ASTHandle
		expected string
	}{
		{symbol.Numerical(42), nil, "k!42"},
		{func() symbol.Symbol { s, err := m.Symbols.Intern("reach"); must(err); return s }(), []ast.ASTHandle{real, integer}, "reach"},
	} {
		decl, err := m.MkFuncDecl(tc.name, tc.sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		declRef, err := m.NewRef(decl)
		must(err)
		ref, err := compiler.CompileQuery(decl, 3)
		must(err)
		query, err := m.RefGet(ref)
		must(err)
		levelDecl, err := m.GetDecl(query)
		must(err)
		name, err := m.GetASTName(levelDecl)
		must(err)
		text, err := m.Symbols.Display(name)
		must(err)
		if text != tc.expected+"#3" {
			t.Fatal("level predicate name", text)
		}
		for i, sort := range tc.sorts {
			arg, err := m.GetArg(query, uint32(i))
			must(err)
			gotSort, err := m.GetSort(arg)
			must(err)
			argDecl, err := m.GetDecl(arg)
			must(err)
			argName, err := m.GetASTName(argDecl)
			must(err)
			argText, err := m.Symbols.Display(argName)
			must(err)
			want := tc.expected + "#3_" + string(rune('0'+i))
			if gotSort != sort || argText != want {
				t.Fatal("query argument", gotSort, sort, argText, want)
			}
		}
		must(m.DelRef(&ref))
		must(m.DelRef(&declRef))
	}
}

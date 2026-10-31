package bmc_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/bmc"
)

func TestBMCBindVariablesCompressesGapsAndBuildsPattern(t *testing.T) {
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
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("bound-head")
	must(err)
	decl, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	v0, err := m.MkVar(0, boolean)
	must(err)
	v2, err := m.MkVar(2, integer)
	must(err)
	head, err := m.MkApp(decl, []ast.ASTHandle{v0, v2})
	must(err)
	headRef, err := m.NewRef(head)
	must(err)
	defer func() { must(m.DelRef(&headRef)) }()
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	ref, err := compiler.BindVariables(head, head)
	must(err)
	defer func() { must(m.DelRef(&ref)) }()
	q, err := m.RefGet(ref)
	must(err)
	if n, err := m.GetNumDeclarations(q); err != nil || n != 2 {
		t.Fatal("bound variables", n, err)
	}
	if n, err := m.GetNumPatterns(q); err != nil || n != 1 {
		t.Fatal("pattern count", n, err)
	}
	if ok, err := m.IsWellSorted(q, nil); err != nil || !ok {
		t.Fatal("bound formula is not well sorted", ok, err)
	}
	body, err := m.GetQuantifierBody(q)
	must(err)
	first, err := m.GetArg(body, 0)
	must(err)
	second, err := m.GetArg(body, 1)
	must(err)
	if index, err := m.GetVarIndex(first); err != nil || index != 0 {
		t.Fatal("first variable", index, err)
	}
	if index, err := m.GetVarIndex(second); err != nil || index != 1 {
		t.Fatal("second variable", index, err)
	}
	pattern, err := m.GetPattern(q, 0)
	must(err)
	matched, err := m.GetArg(pattern, 0)
	must(err)
	if matched != body {
		t.Fatal("pattern head differs", matched, body)
	}
}

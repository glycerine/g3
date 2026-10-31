package bmc_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBMCReplacePredicatesAcrossQuantifiers(t *testing.T) {
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
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("level-p")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rm.RegisterPredicate(predicate))
	v, err := m.MkVar(0, boolean)
	must(err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{v})
	must(err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body})
	must(err)
	qref, err := m.NewRef(q)
	must(err)
	defer func() { must(m.DelRef(&qref)) }()
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	zeroRef, err := compiler.ReplacePredicates(rm, q, 0)
	must(err)
	zero, err := m.RefGet(zeroRef)
	must(err)
	if !m.IsFalse(zero) {
		t.Fatal("level zero quantifier", zero)
	}
	must(m.DelRef(&zeroRef))
	ref, err := compiler.ReplacePredicates(rm, q, 2)
	must(err)
	defer func() { must(m.DelRef(&ref)) }()
	out, err := m.RefGet(ref)
	must(err)
	if kind, err := m.GetKind(out); err != nil || kind != ast.ASTQuantifier {
		t.Fatal("quantifier", kind, err)
	}
	newBody, err := m.GetQuantifierBody(out)
	must(err)
	decl, err := m.GetDecl(newBody)
	must(err)
	levelName, err := m.GetASTName(decl)
	must(err)
	text, err := m.Symbols.Display(levelName)
	must(err)
	if text != "level-p#1" {
		t.Fatal("preceding level", text)
	}
	if n, err := m.GetNumPatterns(out); err != nil || n != 0 {
		t.Fatal("patterns", n, err)
	}
	if n, err := m.GetNumNoPatterns(out); err != nil || n != 1 {
		t.Fatal("no-pattern count", n, err)
	}
	if noPattern, err := m.GetNoPattern(out, 0); err != nil || noPattern != newBody {
		t.Fatal("no-pattern body", noPattern, err)
	}
}

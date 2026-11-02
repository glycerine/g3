package tab_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/tab"
)

func TestClauseFormulaClosesFreeVariablesAndAvoidsNames(t *testing.T) {
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	session, e := cmd_context.NewStandaloneContext()
	must(e)
	defer func() { must(session.Close()) }()
	m, e := session.Manager()
	must(e)
	globals, e := session.GlobalParameters()
	must(e)
	rw, e := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	must(e)
	defer func() { must(rw.Close()) }()
	family, e := m.RegisterArithmeticPlugin()
	must(e)
	integer, e := m.MkFamilySort(family, 1, nil)
	must(e)
	boolean, e := m.MkBoolSort()
	must(e)
	x, e := m.MkVar(0, integer)
	must(e)
	z, e := m.MkVar(2, boolean)
	must(e)
	name, e := m.Symbols.Intern("A")
	must(e)
	headDecl, e := m.MkFuncDecl(name, []ast.ASTHandle{integer, boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(e)
	head, e := m.MkApp(headDecl, []ast.ASTHandle{x, z})
	must(e)
	name, e = m.Symbols.Intern("p")
	must(e)
	predDecl, e := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(e)
	pred, e := m.MkApp(predDecl, []ast.ASTHandle{x})
	must(e)
	truth, e := m.MkTrue()
	must(e)
	clause, e := tab.NewClause(m, rw, head, []ast.ASTHandle{pred}, truth)
	must(e)
	defer func() { must(clause.Close()) }()
	body, e := clause.Body()
	must(e)
	defer func() { must(m.DelRef(&body)) }()
	actual, e := m.RefGet(body)
	must(e)
	if actual != pred {
		t.Fatal("body did not simplify to predicate")
	}
	formula, e := clause.Formula()
	must(e)
	defer func() { must(m.DelRef(&formula)) }()
	quantified, e := m.RefGet(formula)
	must(e)
	forall, e := m.IsForall(quantified)
	must(e)
	count, e := m.GetNumDeclarations(quantified)
	must(e)
	if !forall || count != 3 {
		t.Fatalf("quantifier forall=%v count=%d", forall, count)
	}
	for i, want := range []ast.ASTHandle{boolean, boolean, integer} {
		got, e := m.GetDeclarationSort(quantified, uint32(i))
		must(e)
		if got != want {
			t.Fatalf("declaration sort %d", i)
		}
		sym, e := m.GetDeclarationName(quantified, uint32(i))
		must(e)
		text, e := m.Symbols.String(sym)
		must(e)
		if text != string(rune('C'+i)) {
			t.Fatalf("declaration name %q", text)
		}
	}
	quantifiedBody, e := m.GetQuantifierBody(quantified)
	must(e)
	op, e := m.GetDeclKind(quantifiedBody)
	must(e)
	if op != ast.ASTOpImplies {
		t.Fatalf("formula operator %d", op)
	}
	a, e := m.GetArg(quantifiedBody, 0)
	must(e)
	b, e := m.GetArg(quantifiedBody, 1)
	must(e)
	if a != pred || b != head {
		t.Fatal("wrong clause implication")
	}
}

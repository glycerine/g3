package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

func TestFinalizeUnfoldedClauseEliminatesAndCompactsVariables(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	globals, err := ctx.GlobalParameters()
	must(err)
	rw, err := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	cooperation, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer func() { must(cooperation.Close()) }()
	lite, err := qe.NewLite(m, nil, false, qe.Config{Context: globals.Context, Globals: globals, Cooperation: cooperation})
	must(err)
	defer func() { must(lite.Close()) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	integer, err := m.GetSort(one)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	sym, err := m.Symbols.Intern("finalize-p")
	must(err)
	decl, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	p, err := m.MkApp(decl, []ast.ASTHandle{z})
	must(err)
	condition, err := m.MkEq(x, one)
	must(err)
	headRef, err := m.NewRef(p)
	must(err)
	predRef, err := m.NewRef(p)
	must(err)
	conditionRef, err := m.NewRef(condition)
	must(err)
	u := &unfoldedClause{manager: m, head: headRef, predicates: []ast.ASTRef{predRef}, constraint: conditionRef}
	defer func() { must(u.Close()) }()
	clause, matched, err := finalizeUnfoldedClause(u, rw, lite)
	must(err)
	if !matched || clause == nil {
		t.Fatal("clause unexpectedly rejected")
	}
	defer func() { must(clause.Close()) }()
	head, err := clause.Head()
	must(err)
	arg, err := m.GetArg(head, 0)
	must(err)
	index, err := m.GetVarIndex(arg)
	must(err)
	if index != 0 || clause.NumVars() != 1 {
		t.Fatalf("compacted variable %d count %d", index, clause.NumVars())
	}
	got, err := clause.Constraint()
	must(err)
	if !m.IsTrue(got) {
		t.Fatal("unconstrained variable was not eliminated")
	}
	falsehood, err := m.MkFalse()
	must(err)
	must(m.SetRef(u.constraint, falsehood))
	rejected, matched, err := finalizeUnfoldedClause(u, rw, lite)
	must(err)
	if matched || rejected != nil {
		t.Fatal("false constraint accepted")
	}
}

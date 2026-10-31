package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestGroundClauseSetupSubstitutesVariablesInSourceOrder(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := newTabTestContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	globals, err := ctx.GlobalParameters()
	must(err)
	rw, err := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	z, err := m.MkVar(2, boolean)
	must(err)
	name, err := m.Symbols.Intern("setup-p")
	must(err)
	decl, err := m.MkFuncDecl(name, []ast.ASTHandle{integer, boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	pred, err := m.MkApp(decl, []ast.ASTHandle{x, z})
	must(err)
	truth, err := m.MkTrue()
	must(err)
	clause, err := NewClause(m, rw, pred, []ast.ASTHandle{pred}, truth)
	must(err)
	grounded, err := setupGroundClause(clause)
	must(err)
	must(clause.Close())
	defer func() { must(grounded.Close()) }()
	head, err := m.RefGet(grounded.head)
	must(err)
	body, err := m.RefGet(grounded.predicates[0])
	must(err)
	if head != body {
		t.Fatal("same variable occurrences did not share constants")
	}
	for i, want := range []uint32{0, 2} {
		arg, err := m.GetArg(head, uint32(i))
		must(err)
		decl, err := m.GetDecl(arg)
		must(err)
		sym, err := m.GetASTName(decl)
		must(err)
		if !sym.IsNumerical() || sym.Value != want {
			t.Fatalf("ground symbol %v want %d", sym, want)
		}
	}
	free, err := rewriter.NewExprFreeVars(m)
	must(err)
	must(free.Collect(head))
	if !free.Empty() {
		t.Fatal("head still has free variables")
	}
	condition, err := m.RefGet(grounded.precondition)
	must(err)
	op, err := m.GetDeclKind(condition)
	must(err)
	if op != ast.ASTOpAnd {
		t.Fatalf("precondition operator %d", op)
	}
}

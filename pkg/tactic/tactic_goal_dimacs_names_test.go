package tactic

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestGoalDIMACSIncludesStableSourceNames(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "named-p")
	q := goalTestAtom(t, m, "named-q")
	clause := goalTestApp(t, m, ast.ASTOpOr, p, goalTestApp(t, m, ast.ASTOpNot, q))
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(clause))
	var output bytes.Buffer
	astTestMust(t, g.DisplayDIMACSWithNames(&output, true))
	if got, want := output.String(), "p cnf 2 1\n1 -2 0\nc 1 named-p\nc 2 named-q\n"; got != want {
		t.Fatalf("named DIMACS = %q; want %q", got, want)
	}
}

func TestGoalDIMACSPreservesPositiveNumericAtomIDs(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	makeAtom := func(id uint32) ast.ASTHandle {
		declaration, makeErr := m.MkFuncDecl(usymbol.Numerical(id), nil, boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, makeErr)
		atom, makeErr := m.MkApp(declaration, nil)
		astTestMust(t, makeErr)
		astTestPin(t, m, atom)
		return atom
	}
	seven, eleven := makeAtom(7), makeAtom(11)
	clause := goalTestApp(t, m, ast.ASTOpOr, seven, goalTestApp(t, m, ast.ASTOpNot, eleven))
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(clause))
	var output bytes.Buffer
	astTestMust(t, g.DisplayDIMACSWithNames(&output, true))
	if got, want := output.String(), "p cnf 2 1\n7 -11 0\n"; got != want {
		t.Fatalf("numeric DIMACS = %q; want %q", got, want)
	}
}

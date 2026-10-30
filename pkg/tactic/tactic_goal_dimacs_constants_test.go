package tactic

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGoalDIMACSSourceBooleanConstantEncoding(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "constant-p")
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	clause := goalTestApp(t, m, ast.ASTOpOr, falsity, p, truth)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(clause))
	var output bytes.Buffer
	astTestMust(t, g.DisplayDIMACS(&output))
	if got, want := output.String(), "p cnf 2 1\n1 1 -1 0\n"; got != want {
		t.Fatalf("constant DIMACS = %q; want %q", got, want)
	}
}

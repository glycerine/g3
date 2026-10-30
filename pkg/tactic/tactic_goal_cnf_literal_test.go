package tactic

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGoalCNFRejectsBooleanConnectiveAsLiteral(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "non-cnf-p")
	q := goalTestAtom(t, m, "non-cnf-q")
	conjunction := goalTestApp(t, m, ast.ASTOpAnd, p, q)
	clause := goalTestApp(t, m, ast.ASTOpOr, p, conjunction)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(clause))
	if yes, err := g.IsCNF(); err != nil || yes {
		t.Fatal("nested Boolean connective accepted as literal", yes, err)
	}
}

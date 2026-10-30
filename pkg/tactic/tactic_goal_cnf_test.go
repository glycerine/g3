package tactic

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGoalCNFRecognizesSourceClauseGrammar(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "cnf-p")
	q := goalTestAtom(t, m, "cnf-q")
	notQ := goalTestApp(t, m, ast.ASTOpNot, q)
	clause := goalTestApp(t, m, ast.ASTOpOr, p, notQ)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(clause))
	astTestMust(t, g.Assert(p))
	if yes, err := g.IsCNF(); err != nil || !yes {
		t.Fatal("source CNF rejected", yes, err)
	}
}

package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIGoalDIMACSRejectsNonCNF(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-non-cnf-p", boolean))
	q := f.pin(f.c.Constant("api-non-cnf-q", boolean))
	conjunction := f.pin(f.c.And(p, q))
	clause := f.pin(f.c.Or(p, conjunction))
	astTestMust(t, goal.Assert(clause))
	if text, err := goal.DIMACS(false); text != "" || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-CNF export", text, err)
	}
}

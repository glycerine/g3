package portfolio_test

import (
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"testing"
)

func TestFDTacticSolvesBooleanGoal(t *testing.T) {
	f := enumSolverFixture(t)
	r, e := portfolio.NewFDTactic(f.manager, nil, f.services)
	enumMust(t, e)
	enumMust(t, r.IncRef())
	defer r.DecRef()
	g, e := tactic.NewGoal(f.manager, false, true, false)
	enumMust(t, e)
	defer g.Close()
	var out tactic.TacticResult
	enumMust(t, r.Apply(g, &out))
	defer out.Close()
	if !out.DecidedSat() {
		t.Fatal("not sat")
	}
}

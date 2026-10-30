package portfolio_test

import (
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"testing"
)

func TestFDTacticConstructsSequentialSolverAdapter(t *testing.T) {
	f := enumSolverFixture(t)
	r, e := portfolio.NewFDTactic(f.manager, nil, f.services)
	enumMust(t, e)
	enumMust(t, r.IncRef())
	enumMust(t, r.DecRef())
}

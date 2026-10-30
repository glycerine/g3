package portfolio_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic/portfolio"
)

func TestLazyFDTacticLifecycleBeforeMaterialization(t *testing.T) {
	f := enumSolverFixture(t)
	r, e := portfolio.NewLazyFDTactic(f.manager, nil, f.services)
	enumMust(t, e)
	enumMust(t, r.IncRef())
	enumMust(t, r.DecRef())
}

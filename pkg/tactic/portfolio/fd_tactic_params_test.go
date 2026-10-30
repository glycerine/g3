package portfolio_test

import (
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"testing"
)

func TestFDTacticAcceptsParameterUpdates(t *testing.T) {
	f := enumSolverFixture(t)
	r, e := portfolio.NewFDTactic(f.manager, nil, f.services)
	enumMust(t, e)
	enumMust(t, r.IncRef())
	defer r.DecRef()
	p, e := f.services.Context.NewValue()
	enumMust(t, e)
	defer f.services.Context.Del(&p)
	k := "random_seed"
	enumMust(t, f.services.Context.SetUIntName(&p, &k, 7))
	enumMust(t, r.UpdateParams(&p))
}

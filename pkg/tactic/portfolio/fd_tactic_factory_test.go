package portfolio_test

import (
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"testing"
)

func TestFDTacticFactoryLifecycle(t *testing.T) {
	f := enumSolverFixture(t)
	factory, e := portfolio.NewFDTacticFactory(f.services)
	enumMust(t, e)
	r, e := factory.New(f.manager, nil)
	enumMust(t, e)
	enumMust(t, r.IncRef())
	enumMust(t, r.DecRef())
	enumMust(t, factory.Close())
	if _, e = factory.New(f.manager, nil); e == nil {
		t.Fatal("closed factory")
	}
}

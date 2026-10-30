package sls

import (
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestQFBVSLSFactoryCreatesSourceStrategy(t *testing.T) {
	fixture, manager := slsQFBVFactoryFixture(t)
	factory := NewQFBVSLSFactory(smtcore.SMTSolverConfig{Params: fixture.Context, Globals: fixture.Globals, ModelEvaluator: fixture.ModelEvaluator})
	strategy, err := factory.New(manager, fixture.ModelEvaluator.Params)
	if err != nil {
		t.Fatal(err)
	}
	if err = strategy.IncRef(); err != nil {
		t.Fatal(err)
	}
	defer strategy.DecRef()
	atom := slsBoolConst(t, manager, "qfbv-sls-public-factory")
	slsPin(t, manager, atom)
	goal, err := tactic.NewGoal(manager, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer goal.Close()
	if err = goal.Assert(atom); err != nil {
		t.Fatal(err)
	}
	result := &tactic.TacticResult{}
	defer result.Close()
	if err = strategy.Apply(goal, result); err != nil || len(result.Goals) != 1 {
		t.Fatal("public SLS factory", len(result.Goals), err)
	}
}

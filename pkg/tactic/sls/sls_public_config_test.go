package sls

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestQFBVSLSConfigBindsSolverServices(t *testing.T) {
	fixture, _ := slsQFBVFactoryFixture(t)
	solver := smtcore.SMTSolverConfig{Params: fixture.Context, Globals: fixture.Globals, ModelEvaluator: fixture.ModelEvaluator}
	got, err := QFBVConfigFromSolver(solver)
	if err != nil || got.Context != fixture.Context || got.Globals != fixture.Globals || got.ModelEvaluator.Memory != fixture.ModelEvaluator.Memory {
		t.Fatal(got, err)
	}
	if _, err = QFBVConfigFromSolver(smtcore.SMTSolverConfig{}); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("invalid solver services", err)
	}
}

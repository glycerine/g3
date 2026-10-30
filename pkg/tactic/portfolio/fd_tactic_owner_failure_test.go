package portfolio_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestFDTacticRejectsMissingServices(t *testing.T) {
	f := enumSolverFixture(t)
	if r, e := portfolio.NewFDTactic(f.manager, nil, model.ModelEvaluatorConfig{}); r != nil || !errors.Is(e, params.ErrParamsOwner) {
		t.Fatal(r, e)
	}
}

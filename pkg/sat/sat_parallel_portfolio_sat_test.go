package sat

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATParallelPortfolioChildWinnerPublishesModel(t *testing.T) {
	s := satParallelPortfolioFixture(t, 2)
	v, err := s.MkVar(false, true)
	satOK(t, err)
	satOK(t, s.MkClause([]Literal{MakeLiteral(v, false)}))
	// Existing parent cancellation is not copied into a newly registered source
	// child. The child therefore wins while the owner lane returns undefined.
	s.Limit.Cancel()
	got, err := s.Check(nil)
	s.Limit.ResetCancel()
	satOK(t, err)
	if got != lbool.True {
		t.Fatal("parallel child result", got)
	}
	satParallelAssertModel(t, s, v, lbool.True)
}

// satParallelOwnerModelInstance loads the six-variable instance of AUDIT.md
// sat_03-F2 with random_freq 1.0 and the given random_seed (0 keeps the
// default), and returns the model and decision count of Check(nil).
func satParallelOwnerModelInstance(t *testing.T, s *Solver, seed uint32) ([]lbool.LBool, uint64) {
	t.Helper()
	ctx, p := s.context, s.params
	freq := "random_freq"
	satOK(t, ctx.SetDoubleName(&p, &freq, 1.0))
	if seed != 0 {
		name := "random_seed"
		satOK(t, ctx.SetUIntName(&p, &name, seed))
	}
	satOK(t, s.UpdateParams(&p))
	vars := make([]BoolVar, 6)
	for i := range vars {
		v, err := s.MkVar(false, true)
		satOK(t, err)
		vars[i] = v
	}
	lit := func(i int, sign bool) Literal { return MakeLiteral(vars[i], sign) }
	satOK(t, s.MkClause([]Literal{lit(0, false), lit(1, false), lit(2, false)}))
	satOK(t, s.MkClause([]Literal{lit(3, false), lit(4, false), lit(5, false)}))
	satOK(t, s.MkClause([]Literal{lit(0, true), lit(3, true), lit(1, false)}))
	got, err := s.Check(nil)
	satOK(t, err)
	if got != lbool.True {
		t.Fatal("instance is satisfiable", got)
	}
	model, err := s.AppendModel(nil)
	satOK(t, err)
	return model, uint64(s.decision.decisions)
}

// AUDIT.md sat_03-F2: check_par copies a winning worker's model at once;
// the owner lane then runs to completion and its own model is the one
// reported. Only the statistics come from the winning worker.
func TestSATParallelPortfolioOwnerLaneResultSurvivesChildWinner(t *testing.T) {
	parallel := satParallelPortfolioFixture(t, 2)
	parModel, parDecisions := satParallelOwnerModelInstance(t, parallel, 0)
	owner, _, _ := satSolverCheckFixture(t)
	// 2531011 is the owner's generator state after its one draw for the
	// worker seed; 38 is the seed that draw gives the worker.
	ownerModel, _ := satParallelOwnerModelInstance(t, owner, 2531011)
	worker, _, _ := satSolverCheckFixture(t)
	workerModel, workerDecisions := satParallelOwnerModelInstance(t, worker, 38)
	if fmt.Sprint(ownerModel) == fmt.Sprint(workerModel) {
		t.Fatal("instance does not discriminate the lanes", ownerModel)
	}
	if fmt.Sprint(parModel) != fmt.Sprint(ownerModel) {
		t.Fatal("parallel model is not the owner's", parModel, ownerModel, workerModel)
	}
	if parDecisions != workerDecisions {
		t.Fatal("parallel statistics are not the winning worker's", parDecisions, workerDecisions)
	}
}

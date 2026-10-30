package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATSolverProbSearchDispatchExtractionAndModel(t *testing.T) {
	s, pc, p := satSolverCheckFixture(t)
	key := "prob_search"
	satOK(t, pc.SetBoolName(&p, &key, true))
	satOK(t, s.UpdateParams(&p))
	state, err := s.config.State()
	satOK(t, err)
	if !state.ProbSearch || state.Initialized&satConfigProbSearchMask == 0 {
		t.Fatal("prob config", state)
	}
	for i := 0; i < 4; i++ {
		_, err := s.MkVar(false, true)
		satOK(t, err)
	}
	clauses := [][]Literal{
		{MakeLiteral(0, false)},
		{MakeLiteral(0, true), MakeLiteral(1, false)},
		{MakeLiteral(1, true), MakeLiteral(2, false), MakeLiteral(0, false)},
	}
	for _, c := range clauses {
		satOK(t, s.MkClause(c))
	}
	got, err := s.Check(nil)
	satOK(t, err)
	if got != lbool.True {
		t.Fatal("prob result", got)
	}
	model, err := s.AppendModel(nil)
	satOK(t, err)
	if len(model) != 4 {
		t.Fatal("prob model dropped unconstrained variable", model)
	}
	for _, c := range clauses {
		yes := false
		for _, l := range c {
			yes = yes || model[l.variable()] == lbool.FromBool(!l.sign())
		}
		if !yes {
			t.Fatal("prob model", model, c)
		}
	}
	if current, err := s.sourceModelIsCurrent(); err != nil || !current {
		t.Fatal(current, err)
	}
}

func TestSATSolverProbSearchRejectsAssumptionsBeforeMutation(t *testing.T) {
	s, pc, p := satSolverCheckFixture(t)
	key := "prob_search"
	satOK(t, pc.SetBoolName(&p, &key, true))
	satOK(t, s.UpdateParams(&p))
	v, err := s.MkVar(true, true)
	satOK(t, err)
	got, err := s.Check([]Literal{MakeLiteral(v, false)})
	satOK(t, err)
	if got != lbool.Undef {
		t.Fatal(got)
	}
}

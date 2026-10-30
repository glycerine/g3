package sat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestSATParallelPortfolioRestoresBindingsParametersAndOwners(t *testing.T) {
	s := satParallelPortfolioFixture(t, 4)
	phase := satParallelPhase(t, s)
	_, err := s.Check(nil)
	satOK(t, err)
	if s.conflict.parallelPresent || s.conflict.parallel != nil || len(s.Limit.Children) != 0 || s.runtime.liveSolvers != 1 {
		t.Fatal("parallel teardown", s.conflict.parallelPresent, len(s.Limit.Children), s.runtime.liveSolvers)
	}
	if got := satParallelPhase(t, s); got != phase {
		t.Fatal("parallel phase restoration", got, phase)
	}
}

func TestSATParallelPortfolioConstructionRefusalUnwindsPrefix(t *testing.T) {
	s := satParallelPortfolioFixture(t, 2)
	before := s.budget.Used()
	s.budget.FailAfter(0)
	_, err := s.Check(nil)
	s.budget.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("parallel constructor refusal", err)
	}
	if s.budget.Used() != before || s.conflict.parallelPresent || len(s.Limit.Children) != 0 || s.runtime.liveSolvers != 1 {
		t.Fatal("parallel refusal unwind", before, s.budget.Used(), s.conflict.parallelPresent, len(s.Limit.Children), s.runtime.liveSolvers)
	}
}

package euclid

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestEuclideanResetAllocationRefusalReleasesOldOwner(t *testing.T) {
	m := euclidNumeralManager(t)
	baseline := m.BudgetField.Used()
	s, err := NewSolver(m)
	euclidMust(t, err)
	m.BudgetField.FailAfter(0)
	err = s.Reset()
	m.BudgetField.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || s.setup != nil || s.stop == nil {
		t.Fatalf("reset did not stop after releasing its old owner: %v", err)
	}
	euclidMust(t, s.Close())
	if got := m.BudgetField.Used(); got != baseline {
		t.Fatalf("reset refusal left %d bytes, baseline %d", got, baseline)
	}
}

func TestEuclideanEquationAndSolverOwnersReleaseMemory(t *testing.T) {
	m := euclidNumeralManager(t)
	baseline := m.BudgetField.Used()
	z := m.Integers()
	s, err := NewSolver(m)
	euclidMust(t, err)
	_, err = s.MkVar()
	euclidMust(t, err)
	a, c := numeral.NewInt(2), numeral.NewInt(3)
	euclidMust(t, s.AssertEq([]numeral.Int{a}, []uint32{0}, &c, nullEquationIndex))
	if len(s.setup.equations) != 1 {
		t.Fatal("equation owner was not created")
	}
	euclidMust(t, s.Close())
	euclidMust(t, z.Del(&a))
	euclidMust(t, z.Del(&c))
	if got := m.BudgetField.Used(); got != baseline {
		t.Fatalf("equation/solver teardown left %d bytes, baseline %d", got, baseline)
	}
}

func TestEuclideanResetReplacesOwnerAndReleasesMemory(t *testing.T) {
	m := euclidNumeralManager(t)
	baseline := m.BudgetField.Used()
	s, err := NewSolver(m)
	euclidMust(t, err)
	old := s.setup
	euclidMust(t, s.Reset())
	if s.setup == nil || s.setup == old {
		t.Fatal("reset did not replace its owner")
	}
	euclidMust(t, s.Close())
	if got := m.BudgetField.Used(); got != baseline {
		t.Fatalf("reset/close left %d bytes, baseline %d", got, baseline)
	}
}

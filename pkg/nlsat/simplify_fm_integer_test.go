package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSimplifyFourierMotzkinIntegerStrictBoundsUseUnitShift(t *testing.T) {
	s, lower, upper := fmIntervalFixture(t, true, 1, 3)
	count, supported, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if !supported || count != 1 {
		t.Fatal("integer elimination", count, supported)
	}
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.True {
		t.Fatal("integer check", result)
	}
	loSign, err := s.config.Algebraic.EvalSign(s.config.Polynomial, lower, s.assignment)
	assignmentMust(t, err)
	hiSign, err := s.config.Algebraic.EvalSign(s.config.Polynomial, upper, s.assignment)
	assignmentMust(t, err)
	value, err := s.assignment.Value(0)
	assignmentMust(t, err)
	isInteger, err := s.config.Algebraic.IsInt(value)
	assignmentMust(t, err)
	if !isInteger || loSign <= 0 || hiSign >= 0 {
		t.Fatal("integer patch", isInteger, loSign, hiSign)
	}
}

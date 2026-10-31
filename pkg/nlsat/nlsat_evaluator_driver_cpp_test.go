package nlsat

// Source: src/test/nlsat.cpp tst5.
import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestNLSATSourceEvaluatorCircleInfeasibleIntervals(t *testing.T) {
	e := evaluatorFixture(t)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 2, 0}, evaluatorTerm{1, 0, 2}, evaluatorTerm{-2, 0, 0})
	var atom InequalityAtom
	assignmentMust(t, atom.initialize(AtomGT, []polynomial.Polynomial{p}, []bool{false}, 1, make([]InequalityFactor, 1)))
	atom.boolVar = 1
	evaluatorSetFraction(t, e, 0, 0, 1)
	first, err := e.InfeasibleIntervals(&atom, true)
	assignmentMust(t, err)
	defer func() { assignmentMust(t, e.ism.DecRef(first)) }()
	evaluatorSetFraction(t, e, 1, 0, 1)
	second, err := e.InfeasibleIntervals(&atom, true)
	assignmentMust(t, err)
	defer func() { assignmentMust(t, e.ism.DecRef(second)) }()
	same, err := e.ism.SetEq(first, second)
	assignmentMust(t, err)
	if !same {
		t.Fatal("assignment of the projected variable changed infeasible intervals")
	}
	for x := -6; x <= 6; x++ {
		evaluatorCheckPoint(t, e, first, x, 0, AtomGT, true, x*x-8, sat.MakeLiteral(1, true), &atom, true)
	}
}

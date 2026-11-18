package opt

// Source: src/opt/maxsmt.cpp commit_assignment weight accumulation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

var errMaxSMTAssignment = errors.New("opt: invalid MaxSMT assignment")

// softAssignmentWeight is commit_assignment's rational k: the sum of the
// weights whose assignment holds. The caller owns the result.
func (s *ArithmeticSolver) softAssignmentWeight(objective *arithmeticObjective, assignment []bool) (numeral.Rational, error) {
	if objective == nil || len(assignment) != len(objective.softWeights) {
		return numeral.NewRational(), errMaxSMTAssignment
	}
	o := newRatOps(s.manager)
	weight := o.zero()
	for i, truth := range assignment {
		if truth {
			o.add(&weight, &weight, &objective.softWeights[i])
		}
	}
	return weight, o.err
}

package opt

// Source: src/opt/maxsmt.cpp commit_assignment weight accounting.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

func TestMaxSMTAssignmentWeightUsesSatisfiedEntries(t *testing.T) {
	objective := arithmeticObjective{softWeights: []numeral.Rational{testRat(2, 3), testRat(5, 7), testRat(11, 13)}}
	solver := &ArithmeticSolver{manager: newWeightedSoftFixture(t).manager}
	weight, err := solver.softAssignmentWeight(&objective, []bool{true, false, true})
	if err != nil {
		t.Fatal(err)
	}
	// 2/3 + 11/13 = 59/39
	if !ratIs(&weight, 59, 39) {
		t.Fatal("assignment weight", ratString(&weight))
	}
	if _, err = solver.softAssignmentWeight(&objective, []bool{true}); err == nil {
		t.Fatal("short assignment accepted")
	}
}

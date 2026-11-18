package opt

// Source: src/opt/maxsmt.cpp assignment cost partition invariant.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

func TestPublishedSoftAssignmentPartitionsTotalWeight(t *testing.T) {
	weights := []numeral.Rational{testRat(2, 3), testRat(5, 7), testRat(11, 13)}
	assignment := []bool{true, false, true}
	objective := &arithmeticObjective{softWeights: weights}
	manager := newWeightedSoftFixture(t).manager
	q := manager.NumeralService().Manager
	solver := &ArithmeticSolver{manager: manager}
	satisfied, err := solver.softAssignmentWeight(objective, assignment)
	if err != nil {
		t.Fatal(err)
	}
	cost := testRat(0, 1)
	for i, truth := range assignment {
		if !truth {
			if err = q.Add(&cost, &weights[i], &cost); err != nil {
				t.Fatal(err)
			}
		}
	}
	total, sum := testRat(0, 1), testRat(0, 1)
	for i := range weights {
		if err = q.Add(&total, &weights[i], &total); err != nil {
			t.Fatal(err)
		}
	}
	if err = q.Add(&satisfied, &cost, &sum); err != nil {
		t.Fatal(err)
	}
	if !ratEq(&sum, &total) {
		t.Fatal("assignment partition", ratString(&satisfied), ratString(&cost), ratString(&total))
	}
}

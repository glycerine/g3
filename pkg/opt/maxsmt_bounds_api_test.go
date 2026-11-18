package opt

// Source: src/opt/opt_context.cpp MaxSMT lower/upper result queries.

import (
	"testing"
)

func TestSoftBoundsAndValueReturnDefensiveCopies(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	q := manager.NumeralService().Manager
	solver := &ArithmeticSolver{manager: manager, objectives: []arithmeticObjective{{soft: true}}}
	values := NewObjectiveValues(q, 1)
	values[0].Soft, values[0].SoftAssignment = true, []bool{true}
	values[0].SoftLower, values[0].SoftUpper = testRat(2, 3), testRat(5, 7)
	result := &ArithmeticResult{Values: values, Q: q}
	lower, upper, err := solver.SoftBounds(result, 0)
	if err != nil || !ratIs(&lower, 2, 3) || !ratIs(&upper, 5, 7) {
		t.Fatal(lower, upper, err)
	}
	lower = testRat(99, 1)
	copy, err := solver.SoftValue(result, 0)
	if err != nil || !ratIs(&copy.SoftLower, 2, 3) || !copy.SoftAssignment[0] {
		t.Fatal("soft result alias", copy, err)
	}
	copy.SoftAssignment[0] = false
	if !result.Values[0].SoftAssignment[0] {
		t.Fatal("soft assignment alias")
	}
}

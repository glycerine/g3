package opt

// Source: src/opt/maxsmt.cpp update_assignment/get_assignment.

import (
	"testing"
)

func TestSetSoftObjectiveValueCopiesAssignmentAndExactRationals(t *testing.T) {
	q := newWeightedSoftFixture(t).manager.NumeralService().Manager
	result := &ArithmeticResult{Values: NewObjectiveValues(q, 2), Q: q}
	cost, satisfied := testRat(7, 11), testRat(13, 17)
	assignment := []bool{true, false, true}
	if err := setSoftObjectiveValue(result, 1, &cost, &satisfied, assignment); err != nil {
		t.Fatal(err)
	}
	assignment[0] = false
	cost, satisfied = testRat(99, 1), testRat(99, 1)
	value := &result.Values[1]
	if !value.Soft || !value.SoftAssignment[0] || !ratIs(&value.SoftCost, 7, 11) || !ratIs(&value.SoftSatisfied, 13, 17) {
		t.Fatal("published soft value", value)
	}
}

func TestSoftCandidateUpdatePreservesPublishedBounds(t *testing.T) {
	q := newWeightedSoftFixture(t).manager.NumeralService().Manager
	result := &ArithmeticResult{Values: NewObjectiveValues(q, 1), Q: q}
	value := &result.Values[0]
	value.SoftLower, value.SoftUpper = testRat(1, 2), testRat(5, 2)
	value.SoftBoundsValid = true
	if err := setSoftObjectiveValue(result, 0, testRatPtr(3, 2), testRatPtr(4, 1), []bool{true}); err != nil {
		t.Fatal(err)
	}
	if !value.SoftBoundsValid || !ratIs(&value.SoftLower, 1, 2) || !ratIs(&value.SoftUpper, 5, 2) {
		t.Fatal("candidate update discarded search bounds", value)
	}
}

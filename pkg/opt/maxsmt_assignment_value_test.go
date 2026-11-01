package opt

// Source: src/opt/maxsmt.cpp update_assignment/get_assignment.

import (
	"math/big"
	"testing"
)

func TestSetSoftObjectiveValueCopiesAssignmentAndExactRationals(t *testing.T) {
	result := &ArithmeticResult{Values: make([]ObjectiveValue, 2)}
	cost, satisfied := *big.NewRat(7, 11), *big.NewRat(13, 17)
	assignment := []bool{true, false, true}
	setSoftObjectiveValue(result, 1, cost, satisfied, assignment)
	assignment[0] = false
	cost.SetInt64(99)
	satisfied.SetInt64(99)
	value := &result.Values[1]
	if !value.Soft || !value.SoftAssignment[0] || value.SoftCost.Cmp(big.NewRat(7, 11)) != 0 || value.SoftSatisfied.Cmp(big.NewRat(13, 17)) != 0 {
		t.Fatal("published soft value", value)
	}
}

func TestSoftCandidateUpdatePreservesPublishedBounds(t *testing.T) {
	result := &ArithmeticResult{Values: make([]ObjectiveValue, 1)}
	value := &result.Values[0]
	value.SoftLower.SetFrac64(1, 2)
	value.SoftUpper.SetFrac64(5, 2)
	value.SoftBoundsValid = true
	setSoftObjectiveValue(result, 0, *big.NewRat(3, 2), *big.NewRat(4, 1), []bool{true})
	if !value.SoftBoundsValid || value.SoftLower.Cmp(big.NewRat(1, 2)) != 0 || value.SoftUpper.Cmp(big.NewRat(5, 2)) != 0 {
		t.Fatal("candidate update discarded search bounds", value)
	}
}

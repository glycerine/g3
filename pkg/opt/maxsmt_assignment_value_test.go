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
	value := &result.Values[1]
	if !value.Soft || !value.SoftAssignment[0] || value.SoftCost.Cmp(big.NewRat(7, 11)) != 0 || value.SoftSatisfied.Cmp(big.NewRat(13, 17)) != 0 {
		t.Fatal("published soft value", value)
	}
}

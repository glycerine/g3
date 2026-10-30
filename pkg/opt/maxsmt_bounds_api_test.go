package opt

// Source: src/opt/opt_context.cpp MaxSMT lower/upper result queries.

import (
	"math/big"
	"testing"
)

func TestSoftBoundsAndValueReturnDefensiveCopies(t *testing.T) {
	solver := &ArithmeticSolver{objectives: []arithmeticObjective{{soft: true}}}
	value := ObjectiveValue{Soft: true, SoftAssignment: []bool{true}}
	value.SoftLower.SetFrac64(2, 3)
	value.SoftUpper.SetFrac64(5, 7)
	result := &ArithmeticResult{Values: []ObjectiveValue{value}}
	lower, upper, err := solver.SoftBounds(result, 0)
	if err != nil || lower.Cmp(big.NewRat(2, 3)) != 0 || upper.Cmp(big.NewRat(5, 7)) != 0 {
		t.Fatal(lower, upper, err)
	}
	lower.SetInt64(99)
	copy, err := solver.SoftValue(result, 0)
	if err != nil || copy.SoftLower.Cmp(big.NewRat(2, 3)) != 0 || !copy.SoftAssignment[0] {
		t.Fatal("soft result alias", copy, err)
	}
	copy.SoftAssignment[0] = false
	if !result.Values[0].SoftAssignment[0] {
		t.Fatal("soft assignment alias")
	}
}

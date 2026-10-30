package opt

// Source: src/opt/maxsmt.cpp commit_assignment weight accounting.

import (
	"math/big"
	"testing"
)

func TestMaxSMTAssignmentWeightUsesSatisfiedEntries(t *testing.T) {
	objective := arithmeticObjective{softWeights: []*big.Rat{big.NewRat(2, 3), big.NewRat(5, 7), big.NewRat(11, 13)}}
	weight, err := softAssignmentWeight(&objective, []bool{true, false, true})
	if err != nil {
		t.Fatal(err)
	}
	want := new(big.Rat).Add(big.NewRat(2, 3), big.NewRat(11, 13))
	if weight.Cmp(want) != 0 {
		t.Fatal("assignment weight", weight, want)
	}
	if _, err = softAssignmentWeight(&objective, []bool{true}); err == nil {
		t.Fatal("short assignment accepted")
	}
}

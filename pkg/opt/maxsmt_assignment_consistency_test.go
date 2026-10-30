package opt

// Source: src/opt/maxsmt.cpp assignment cost partition invariant.

import (
	"math/big"
	"testing"
)

func TestPublishedSoftAssignmentPartitionsTotalWeight(t *testing.T) {
	weights := []*big.Rat{big.NewRat(2, 3), big.NewRat(5, 7), big.NewRat(11, 13)}
	assignment := []bool{true, false, true}
	objective := &arithmeticObjective{softWeights: weights}
	satisfied, err := softAssignmentWeight(objective, assignment)
	if err != nil {
		t.Fatal(err)
	}
	cost := new(big.Rat)
	for i, truth := range assignment {
		if !truth {
			cost.Add(cost, weights[i])
		}
	}
	total := new(big.Rat)
	for _, weight := range weights {
		total.Add(total, weight)
	}
	if new(big.Rat).Add(satisfied, cost).Cmp(total) != 0 {
		t.Fatal("assignment partition", satisfied, cost, total)
	}
}

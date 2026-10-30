package opt

// Source: src/opt/maxsmt.cpp commit_assignment consistency.

import (
	"math/big"
	"testing"
)

func TestValidateSoftCommitMatchesSatisfiedAssignmentWeight(t *testing.T) {
	solver := &ArithmeticSolver{objectives: []arithmeticObjective{{soft: true, softWeights: []*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}}}}
	value := ObjectiveValue{Soft: true, SoftAssignment: []bool{true, false}}
	value.SoftSatisfied.SetInt64(2)
	if err := solver.validateSoftCommit(0, value); err != nil {
		t.Fatal(err)
	}
	value.SoftSatisfied.SetInt64(3)
	if err := solver.validateSoftCommit(0, value); err != errMaxSMTAssignment {
		t.Fatal("inconsistent commit", err)
	}
}

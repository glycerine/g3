package opt

// Source: src/opt/maxsmt.h get_assignment.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSoftAssignmentValidatesObjectiveAndReturnsDefensiveCopy(t *testing.T) {
	solver := &ArithmeticSolver{objectives: []arithmeticObjective{{soft: true, softTerms: make([]ast.ASTRef, 2), softWeights: []*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}}}}
	result := &ArithmeticResult{Values: []ObjectiveValue{{Soft: true, SoftAssignment: []bool{true, false}}}}
	assignment, err := solver.SoftAssignment(result, 0)
	if err != nil || len(assignment) != 2 || !assignment[0] || assignment[1] {
		t.Fatal(assignment, err)
	}
	assignment[0] = false
	again, err := solver.SoftAssignment(result, 0)
	if err != nil || !again[0] {
		t.Fatal("assignment alias", again, err)
	}
	result.Values[0].SoftAssignment = result.Values[0].SoftAssignment[:1]
	if _, err = solver.SoftAssignment(result, 0); err == nil {
		t.Fatal("short assignment accepted")
	}
}

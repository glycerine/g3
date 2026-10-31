package smtcore

// Source: src/test/arith_simplifier_plugin.cpp, active integer matrix example.

import (
	"math/big"
	"testing"
)

func TestArithmeticSimplifierSourceIntegerEquations(t *testing.T) {
	rows := [][]*big.Int{smtEquationRow(1, 3, 0, 0), smtEquationRow(1, 0, 3, 1)}
	solver := &smtArithmeticEquationSolver{}
	satisfiable, witness, err := solver.SolveEquations(rows)
	astTestMust(t, err)
	if satisfiable || len(witness) == 0 {
		t.Fatal("expected integer contradiction", satisfiable, witness)
	}
	valid, err := smtIntegerRowGCDTest(witness)
	astTestMust(t, err)
	if valid {
		t.Fatal("contradiction witness passes the GCD test", witness)
	}
}

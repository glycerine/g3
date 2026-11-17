package smtcore

// Source: src/test/arith_simplifier_plugin.cpp, active integer matrix example.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestArithmeticSimplifierSourceIntegerEquations(t *testing.T) {
	rows := [][]numeral.Rational{smtEquationRow(1, 3, 0, 0), smtEquationRow(1, 0, 3, 1)}
	solver := smtEquationSolver()
	satisfiable, witness, err := solver.SolveEquations(rows)
	astTestMust(t, err)
	if satisfiable || len(witness) == 0 {
		t.Fatal("expected integer contradiction", satisfiable, witness)
	}
	valid, err := solver.gcdTest(witness)
	astTestMust(t, err)
	if valid {
		t.Fatal("contradiction witness passes the GCD test", witness)
	}
}

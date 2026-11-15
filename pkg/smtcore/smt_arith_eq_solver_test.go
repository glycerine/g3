package smtcore

import (
	"math/big"
	"math/rand"
	"testing"
)

func smtEquationRow(values ...int64) []*big.Int {
	row := make([]*big.Int, len(values))
	for index, value := range values {
		row[index] = big.NewInt(value)
	}
	return row
}

func TestSMTArithmeticEquationSolverRetainsKnownSolutions(t *testing.T) {
	random := rand.New(rand.NewSource(17))
	solver := &smtArithmeticEquationSolver{}
	for trial := 0; trial < 200; trial++ {
		variables := []int64{int64(random.Intn(9) - 4), int64(random.Intn(9) - 4), int64(random.Intn(9) - 4)}
		rows := make([][]*big.Int, 1+random.Intn(5))
		for rowIndex := range rows {
			coefficients := []int64{0, int64(random.Intn(11) - 5), int64(random.Intn(11) - 5), int64(random.Intn(11) - 5)}
			for index, variable := range variables {
				coefficients[0] -= coefficients[index+1] * variable
			}
			rows[rowIndex] = smtEquationRow(coefficients...)
		}
		satisfiable, witness, err := solver.SolveEquations(rows)
		astTestMust(t, err)
		if !satisfiable || witness != nil {
			t.Fatal("known integer solution was rejected", trial, variables, rows, witness)
		}
	}
}

func smtEquationRowEqual(row []*big.Int, values ...int64) bool {
	if len(row) != len(values) {
		return false
	}
	for index, value := range values {
		if row[index].Cmp(big.NewInt(value)) != 0 {
			return false
		}
	}
	return true
}

func TestSMTArithmeticEquationSolverSingleEquation(t *testing.T) {
	solver := &smtArithmeticEquationSolver{}
	unsat := smtEquationRow(3, 2, 4)
	_, _, satisfiable, err := solver.SolveEquation(unsat)
	astTestMust(t, err)
	if satisfiable || !smtEquationRowEqual(unsat, 3, 2, 4) {
		t.Fatal("GCD contradiction", satisfiable, unsat)
	}

	fresh := smtEquationRow(2, 4, 6)
	index, auxiliary, satisfiable, err := solver.SolveEquation(fresh)
	astTestMust(t, err)
	if !satisfiable || !auxiliary || index != 1 || !smtEquationRowEqual(fresh, 1, 3, 0) {
		t.Fatal("fresh Omega substitution", index, auxiliary, satisfiable, fresh)
	}

	unit := smtEquationRow(4, 1, -2)
	index, auxiliary, satisfiable, err = solver.SolveEquation(unit)
	astTestMust(t, err)
	if !satisfiable || auxiliary || index != 1 || !smtEquationRowEqual(unit, -4, -1, 2) {
		t.Fatal("unit substitution", index, auxiliary, satisfiable, unit)
	}

	trivial := smtEquationRow(0, 0, 0)
	index, auxiliary, satisfiable, err = solver.SolveEquation(trivial)
	astTestMust(t, err)
	if !satisfiable || auxiliary || index != 2 || !smtEquationRowEqual(trivial, 0, 0, 0) {
		t.Fatal("trivial equation", index, auxiliary, satisfiable, trivial)
	}
}

func TestSMTArithmeticEquationSolverSubstitution(t *testing.T) {
	unit, err := smtIntegerRowSubstitute(smtEquationRow(1, 3, 4), smtEquationRow(2, -1, 5), 1)
	astTestMust(t, err)
	if !smtEquationRowEqual(unit, 7, 0, 19) {
		t.Fatal("unit substitution", unit)
	}
	nonunit, err := smtIntegerRowSubstitute(smtEquationRow(1, 2), smtEquationRow(3, 4, 5), 1)
	astTestMust(t, err)
	if !smtEquationRowEqual(nonunit, 7, 8, 10) {
		t.Fatal("auxiliary substitution", nonunit)
	}
}

func TestSMTArithmeticEquationSolverMatrixGCDElimination(t *testing.T) {
	solver := &smtArithmeticEquationSolver{}
	unsatRows := [][]*big.Int{smtEquationRow(0, 1, 1), smtEquationRow(1, 1, -1)}
	satisfiable, witness, err := solver.SolveEquations(unsatRows)
	astTestMust(t, err)
	if satisfiable || len(witness) == 0 {
		t.Fatal("matrix GCD contradiction was missed", satisfiable, witness, unsatRows)
	}
	valid, err := smtIntegerRowGCDTest(witness)
	astTestMust(t, err)
	if valid {
		t.Fatal("reported contradiction row passes divisibility", witness)
	}

	satRows := [][]*big.Int{smtEquationRow(0, 1, 1), smtEquationRow(2, 1, -1)}
	satisfiable, witness, err = solver.SolveEquations(satRows)
	astTestMust(t, err)
	if !satisfiable || witness != nil {
		t.Fatal("satisfiable matrix rejected", satisfiable, witness, satRows)
	}

	satisfiable, witness, err = solver.SolveEquations(nil)
	astTestMust(t, err)
	if !satisfiable || witness != nil {
		t.Fatal("empty matrix", satisfiable, witness)
	}
}

// smtcore_01-F4: solve_integer_equations_gcd never resets `useful`, and
// rational gcd(r1, r2, u, v) follows mpz_manager::gcd (gcd(0,0) gives u=1,
// v=0). Expected rows are those of the C++ arith_eq_solver.
func TestSMTArithmeticEquationSolverMatchesSourceUsefulAccumulation(t *testing.T) {
	solver := &smtArithmeticEquationSolver{}
	rows := [][]*big.Int{smtEquationRow(1, 4, 1, 3), smtEquationRow(0, 3, 0, 4), smtEquationRow(-2, 2, 0, -1)}
	satisfiable, witness, err := solver.SolveEquations(rows)
	astTestMust(t, err)
	if !satisfiable || witness != nil {
		t.Fatal("source verdict is satisfiable", satisfiable, witness)
	}
	want := [][]int64{{0, 3, 0, 4}, {1, 1, 1, -1}, {-4, 0, -2, 1}}
	for i := range want {
		if !smtEquationRowEqual(rows[i], want[i]...) {
			t.Fatal("row", i, rows[i], want[i])
		}
	}
	rows = [][]*big.Int{smtEquationRow(0, 1, 1), smtEquationRow(2, 1, -1)}
	satisfiable, witness, err = solver.SolveEquations(rows)
	astTestMust(t, err)
	if !satisfiable || witness != nil {
		t.Fatal("source verdict is satisfiable", satisfiable, witness)
	}
	want = [][]int64{{-1, 0, 1}, {1, 0, -1}}
	for i := range want {
		if !smtEquationRowEqual(rows[i], want[i]...) {
			t.Fatal("row", i, rows[i], want[i])
		}
	}
}

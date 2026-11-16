package lp

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLPFloatMPSDualIndependentMinMax(t *testing.T) {
	rng := rand.New(rand.NewSource(796724))
	for world := 0; world < 120; world++ {
		c := []float64{float64(rng.Intn(15) - 7), float64(rng.Intn(15) - 7), 0, 0, 0, 0}
		shift := []float64{-2, 1}
		a := make([][]float64, 4)
		b := make([]float64, 4)
		var input strings.Builder
		input.WriteString("NAME PAIR\nROWS\n N COST\n L R0\n L R1\n L R2\nCOLUMNS\n")
		for i := 0; i < 3; i++ {
			a[i] = []float64{float64(1 + rng.Intn(8)), float64(1 + rng.Intn(8)), 0, 0, 0, 0}
			a[i][2+i] = 1
			b[i] = float64(1 + rng.Intn(24))
		}
		for j, name := range []string{"x", "y"} {
			input.WriteString(lpMPSPair(name, "COST", c[j]))
			for i := 0; i < 3; i++ {
				input.WriteString(lpMPSPair(name, fmt.Sprint("R", i), a[i][j]))
			}
		}
		input.WriteString("RHS\n")
		for i := 0; i < 3; i++ {
			input.WriteString(lpMPSPair("RHS1", fmt.Sprint("R", i), b[i]+float64(a[i][0]*shift[0])+float64(a[i][1]*shift[1])))
		}
		input.WriteString("BOUNDS\n")
		input.WriteString(lpMPSBoundLine("LO", "x -2"))
		input.WriteString(lpMPSBoundLine("LO", "y 1"))
		input.WriteString(lpMPSBoundLine("UP", "x 4"))
		input.WriteString("ENDATA\n")
		a[3] = []float64{1, 0, 0, 0, 0, 1}
		b[3] = 6
		reader := lpMPSRead(t, input.String())
		for _, minimum := range []bool{false, true} {
			objective := append([]float64(nil), c...)
			if minimum {
				for j := range objective {
					objective[j] = -objective[j]
				}
			}
			want := lpVertexMaximum(a, b, objective) + float64(objective[0]*shift[0]) + float64(objective[1]*shift[1])
			if minimum {
				want = -want
			}
			dual, err := reader.CreateDualSolver()
			lpStaticMust(t, err)
			primal, err := reader.CreatePrimalSolver()
			lpStaticMust(t, err)
			dual.Settings.SimplexStrategy = StrategyLU
			dual.Settings.DebugOut = nil
			dual.Settings.MessageOut = nil
			dual.Settings.MaxTotalIterations = 1000
			primal.Settings.SimplexStrategy = StrategyLU
			primal.Settings.DebugOut = nil
			primal.Settings.MessageOut = nil
			primal.Settings.MaxTotalIterations = 1000
			if minimum {
				lpStaticMust(t, dual.FlipCosts())
				lpStaticMust(t, primal.FlipCosts())
			}
			lpStaticMust(t, dual.FindMaximalSolution())
			lpStaticMust(t, primal.FindMaximalSolution())
			dc, err := dual.GetCurrentCost()
			lpStaticMust(t, err)
			pc, err := primal.GetCurrentCost()
			lpStaticMust(t, err)
			if minimum {
				dc = -dc
				pc = -pc
			}
			if dual.Status != StatusOptimal || primal.Status != StatusOptimal || math.Abs(dc-want) > 1e-7 || math.Abs(pc-want) > 1e-7 {
				t.Fatal(world, minimum, dual.Status, primal.Status, dc, pc, want)
			}
			x, err := dual.GetColumnValue(lpTestFloatNamedColumn(t, dual.NamesToColumns, "x"))
			lpStaticMust(t, err)
			y, err := dual.GetColumnValue(lpTestFloatNamedColumn(t, dual.NamesToColumns, "y"))
			lpStaticMust(t, err)
			if x < -2-1e-7 || x > 4+1e-7 || y < 1-1e-7 {
				t.Fatal(world, minimum, x, y)
			}
			for i := range b {
				lhs := float64(a[i][0]*(x+2)) + float64(a[i][1]*(y-1))
				if lhs > b[i]+1e-7 {
					t.Fatal(world, minimum, i, lhs, b[i])
				}
			}
			lpStaticMust(t, dual.Close())
			lpStaticMust(t, primal.Close())
		}
	}
}

func TestLPFloatMPSDualStoredAndPopulationIsolation(t *testing.T) {
	r := OpenFloatMPSReader(filepath.Join("test_vectors", "mps_small.mps"))
	r.Messages = &bytes.Buffer{}
	defer r.Close()
	lpStaticMust(t, r.Read())
	dual, err := r.CreateDualSolver()
	lpStaticMust(t, err)
	defer dual.Close()
	primal, err := r.CreatePrimalSolver()
	lpStaticMust(t, err)
	defer primal.Close()
	if !reflect.DeepEqual(dual.Columns, primal.Columns) || !reflect.DeepEqual(dual.Constraints, primal.Constraints) {
		t.Fatal(dual.Columns, primal.Columns, dual.Constraints, primal.Constraints)
	}
	dual.Settings.SimplexStrategy = StrategyLU
	dual.Settings.DebugOut = nil
	dual.Settings.MessageOut = nil
	lpStaticMust(t, dual.FindMaximalSolution())
	cost, err := dual.GetCurrentCost()
	lpStaticMust(t, err)
	if dual.Status != StatusOptimal || math.Abs(cost-24) > 1e-8 {
		t.Fatal(dual.Status, cost)
	}
	for name, want := range map[string]float64{"x": 2, "y": 4, "f": 2} {
		value, err := dual.GetColumnValue(lpTestFloatNamedColumn(t, dual.NamesToColumns, name))
		lpStaticMust(t, err)
		if math.Abs(value-want) > 1e-8 {
			t.Fatal(name, value, want)
		}
	}
	// Solvers own copied model values. Cost flipping/presolve cannot mutate
	// reader coefficients/bounds, another existing solver, or future creation.
	again, err := r.CreatePrimalSolver()
	lpStaticMust(t, err)
	defer again.Close()
	if !reflect.DeepEqual(primal.Columns, again.Columns) || !reflect.DeepEqual(primal.Constraints, again.Constraints) {
		t.Fatal(primal.Columns, again.Columns)
	}
	if primal.Settings == dual.Settings || primal.Columns[0] == dual.Columns[0] {
		t.Fatal("aliased model owners")
	}
	// Creation has no IsOK gate in source, and does not repair reader status.
	r.ok = false
	partial, err := r.CreateDualSolver()
	lpStaticMust(t, err)
	defer partial.Close()
	if r.IsOK() || !reflect.DeepEqual(partial.Columns, again.Columns) {
		t.Fatal(r.IsOK(), partial.Columns, again.Columns)
	}
}

func TestLPFloatMPSDualPopulationErrorPrefixes(t *testing.T) {
	for _, missing := range []bool{false, true} {
		r := NewFloatMPSReader(nil)
		r.Messages = &bytes.Buffer{}
		row := newFloatMPSRow("R", 7, mpsLess)
		row.coefficients.Set("missing", 3)
		r.rows.Set("R", row)
		if !missing {
			r.columns.Set("missing", nil)
		}
		solver, err := r.CreateDualSolver()
		if solver == nil {
			t.Fatal(err)
		}
		defer solver.Close()
		want := error(ErrBounds)
		if missing && sourceDebugEnabled {
			want = ErrAssertion
		}
		if err != want || len(solver.Constraints) != 1 || solver.Constraints[7].Relation != RelationLessOrEqual {
			t.Fatal(err, want, solver.Constraints)
		}
		_, present := r.columns.Get2("missing")
		if missing && present == sourceDebugEnabled {
			t.Fatal("source operator[] prefix", present)
		}
	}
	r := lpMPSRead(t, "NAME T\nROWS\n N COST\n L R\nCOLUMNS\n"+lpMPSPair("x", "COST", 1)+lpMPSPair("x", "R", 1)+"RHS\n"+lpMPSPair("RHS1", "R", 3)+"ENDATA\n")
	solver, err := r.CreateDualSolver()
	lpStaticMust(t, err)
	defer solver.Close()
	solver.Settings.DebugOut = nil
	solver.Settings.MessageOut = nil
	// Unchanged default tableau profile fails construction with no LU.
	if err := solver.FindMaximalSolution(); err != ErrBounds || solver.CoreSolver != nil {
		t.Fatal(err, solver.CoreSolver)
	}
}

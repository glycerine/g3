package lp

import (
	"bytes"
	"errors"
	"math"
	"math/rand"
	"testing"
)

func lpFloatPrimal(t *testing.T) *FloatPrimalSimplex {
	t.Helper()
	m, err := NewFloatPrimalSimplex()
	if err != nil {
		t.Fatal(err)
	}
	m.Settings.DebugOut = nil
	m.Settings.MessageOut = nil
	return m
}

func TestLPFloatPrimalPreparedBasisIndependentOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(891774))
	for world := 0; world < 120; world++ {
		m := lpFloatPrimal(t)
		nr, nc := 1+rng.Intn(9), 1+rng.Intn(6)
		for j := 0; j < nc; j++ {
			c := m.GetOrCreateColumnInfo(uint32(j))
			c.Cost = float64(j-2) / 4
			switch j % 4 {
			case 0:
				c.SetLowBound(-2)
			case 1:
				c.SetUpperBound(3)
			case 2:
				c.SetLowBound(-2)
				c.SetUpperBound(3)
			}
		}
		for i := 0; i < nr; i++ {
			shift := 0.
			for j := 0; j < nc; j++ {
				v := float64(1+rng.Intn(8)) / 4
				if rng.Intn(2) == 0 {
					v = -v
				}
				m.SetRowColumnCoefficient(uint32(i), uint32(j), v)
				shift += v * m.Columns[uint32(j)].Shift()
			}
			if err := m.AddConstraint(LPRelation(i%3), float64(rng.Intn(11)-5)/2+shift, uint32(i)); err != nil {
				t.Fatal(err)
			}
		}
		if err := m.FillMatrixAAndInitRightSide(); err != nil {
			t.Fatal(err)
		}
		if err := m.FillB(); err != nil {
			t.Fatal(err)
		}
		if err := m.Scale(); err != nil {
			t.Fatal(err)
		}
		m.X = make([]float64, nc)
		for j := range m.X {
			m.X[j] = 19
		}
		if err := m.FillAcceptableValuesForX(); err != nil {
			t.Fatal(err)
		}
		if err := m.CountSlacksAndArtificials(); err != nil {
			t.Fatal(err)
		}
		if err := m.SetCoreSolverBounds(); err != nil {
			t.Fatal(err)
		}
		old := make([][]FloatRowCell, nr)
		for i := range old {
			old[i] = append([]FloatRowCell(nil), m.A.Rows[i]...)
		}
		b := append([]float64(nil), m.B...)
		if err := m.PrepareStageOneTotalInf(); err != nil {
			t.Fatal(world, err)
		}
		if m.Status != StatusUnknown || len(m.X) != nc+nr || m.A.ColumnCount() != uint32(nc+nr) || len(m.LowBounds) != nc+nr || len(m.ColumnTypes) != nc+int(m.Slacks+m.Artificials) {
			t.Fatal("source dimensions/status", world)
		}
		for i, row := range m.A.Rows {
			if len(row) != len(old[i])+1 || m.Basis[i] != uint32(nc+i) {
				t.Fatal("residual column identity", world, i)
			}
			for k, c := range old[i] {
				if row[k] != c {
					t.Fatal("structural coefficient changed")
				}
			}
			// Core row i is external row CoreRowsToExternal[i] (libstdc++
			// m_A_values order).
			rel := LPRelation(m.CoreRowsToExternal[uint32(i)] % 3)
			sign := 1.
			if rel == RelationGreaterOrEqual {
				sign = -1
			}
			cell := row[len(row)-1]
			if cell.Column != uint32(nc+i) || cell.Value != sign {
				t.Fatal("signed diagonal")
			}
			lpScalerNear(t, m.X[nc+i], sign*b[i])
			wantType := ColumnLowBound
			if rel == RelationEqual {
				wantType = ColumnFixed
			}
			if m.ColumnTypes[nc+i] != wantType || m.Costs[nc+i] != 0 || m.LowBounds[nc+i] != 0 || m.UpperBounds[nc+i] != 0 {
				t.Fatal("residual metadata")
			}
			var dot float64
			for _, c := range row {
				dot += c.Value * m.X[c.Column]
			}
			lpScalerNear(t, dot, b[i])
		}
		for j := 0; j < nc; j++ {
			c := m.Columns[m.CoreColumnsToExternal[uint32(j)]] // core column j
			cost := c.Cost
			if c.IsFlipped() {
				cost = -cost
			}
			lpScalerNear(t, m.Costs[j], cost*m.ColumnScale[j])
			lpScalerNear(t, m.X[j], 0)
			if c.ColumnType() == ColumnBoxed {
				lpScalerNear(t, m.UpperBounds[j], 5/m.ColumnScale[j])
			}
		}
		// The new signed diagonal is a real retained LU basis. Check both
		// solution orientations against independent diagonal products.
		if world%2 == 0 {
			m.Settings.DensityThreshold = 0
		} else {
			m.Settings.DensityThreshold = math.Inf(1)
		}
		m.Settings.SimplexStrategy = StrategyLU
		core, err := NewFloatPrimalCoreSolver(FloatCoreInputs{
			A: m.A, B: &m.B, X: &m.X, Costs: &m.Costs,
			Basis: &m.Basis, NBasis: &m.NBasis, Heading: &m.Heading,
			Settings: m.Settings, Names: m.FloatLPSolver,
			ColumnTypes: &m.ColumnTypes, LowBounds: &m.LowBounds, UpperBounds: &m.UpperBounds,
		})
		if err != nil {
			t.Fatal(world, err)
		}
		lu := core.Factorization
		if err != nil || lu.GetStatus() != LUOK {
			t.Fatal(world, err)
		}
		for _, right := range []bool{false, true} {
			y := append([]float64(nil), b...)
			if right {
				err = lu.SolveYB(y)
			} else {
				err = lu.SolveBy(y)
			}
			if err != nil {
				t.Fatal(err)
			}
			for i := range y {
				lpScalerNear(t, y[i], m.X[nc+i])
			}
		}
		if err := core.InitRun(); err != nil {
			t.Fatal("prepared primal run initialization", world, err)
		}
		if err := core.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLPFloatPrimalBoundsResizeAndUnsignedExtent(t *testing.T) {
	m := lpFloatPrimal(t)
	m.A = NewFloatStaticMatrix(0, 5)
	m.ColumnScale = []float64{1, 2, 3, 4, 5}
	for j := uint32(0); j < 5; j++ {
		m.Columns[j] = NewFloatColumnInfo(j)
	}
	m.Columns[0].SetFixedValue(9)
	m.Columns[1].SetLowBound(-2)
	m.Columns[1].SetUpperBound(4)
	m.Columns[2].SetLowBound(1)
	m.Columns[3].SetUpperBound(1)
	m.Columns[8] = NewFloatColumnInfo(InvalidColumnIndex)
	m.Columns[9] = NewFloatColumnInfo(1 << 31)
	m.UpperBounds = []float64{99, 99, 99, 99, 99}
	m.Slacks = 2
	m.Artificials = 3
	if err := m.SetCoreSolverBounds(); err != nil {
		t.Fatal(err)
	}
	for j, want := range []ColumnType{ColumnFixed, ColumnBoxed, ColumnLowBound, ColumnUpperBound, ColumnFree} {
		if m.ColumnTypes[j] != want {
			t.Fatal(j, m.ColumnTypes)
		}
	}
	for j, want := range []float64{0, 3, 99, 99, 99, 0, 0, 0, 0, 0} {
		lpScalerNear(t, m.UpperBounds[j], want)
	}
	// Literal unsigned wrap gives 5+UINT_MAX+1 == 5, rather than rejecting
	// the counter fields or attempting a huge Go allocation.
	m.Slacks = ^uint32(0)
	m.Artificials = 1
	if err := m.SetCoreSolverBounds(); err != nil || len(m.ColumnTypes) != 5 || len(m.UpperBounds) != 5 {
		t.Fatal(err)
	}
	m.ColumnScale[1] = 0
	if err := m.SetCoreSolverBounds(); err != nil || !math.IsInf(m.UpperBounds[1], 1) {
		t.Fatal(err, m.UpperBounds)
	}
}

func lpPrimalRowFixture(t *testing.T, rel LPRelation) *FloatPrimalSimplex {
	t.Helper()
	m := lpFloatPrimal(t)
	m.A = NewFloatStaticMatrix(1, 0)
	m.CoreRowsToExternal[0] = 7
	m.Constraints[7] = FloatConstraint{0, rel}
	m.B = []float64{-2}
	m.X = []float64{9}
	m.Basis = []uint32{8}
	m.ColumnTypes = []ColumnType{ColumnBoxed}
	m.UpperBounds = []float64{5}
	m.LowBounds = []float64{6}
	return m
}

func TestLPFloatPrimalResidualSignsBoundsAndIEEE(t *testing.T) {
	for _, rel := range []LPRelation{RelationEqual, RelationLessOrEqual, RelationGreaterOrEqual} {
		for _, b := range []float64{-2, 0, 2, math.Copysign(0, -1), math.Inf(1), math.NaN()} {
			m := lpPrimalRowFixture(t, rel)
			m.B[0] = b
			if err := m.FillAXAndBasisForStageOneTotalInfForRow(0); err != nil {
				t.Fatal(err)
			}
			sign := 1.
			if rel == RelationGreaterOrEqual {
				sign = -1
			}
			want := sign * b
			got := m.X[0]
			if math.IsNaN(want) {
				if !math.IsNaN(got) {
					t.Fatal(got)
				}
			} else if got != want || math.Signbit(got) != math.Signbit(want) {
				t.Fatal(rel, b, got, want)
			}
			if m.A.Rows[0][0].Value != sign || m.UpperBounds[0] != 0 || m.Basis[0] != 0 {
				t.Fatal("literal residual")
			}
			low := 0.
			if rel == RelationGreaterOrEqual {
				low = 6
			}
			lpScalerNear(t, m.LowBounds[0], low)
		}
	}
	// Empty source program returns OPTIMAL before clearing any vectors.
	m := lpFloatPrimal(t)
	m.A = NewFloatStaticMatrix(0, 0)
	m.LowBounds = []float64{7}
	m.X = []float64{8}
	m.Basis = []uint32{9}
	m.Costs = []float64{10}
	if err := m.PrepareStageOneTotalInf(); err != nil || m.Status != StatusOptimal || m.LowBounds[0] != 7 || m.X[0] != 8 || m.Basis[0] != 9 || m.Costs[0] != 10 {
		t.Fatal(err)
	}
	// With columns but no rows, source resizes X, clears costs and shrinks basis.
	m = lpFloatPrimal(t)
	m.A = NewFloatStaticMatrix(0, 2)
	m.X = []float64{1, 2, 3}
	m.Basis = []uint32{9}
	if err := m.PrepareStageOneTotalInf(); err != nil || len(m.X) != 2 || m.X[0] != 1 || m.X[1] != 2 || len(m.Basis) != 0 || len(m.Costs) != 2 {
		t.Fatal(err)
	}
}

func TestLPFloatPrimalPreparationPublicationPrefixes(t *testing.T) {
	for _, which := range []string{"row", "mapping", "constraint", "basis", "rhs", "x", "type", "low", "upper"} {
		m := lpPrimalRowFixture(t, RelationEqual)
		row := uint32(0)
		switch which {
		case "row":
			row = 1
		case "mapping":
			delete(m.CoreRowsToExternal, 0)
		case "constraint":
			delete(m.Constraints, 7)
		case "basis":
			m.Basis = nil
		case "rhs":
			m.B = nil
		case "x":
			m.X = nil
		case "type":
			m.ColumnTypes = nil
		case "low":
			m.LowBounds = nil
		case "upper":
			m.UpperBounds = nil
		}
		err := m.FillAXAndBasisForStageOneTotalInfForRow(row)
		want := ErrBounds
		if which == "row" || which == "mapping" || which == "constraint" {
			want = lpSolverDomainError()
		}
		if err != want {
			t.Fatal(which, err)
		}
		beforeColumn := which == "row" || which == "mapping" || which == "constraint"
		if (m.A.ColumnCount() == 0) != beforeColumn {
			t.Fatal(which, "column publication")
		}
		if which == "type" || which == "low" || which == "upper" {
			if len(m.A.Rows[0]) != 1 || m.X[0] != -2 {
				t.Fatal(which, "matrix/X prefix")
			}
		}
		if which == "upper" && m.LowBounds[0] != 0 {
			t.Fatal("chained assignment order")
		}
	}
	m := lpPrimalRowFixture(t, LPRelation(99))
	err := m.FillAXAndBasisForStageOneTotalInfForRow(0)
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	if m.A.ColumnCount() != 1 || m.Basis[0] != 0 || m.X[0] != 9 || len(m.A.Rows[0]) != 0 {
		t.Fatal("unknown relation prefix")
	}
	m = lpPrimalRowFixture(t, RelationEqual)
	m.ColumnTypes = nil
	if err := m.PrepareStageOneTotalInf(); err != ErrBounds || m.A.ColumnCount() != 1 || m.X[0] != -2 || m.Costs[0] != 0 {
		t.Fatal(err)
	}
	m = lpPrimalRowFixture(t, RelationEqual)
	var out bytes.Buffer
	m.Settings.MessageOut = &out
	if err := m.PrepareStageOneTotalInf(); err != nil || out.String() != "extended A[1,1]\n" {
		t.Fatal(err, out.String())
	}
	want := errors.New("primal output")
	m = lpPrimalRowFixture(t, RelationEqual)
	m.Settings.MessageOut = lpPresolveFailWriter{want}
	if err := m.PrepareStageOneTotalInf(); err != want || m.A.ColumnCount() != 1 || m.Costs[0] != 0 {
		t.Fatal(err)
	}
}

func TestLPFloatPrimalBoundAndCostGuards(t *testing.T) {
	m := lpFloatPrimal(t)
	m.A = NewFloatStaticMatrix(0, 1)
	m.Columns[0] = nil
	if err := m.SetCoreSolverBounds(); err != ErrBounds || len(m.ColumnTypes) != 1 {
		t.Fatal(err)
	}
	m.Columns[0] = NewFloatColumnInfo(2)
	if err := m.SetCoreSolverBounds(); err != ErrBounds {
		t.Fatal(err)
	}
	m.Columns[0].Index = 0
	m.Columns[0].SetLowBound(0)
	m.Columns[0].SetUpperBound(1)
	if err := m.SetCoreSolverBounds(); err != ErrBounds || m.ColumnTypes[0] != ColumnBoxed {
		t.Fatal(err)
	}
	m.CoreColumnsToExternal[0] = 0
	if err := m.FillAcceptableValuesForX(); err != ErrBounds {
		t.Fatal(err)
	}
	// Reverse cost traversal publishes the high column before a missing middle
	// metadata entry; lower cost remains untouched.
	m = lpFloatPrimal(t)
	m.Costs = []float64{99, 99, 99}
	m.ColumnScale = []float64{1, 2, 3}
	for j := uint32(0); j < 3; j++ {
		m.CoreColumnsToExternal[j] = j
		m.Columns[j] = NewFloatColumnInfo(j)
		m.Columns[j].Cost = 2
	}
	delete(m.Columns, 1)
	if err := m.SetScaledCosts(); err != ErrBounds || m.Costs[2] != 6 || m.Costs[0] != 99 || m.Costs[1] != 99 {
		t.Fatal(err, m.Costs)
	}
}

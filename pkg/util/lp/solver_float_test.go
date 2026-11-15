package lp

import (
	"math"
	"math/rand"
	"testing"
)

func lpTestNamedColumn(t *testing.T, names map[string]uint32, name string) uint32 {
	t.Helper()
	column, ok := names[name]
	if !ok {
		t.Fatalf("missing column name %q", name)
	}
	return column
}

func lpTestFloatNamedColumn(t *testing.T, names *StdUnorderedStringMap, name string) uint32 {
	t.Helper()
	column, ok := names.Get2(name)
	if !ok {
		t.Fatalf("missing column name %q", name)
	}
	return column
}

func lpFloatSolver(t *testing.T) *FloatLPSolver {
	t.Helper()
	m, err := NewFloatLPSolver()
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLPFloatSolverIndependentAssemblyScalingAndRestoration(t *testing.T) {
	rng := rand.New(rand.NewSource(778213))
	for world := 0; world < 120; world++ {
		m := lpFloatSolver(t)
		nr, nc := 1+rng.Intn(6), 1+rng.Intn(6)
		a := make([][]float64, nr)
		externalX := make([]float64, nc)
		shift := make([]float64, nc)
		sign := make([]float64, nc)
		coreValue := make([]float64, nc)
		for j := 0; j < nc; j++ {
			c := m.GetOrCreateColumnInfo(uint32(10 + 7*j))
			sign[j] = 1
			switch j % 4 {
			case 0:
				c.SetLowBound(float64(j - 2))
				shift[j] = float64(j - 2)
			case 1:
				c.SetUpperBound(float64(j + 4))
				shift[j] = float64(j + 4)
				sign[j] = -1
			case 2:
				c.SetLowBound(float64(j - 2))
				c.SetUpperBound(float64(j + 4))
				shift[j] = float64(j - 2)
			}
			c.Cost = float64(rng.Intn(17)-8) / 4
			coreValue[j] = float64(rng.Intn(9)-4) / 4
			externalX[j] = shift[j] + sign[j]*coreValue[j]
		}
		ro, co := rng.Perm(nr), rng.Perm(nc)
		for i := range a {
			a[i] = make([]float64, nc)
		}
		for _, i := range ro {
			var rhs float64
			for _, j := range co {
				v := math.Ldexp(float64(1+rng.Intn(7)), rng.Intn(9)-4)
				if rng.Intn(2) == 0 {
					v = -v
				}
				a[i][j] = v
				rhs += v * externalX[j]
				m.SetRowColumnCoefficient(uint32(100+11*i), uint32(10+7*j), v)
			}
			if err := m.AddConstraint(LPRelation(i%3), rhs, uint32(100+11*i)); err != nil {
				t.Fatal(err)
			}
		}
		if m.ProblemIsEmpty() || m.Status != StatusUnknown {
			t.Fatal("source initial state")
		}
		if err := m.FillMatrixAAndInitRightSide(); err != nil {
			t.Fatal(world, err)
		}
		if err := m.FillB(); err != nil {
			t.Fatal(world, err)
		}
		originalB := append([]float64(nil), m.B...)
		for r, i := range ro {
			if m.ExternalRowsToCore[uint32(100+11*i)] != uint32(r) || m.CoreRowsToExternal[uint32(r)] != uint32(100+11*i) {
				t.Fatal("row order")
			}
			var wantB float64
			for c, j := range co {
				if m.Columns[uint32(10+7*j)].Index != uint32(c) {
					t.Fatal("column order")
				}
				lpScalerNear(t, m.A.Rows[r][c].Value, a[i][j]*sign[j])
				wantB += a[i][j] * (externalX[j] - shift[j])
			}
			lpScalerNear(t, m.B[r], wantB)
		}
		m.Settings.RepsInScaler = uint32(world % 4)
		if err := m.Scale(); err != nil {
			t.Fatal(world, err)
		}
		x := make([]float64, nc)
		m.Costs = make([]float64, nc)
		var gotCost, wantCost float64
		for c, j := range co {
			x[c] = coreValue[j] / m.ColumnScale[c]
			v, err := m.ColumnValueFromCore(uint32(10+7*j), x)
			if err != nil {
				t.Fatal(err)
			}
			lpScalerNear(t, v, externalX[j])
			if err := m.SetScaledCost(uint32(c)); err != nil {
				t.Fatal(err)
			}
			gotCost += m.Costs[c] * x[c]
			wantCost += m.Columns[uint32(10+7*j)].Cost * (externalX[j] - shift[j])
		}
		lpScalerNear(t, gotCost, wantCost)
		for r, row := range m.A.Rows {
			var sum float64
			for _, c := range row {
				sum += c.Value * x[c.Column]
			}
			lpScalerNear(t, sum, m.B[r])
		}
		old := m.A
		if err := m.Unscale(); err != nil {
			t.Fatal(world, err)
		}
		if m.A == old || old.RowCount() != 0 || old.ColumnCount() != 0 {
			t.Fatal("old matrix retirement")
		}
		for r, i := range ro {
			lpScalerNear(t, m.B[r], originalB[r])
			for c, j := range co {
				lpScalerNear(t, m.A.Rows[r][c].Value, a[i][j]*sign[j])
				lpScalerNear(t, m.ColumnScale[c], 1)
			}
		}
	}
}

func TestLPFloatColumnInfoBoundStatesAndSourceTolerances(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		c := NewFloatColumnInfo(InvalidColumnIndex)
		if mask&1 != 0 {
			c.SetLowBound(2)
		}
		if mask&2 != 0 {
			c.SetUpperBound(5)
		}
		if mask&4 != 0 {
			c.SetFixedValue(3)
		}
		want := ColumnFree
		if mask&1 != 0 {
			want = ColumnLowBound
		}
		if mask&2 != 0 {
			want = ColumnUpperBound
		}
		if mask&3 == 3 {
			want = ColumnBoxed
		}
		if mask&4 != 0 {
			want = ColumnFixed
		}
		if c.ColumnType() != want || c.IsFree() != (mask&3 == 0) || c.IsFlipped() != (mask&3 == 2) {
			t.Fatal(mask, c)
		}
		shift := 0.
		if mask&1 != 0 {
			shift = 2
		}
		if mask&3 == 2 {
			shift = 5
		}
		if mask&4 != 0 {
			shift = 3
		}
		lpScalerNear(t, c.Shift(), shift)
		for _, v := range []float64{-10, 2 - 1e-7, 2, 3, 5, 5 + 1e-6, 10, math.NaN()} {
			want := (mask&1 == 0 || v >= 2-1e-7) && (mask&2 == 0 || v <= 5+1e-6)
			if c.BoundsHold(v) != want {
				t.Fatal(mask, v)
			}
		}
	}
	c := NewFloatColumnInfo(3)
	c.SetLowBound(2)
	c.SetUpperBound(5)
	if v, err := c.AdjustedUpperBound(); err != nil || v != 3 {
		t.Fatal(v, err)
	}
	for _, i := range []uint32{0, 1, 1<<31 - 1, 1 << 31, InvalidColumnIndex} {
		if IsValidColumnIndex(i) != (int32(i) >= 0) {
			t.Fatal(i)
		}
	}
	for _, which := range []string{"low", "upper", "fixed", "adjusted"} {
		c = NewFloatColumnInfo(0)
		var err error
		switch which {
		case "low":
			_, err = c.GetLowBound()
		case "upper":
			_, err = c.GetUpperBound()
		case "fixed":
			_, err = c.GetFixedValue()
		case "adjusted":
			_, err = c.AdjustedUpperBound()
		}
		if sourceDebugEnabled {
			if err != ErrAssertion {
				t.Fatal(which, err)
			}
		} else if err != nil {
			t.Fatal(which, err)
		}
	}
}

func TestLPFloatSolverSlackCountsNamesAndBypass(t *testing.T) {
	m := lpFloatSolver(t)
	if !m.ProblemIsEmpty() {
		t.Fatal("empty solver")
	}
	if m.ColumnName(9) != "x9" {
		t.Fatal("name defaults")
	}
	m.CoreColumnsToExternal[0] = 17
	if m.ColumnName(0) != "x17" {
		t.Fatal("external fallback")
	}
	m.GiveSymbolicNameToColumn("a", 17)
	m.GiveSymbolicNameToColumn("b", 17)
	if m.ColumnName(0) != "b" || lpTestFloatNamedColumn(t, m.NamesToColumns, "a") != 17 || lpTestFloatNamedColumn(t, m.NamesToColumns, "b") != 17 {
		t.Fatal("source stale name alias")
	}
	m = lpFloatSolver(t)
	for i := uint32(0); i < 9; i++ {
		m.SetRowColumnCoefficient(i, 7, 1)
		m.GetOrCreateColumnInfo(7)
		if err := m.AddConstraint(LPRelation(i/3), float64(int(i%3)-1), i); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.FillMatrixAAndInitRightSide(); err != nil {
		t.Fatal(err)
	}
	if err := m.FillB(); err != nil {
		t.Fatal(err)
	}
	if err := m.CountSlacksAndArtificials(); err != nil {
		t.Fatal(err)
	}
	if m.Slacks != 6 || m.Artificials != 5 {
		t.Fatal(m.Slacks, m.Artificials)
	}
	if err := m.CountSlacksAndArtificials(); err != nil {
		t.Fatal(err)
	}
	if m.Slacks != 12 || m.Artificials != 10 {
		t.Fatal("source counters accumulate")
	}
	m.ColumnScale = []float64{2, 3, 4}
	m.Settings.UseScaling = false
	old := m.A
	before := append([]float64(nil), m.B...)
	if err := m.Scale(); err != nil {
		t.Fatal(err)
	}
	if len(m.ColumnScale) != 1 || m.ColumnScale[0] != 1 || m.A != old {
		t.Fatal("bypass")
	}
	for i := range before {
		lpScalerNear(t, m.B[i], before[i])
	}
	m.Columns[10] = NewFloatColumnInfo(InvalidColumnIndex)
	m.Columns[10].SetFixedValue(19)
	if v, err := m.ColumnValueFromCore(10, nil); err != nil || v != 19 {
		t.Fatal(v, err)
	}
	if v, err := m.ColumnValueFromCore(999, nil); err != nil || v != 0 {
		t.Fatal(v, err)
	}
	m.Columns[11] = NewFloatColumnInfo(InvalidColumnIndex)
	if v, err := m.ColumnValueFromCore(11, nil); err != nil || v != 0 {
		t.Fatal(v, err)
	}
}

func lpSolverDomainError() error {
	if sourceDebugEnabled {
		return ErrAssertion
	}
	return ErrBounds
}

func TestLPFloatSolverSourceErrorsAndPublicationPrefixes(t *testing.T) {
	m := lpFloatSolver(t)
	m.SetRowColumnCoefficient(4, 9, 0)
	if err := m.FillMatrixAAndInitRightSide(); err != ErrLPFixedColumn || m.A != nil || len(m.Columns) != 0 || m.ExternalRowsToCore[4] != 0 {
		t.Fatal(err, m)
	}
	m = lpFloatSolver(t)
	m.GetOrCreateColumnInfo(9)
	m.GetOrCreateColumnInfo(10).SetFixedValue(3)
	m.SetRowColumnCoefficient(4, 9, 1)
	m.SetRowColumnCoefficient(5, 10, 1)
	if err := m.FillMatrixAAndInitRightSide(); err != ErrLPFixedColumn || len(m.CoreColumnsToExternal) != 1 || len(m.ExternalRowsToCore) != 2 {
		t.Fatal(err, m)
	}
	// Replacement coefficients retain their insertion position and names.
	m = lpFloatSolver(t)
	m.SetRowColumnCoefficient(4, 9, 1)
	m.SetRowColumnCoefficient(4, 9, 2)
	m.GetOrCreateColumnInfo(9)
	if err := m.AddConstraint(RelationEqual, 3, 4); err != nil {
		t.Fatal(err)
	}
	err := m.AddConstraint(RelationGreaterOrEqual, 7, 4)
	if sourceDebugEnabled {
		if err != ErrAssertion || m.Constraints[4].RightSide != 3 {
			t.Fatal(err)
		}
	} else if err != nil || m.Constraints[4].RightSide != 7 {
		t.Fatal(err)
	}
	m.B = []float64{19, 20}
	if err := m.FillMatrixAAndInitRightSide(); err != nil || len(m.B) != 1 || m.B[0] != 19 {
		t.Fatal(err, m.B)
	}
	lpScalerNear(t, m.A.Rows[0][0].Value, 2)
	old := m.A
	err = m.FillMatrixAAndInitRightSide()
	if sourceDebugEnabled {
		if err != ErrAssertion || m.A != old {
			t.Fatal(err)
		}
	} else if err != nil || m.A == old {
		t.Fatal(err)
	}
	for _, which := range []string{"missing-row", "missing-column", "invalid-column", "row-bounds", "column-bounds"} {
		m = lpFloatSolver(t)
		m.SetRowColumnCoefficient(4, 9, 2)
		m.ExternalRowsToCore[4] = 0
		m.CoreColumnsToExternal[0] = 9
		m.Columns[9] = NewFloatColumnInfo(0)
		switch which {
		case "missing-row":
			delete(m.ExternalRowsToCore, 4)
		case "missing-column":
			delete(m.Columns, 9)
		case "invalid-column":
			m.Columns[9].Index = InvalidColumnIndex
		case "row-bounds":
			m.ExternalRowsToCore[4] = 2
		case "column-bounds":
			m.Columns[9].Index = 2
		}
		err = m.FillAFromAValues()
		want := lpSolverDomainError()
		if err != want || m.A == nil || m.A.RowCount() != 1 {
			t.Fatal(which, err, m.A)
		}
	}
	m = lpFloatSolver(t)
	m.SetRowColumnCoefficient(4, 9, 1)
	if err := m.MapExternalColumnsToCoreSolverColumns(); err != ErrBounds {
		t.Fatal(err)
	}
	if c, exists := m.Columns[9]; !exists || c != nil || m.GetOrCreateColumnInfo(9) != nil {
		t.Fatal("literal source null insertion")
	}
	if _, err := m.LowBoundShiftForRow(999); err != ErrLPSolverMissingRow {
		t.Fatal(err)
	}
	if _, err := m.LowBoundShiftForRow(4); err != ErrBounds {
		t.Fatal(err)
	}
	// Retire old A, publish reconstructed A, restore scales, then descend RHS.
	m = lpFloatSolver(t)
	for i := uint32(0); i < 2; i++ {
		m.SetRowColumnCoefficient(i, 7, 2)
		m.GetOrCreateColumnInfo(7)
		if err := m.AddConstraint(RelationEqual, float64(i+4), i); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.FillMatrixAAndInitRightSide(); err != nil {
		t.Fatal(err)
	}
	old = m.A
	m.ColumnScale = []float64{3}
	m.B = []float64{77, 88}
	delete(m.Constraints, 0)
	if err := m.Unscale(); err != lpSolverDomainError() || m.A == old || old.RowCount() != 0 || m.ColumnScale[0] != 1 || m.B[0] != 77 || m.B[1] != 5 {
		t.Fatal(err, m.B, m.ColumnScale)
	}
	delete(m.ExternalRowsToCore, 0)
	old = m.A
	m.ColumnScale[0] = 3
	if err := m.Unscale(); err != lpSolverDomainError() || m.A == old || old.RowCount() != 0 || m.ColumnScale[0] != 3 {
		t.Fatal(err)
	}
	m = lpFloatSolver(t)
	m.A = NewFloatStaticMatrix(1, 1)
	if err := m.FillB(); err != lpSolverDomainError() {
		t.Fatal(err)
	}
	if v, ok := m.CoreRowsToExternal[0]; !ok || v != 0 {
		t.Fatal("source row operator[]")
	}
	m.Constraints[0] = FloatConstraint{3, RelationEqual}
	if err := m.FillB(); err != ErrLPSolverMissingRow {
		t.Fatal(err)
	}
	m.SetRowColumnCoefficient(0, 2, 1)
	m.GetOrCreateColumnInfo(2)
	if err := m.FillB(); err != ErrBounds {
		t.Fatal(err)
	}
	for _, rel := range []LPRelation{RelationLessOrEqual, RelationGreaterOrEqual} {
		m = lpFloatSolver(t)
		m.Constraints[0] = FloatConstraint{0, rel}
		if err := m.CountSlacksAndArtificialsForRow(0); err != ErrBounds || m.Slacks != 1 {
			t.Fatal(rel, err, m.Slacks)
		}
	}
	m = lpFloatSolver(t)
	m.A = NewFloatStaticMatrix(1, 1)
	if err := m.CountSlacksAndArtificials(); err != lpSolverDomainError() {
		t.Fatal(err)
	}
	m = lpFloatSolver(t)
	m.SetRowColumnCoefficient(4, 9, 2)
	m.GetOrCreateColumnInfo(9)
	if err := m.FillMatrixAAndInitRightSide(); err != nil {
		t.Fatal(err)
	}
	old = m.A
	m.Settings.ScaleWithRatio = false
	if err := m.Scale(); err != ErrLPBalanceNotImplemented || m.A != old || m.A.Rows[0][0].Value != 2 {
		t.Fatal(err)
	}
	if sourceDebugEnabled {
		if err := m.Scale(); err != ErrAssertion {
			t.Fatal("nonempty scale assertion", err)
		}
	}
}

func TestLPFloatSolverScaledCostAndValueGuards(t *testing.T) {
	m := lpFloatSolver(t)
	if err := m.SetScaledCost(0); err != lpSolverDomainError() {
		t.Fatal(err)
	}
	m.ColumnScale = []float64{1}
	if err := m.SetScaledCost(0); err != ErrBounds {
		t.Fatal(err)
	}
	if v, ok := m.CoreColumnsToExternal[0]; !ok || v != 0 {
		t.Fatal("source missing core key insertion")
	}
	c := NewFloatColumnInfo(0)
	m.Columns[0] = c
	c.SetFixedValue(3)
	if err := m.SetScaledCost(0); err != lpSolverDomainError() {
		t.Fatal(err)
	}
	c.Fixed = false
	if err := m.SetScaledCost(0); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err := m.ColumnValueFromCore(0, nil); err != ErrBounds {
		t.Fatal(err)
	}
	m.ColumnScale = nil
	if _, err := m.ColumnValueFromCore(0, []float64{2}); err != ErrBounds {
		t.Fatal(err)
	}
	c.SetUpperBound(5)
	if _, err := c.AdjustedUpperBound(); sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
}

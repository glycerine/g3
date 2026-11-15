package lp

import (
	"bytes"
	"errors"
	"io"
	"math"
	"math/big"
	"math/rand"
	"testing"
)

func lpPresolveFixture(t *testing.T, a, low, high []float64, rel LPRelation, rhs float64) *FloatLPSolver {
	t.Helper()
	m := lpFloatSolver(t)
	m.Settings.DebugOut = nil
	for j, v := range a {
		m.SetRowColumnCoefficient(0, uint32(j), v)
		c := m.GetOrCreateColumnInfo(uint32(j))
		c.SetLowBound(low[j])
		c.SetUpperBound(high[j])
	}
	if err := m.AddConstraint(rel, rhs, 0); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLPFloatPresolveIndependentCornerOracle(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	rng := rand.New(rand.NewSource(911749))
	for world := 0; world < 100; world++ {
		n := 1 + rng.Intn(6)
		a, lo, hi := make([]float64, n), make([]float64, n), make([]float64, n)
		for j := range a {
			a[j] = float64(1+rng.Intn(7)) / 4
			if rng.Intn(2) == 0 {
				a[j] = -a[j]
			}
			lo[j] = float64(rng.Intn(9)-4) / 4
			hi[j] = lo[j] + float64(1+rng.Intn(7))/4
		}
		var minimum, maximum *big.Rat
		var minX, maxX []float64
		// Enumerate every box corner with exact arithmetic, independently of
		// the sign-selection algorithm in the source and Go implementation.
		for mask := 0; mask < 1<<n; mask++ {
			x := make([]float64, n)
			sum := new(big.Rat)
			for j := range x {
				x[j] = lo[j]
				if mask&(1<<j) != 0 {
					x[j] = hi[j]
				}
				sum.Add(sum, new(big.Rat).Mul(new(big.Rat).SetFloat64(a[j]), new(big.Rat).SetFloat64(x[j])))
			}
			if minimum == nil || sum.Cmp(minimum) < 0 {
				minimum = new(big.Rat).Set(sum)
				minX = x
			}
			if maximum == nil || sum.Cmp(maximum) > 0 {
				maximum = new(big.Rat).Set(sum)
				maxX = x
			}
		}
		min, _ := minimum.Float64()
		max, _ := maximum.Float64()
		m := lpPresolveFixture(t, a, lo, hi, RelationEqual, 0)
		row, _ := m.AValues.Get2(0)
		v, known, err := m.MinimalRowValue(row)
		if err != nil || !known {
			t.Fatal(err, known)
		}
		lpScalerNear(t, v, min)
		v, known, err = m.MaximalRowValue(row)
		if err != nil || !known {
			t.Fatal(err, known)
		}
		lpScalerNear(t, v, max)
		for _, rel := range []LPRelation{RelationEqual, RelationLessOrEqual, RelationGreaterOrEqual} {
			for _, rhs := range []float64{min - 2, min, (min + max) / 2, max, max + 2} {
				m = lpPresolveFixture(t, a, lo, hi, rel, rhs)
				row, _ = m.AValues.Get2(0)
				impossible := rel != RelationGreaterOrEqual && rhs < min || rel != RelationLessOrEqual && rhs > max
				var pin []float64
				if rel != RelationGreaterOrEqual && rhs == min {
					pin = minX
				}
				if rel != RelationLessOrEqual && rhs == max {
					pin = maxX
				}
				obsolete, err := m.RowIsObsolete(row, 0)
				if err != nil || obsolete != (impossible || pin != nil) || (m.Status == StatusInfeasible) != impossible {
					t.Fatal(world, rel, rhs, min, max, obsolete, m.Status, err)
				}
				for j, c := range m.Columns {
					if c.Fixed != (pin != nil) {
						t.Fatal("pin flag", world, rel, rhs, j)
					}
					if pin != nil {
						lpScalerNear(t, c.FixedValue, pin[j])
					}
				}
			}
		}
	}
}

func TestLPFloatPresolveDirectionalToleranceAndIEEE(t *testing.T) {
	for _, tc := range []struct {
		rhs                  float64
		obsolete, infeasible bool
		pin                  float64
	}{
		{-.125, true, true, 0}, {-.124, true, false, 0}, {0, true, false, 0}, {.124, true, false, 0},
		{.125, false, false, 0}, {1.875, false, false, 0}, {1.876, true, false, 2}, {2, true, false, 2},
		{2.124, true, false, 2}, {2.125, true, true, 0}, {math.NaN(), true, true, 0},
	} {
		m := lpPresolveFixture(t, []float64{1}, []float64{0}, []float64{2}, RelationEqual, tc.rhs)
		m.Settings.RefactorTolerance = .125
		row, _ := m.AValues.Get2(0)
		got, err := m.RowIsObsolete(row, 0)
		if err != nil || got != tc.obsolete || (m.Status == StatusInfeasible) != tc.infeasible {
			t.Fatal(tc, got, m.Status, err)
		}
		c := m.Columns[0]
		if c.Fixed != (tc.obsolete && !tc.infeasible) {
			t.Fatal(tc, c)
		}
		if c.Fixed {
			lpScalerNear(t, c.FixedValue, tc.pin)
		}
	}
	for _, rel := range []LPRelation{RelationEqual, RelationLessOrEqual, RelationGreaterOrEqual} {
		for _, rhs := range []float64{-1, 0, 1, math.NaN()} {
			m := lpPresolveFixture(t, []float64{math.Copysign(0, -1)}, []float64{0}, []float64{1}, rel, rhs)
			row, _ := m.AValues.Get2(0)
			bad := rhs != 0
			if rel == RelationLessOrEqual {
				bad = rhs < 0
			}
			if rel == RelationGreaterOrEqual {
				bad = rhs > 0
			}
			if got, err := m.RowIsObsolete(row, 0); err != nil || !got || (m.Status == StatusInfeasible) != bad {
				t.Fatal(rel, rhs, got, m.Status, err)
			}
		}
	}
	// Source exact equality at a zero tolerance is classified infeasible.
	m := lpPresolveFixture(t, []float64{1}, []float64{0}, []float64{2}, RelationEqual, 0)
	m.Settings.RefactorTolerance = 0
	row, _ := m.AValues.Get2(0)
	if got, err := m.RowIsObsolete(row, 0); err != nil || !got || m.Status != StatusInfeasible {
		t.Fatal(got, err)
	}
	for _, a := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		m = lpPresolveFixture(t, []float64{a}, []float64{0}, []float64{1}, RelationEqual, 0)
		row, _ = m.AValues.Get2(0)
		if got, err := m.RowIsObsolete(row, 0); err != nil || !got || m.Status != StatusInfeasible {
			t.Fatal(a, got, err, m.Status)
		}
	}
}

func TestLPFloatPresolveUnknownBoundsAndPinPrefixes(t *testing.T) {
	for _, maximum := range []bool{false, true} {
		for _, coefficient := range []float64{-2, 0, 2} {
			m := lpPresolveFixture(t, []float64{1, coefficient, 3}, []float64{2, 4, 6}, []float64{3, 5, 7}, RelationEqual, 100)
			c := m.Columns[1]
			low := coefficient > 0
			if maximum {
				low = coefficient < 0
			}
			if low {
				c.LowSet = false
			} else {
				c.UpperSet = false
			}
			row, _ := m.AValues.Get2(0)
			v, known, err := m.rowBound(row, maximum)
			// libstdc++ visits columns 2 1 0: column 2 (3 * [6, 7])
			// contributes before the unknown bound of column 1.
			want := 18.
			if maximum {
				want = 21
			}
			if err != nil || known || v != want {
				t.Fatal(maximum, coefficient, v, known, err)
			}
			// Even a zero term requires the source-selected upper bound.
		}
	}
	m := lpPresolveFixture(t, []float64{1, 1}, []float64{2, 4}, []float64{3, 5}, RelationEqual, 0)
	row, _ := m.AValues.Get2(0)
	// libstdc++ visits column 1 before column 0.
	m.Columns[0].LowSet = false
	err := m.PinVarsOnRowWithSign(row, -1)
	if !m.Columns[1].Fixed || m.Columns[1].FixedValue != 4 {
		t.Fatal("successful earlier pin")
	}
	if sourceDebugEnabled {
		if err != ErrAssertion || m.Columns[0].Fixed {
			t.Fatal(err)
		}
	} else if err != nil || !m.Columns[0].Fixed || m.Columns[0].FixedValue != 2 {
		t.Fatal(err)
	}
	m = lpPresolveFixture(t, []float64{1}, []float64{2}, []float64{3}, RelationEqual, 0)
	row, _ = m.AValues.Get2(0)
	m.Columns[0].UpperSet = false
	err = m.PinVarsOnRowWithSign(row, 1)
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil || m.Columns[0].FixedValue != 3 {
		t.Fatal(err)
	}
	m = lpPresolveFixture(t, []float64{1, 0}, []float64{0, 0}, []float64{1, 1}, RelationEqual, 0)
	row, _ = m.AValues.Get2(0)
	m.Columns[1].LowSet = false
	got, err := m.RowIsObsolete(row, 0)
	if sourceDebugEnabled {
		// libstdc++ visits column 1 (the missing-bound assertion) before
		// column 0 is pinned.
		if got || err != ErrAssertion || m.Columns[0].Fixed {
			t.Fatal(got, err)
		}
	} else if !got || err != nil {
		t.Fatal(got, err)
	}
}

func TestLPFloatPresolveCleanupReportBoundaries(t *testing.T) {
	fixture := func(passes int) *FloatLPSolver {
		m := lpFloatSolver(t)
		if passes == 1 {
			// Three rows removed together still produce the singular report.
			m.GetOrCreateColumnInfo(0)
			for i := uint32(0); i < 3; i++ {
				m.SetRowColumnCoefficient(i, 0, 0)
				if err := m.AddConstraint(RelationEqual, 0, i); err != nil {
					t.Fatal(err)
				}
			}
		} else if passes == 2 {
			x, y := m.GetOrCreateColumnInfo(0), m.GetOrCreateColumnInfo(1)
			x.SetLowBound(0)
			x.SetUpperBound(1)
			y.SetLowBound(0)
			y.SetUpperBound(2)
			m.SetRowColumnCoefficient(0, 0, 1)
			m.SetRowColumnCoefficient(0, 1, 1)
			for i := uint32(0); i < 3; i++ {
				if i > 0 {
					m.SetRowColumnCoefficient(i, 0, 1)
				}
				if err := m.AddConstraint(RelationEqual, 1, i); err != nil {
					t.Fatal(err)
				}
			}
		}
		return m
	}
	checkState := func(m *FloatLPSolver, passes int) {
		t.Helper()
		if m.AValues.Len() != 0 {
			t.Fatal("output failure preceded completed row erasure")
		}
		if passes == 2 {
			x, y := m.Columns[0], m.Columns[1]
			if !x.Fixed || x.FixedValue != 1 || !y.Fixed || y.FixedValue != 0 {
				t.Fatal("output failure preceded completed pinning", x, y)
			}
		}
	}
	refused := errors.New("cleanup report refused")
	for passes, report := range []string{"", "deleted one row\n", "deleted 2 rows\n"} {
		for _, writeErr := range []error{nil, refused} {
			for limit := 0; limit < len(report); limit++ {
				m := fixture(passes)
				w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: limit, err: writeErr}}
				m.Settings.DebugOut = w
				want := writeErr
				if want == nil {
					want = io.ErrShortWrite
				}
				if err := m.Cleanup(); err != want || w.output.String() != report[:limit] || w.flushes != 0 {
					t.Fatal(passes, limit, err, w.output.String(), w.flushes)
				}
				checkState(m, passes)
			}
		}
		for _, failFlush := range []bool{false, true} {
			m := fixture(passes)
			w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: len(report)}}
			var want error
			if failFlush && passes > 0 {
				w.failAt, want = 1, lpErrResidualWrite
			}
			m.Settings.DebugOut = w
			flushes := 0
			if passes > 0 {
				flushes = 1
			}
			if err := m.Cleanup(); err != want || w.output.String() != report || w.flushes != flushes {
				t.Fatal(passes, failFlush, err, w.output.String(), w.flushes)
			}
			checkState(m, passes)
		}
		m := fixture(passes)
		m.Settings.DebugOut = nil
		if err := m.Cleanup(); err != nil {
			t.Fatal(err)
		}
		checkState(m, passes)
	}
}

func TestLPFloatPresolveCleanupPassesAndDeferredErasure(t *testing.T) {
	m := lpFloatSolver(t)
	var out bytes.Buffer
	m.Settings.DebugOut = &out
	x, y := m.GetOrCreateColumnInfo(0), m.GetOrCreateColumnInfo(1)
	x.SetLowBound(0)
	x.SetUpperBound(1)
	y.SetLowBound(0)
	y.SetUpperBound(2)
	m.SetRowColumnCoefficient(0, 0, 1)
	m.SetRowColumnCoefficient(0, 1, 1)
	for i := uint32(0); i < 3; i++ {
		if i > 0 {
			m.SetRowColumnCoefficient(i, 0, 1)
		}
		if err := m.AddConstraint(RelationEqual, 1, i); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if m.AValues.Len() != 0 || out.String() != "deleted 2 rows\n" || !x.Fixed || x.FixedValue != 1 || !y.Fixed || y.FixedValue != 0 {
		t.Fatal(out.String(), m.AValues.Len(), x, y)
	}
	// Removing multiple rows in one pass prints the source singular message.
	m = lpFloatSolver(t)
	out.Reset()
	m.Settings.DebugOut = &out
	for i := uint32(0); i < 2; i++ {
		m.SetRowColumnCoefficient(i, 0, 0)
		m.GetOrCreateColumnInfo(0)
		if err := m.AddConstraint(RelationEqual, 0, i); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Cleanup(); err != nil || out.String() != "deleted one row\n" {
		t.Fatal(err, out.String())
	}
	// No removed rows means no next pass, even if fixed-column elimination
	// just made a row empty. A subsequent explicit cleanup removes it.
	m = lpFloatSolver(t)
	m.Settings.DebugOut = nil
	m.GetOrCreateColumnInfo(0).SetFixedValue(3)
	m.SetRowColumnCoefficient(0, 0, 2)
	if err := m.AddConstraint(RelationEqual, 6, 0); err != nil {
		t.Fatal(err)
	}
	if err := m.Cleanup(); err != nil || m.AValues.Len() != 1 {
		t.Fatal(err)
	}
	row, _ := m.AValues.Get2(0)
	if row.Len() != 0 || m.Constraints[0].RightSide != 0 {
		t.Fatal("fixed substitution")
	}
	if err := m.Cleanup(); err != nil || m.AValues.Len() != 0 {
		t.Fatal(err)
	}
	// Infeasibility returns before deleting pending obsolete rows or removing
	// fixed terms. libstdc++ visits the infeasible row 1 before row 0, so
	// row 0 has not pinned its column yet.
	m = lpPresolveFixture(t, []float64{1}, []float64{0}, []float64{1}, RelationEqual, 0)
	m.SetRowColumnCoefficient(1, 0, 1)
	if err := m.AddConstraint(RelationEqual, -1, 1); err != nil {
		t.Fatal(err)
	}
	if n, err := m.TryToRemoveSomeRows(); err != nil || n != 0 || m.Status != StatusInfeasible || m.AValues.Len() != 2 || m.Columns[0].Fixed {
		t.Fatal(n, err, m.Status)
	}
	row, _ = m.AValues.Get2(0)
	if row.Len() != 1 {
		t.Fatal("premature erasure")
	}
}

type lpPresolveFailWriter struct{ err error }

func (w lpPresolveFailWriter) Write(p []byte) (int, error) { return 0, w.err }

func TestLPFloatPresolveInputAndWriterErrorPrefixes(t *testing.T) {
	m := lpPresolveFixture(t, []float64{2, 1}, []float64{0, 0}, []float64{1, 1}, RelationEqual, 10)
	row, _ := m.AValues.Get2(0)
	m.Columns[0].SetFixedValue(3)
	delete(m.Columns, 1)
	err := m.RemoveFixedOrZeroColumnsFromRow(0, row)
	// libstdc++ visits the missing column 1 before the fixed column 0, so
	// the right side is not adjusted yet.
	if err != lpSolverDomainError() || row.Len() != 2 || m.Constraints[0].RightSide != 10 {
		t.Fatal(err, row.Len(), m.Constraints[0])
	}
	m = lpPresolveFixture(t, []float64{1}, []float64{0}, []float64{1}, RelationEqual, .5)
	row, _ = m.AValues.Get2(0)
	delete(m.Columns, 0)
	if _, _, err := m.MinimalRowValue(row); err != ErrBounds {
		t.Fatal(err)
	}
	if ci, exists := m.Columns[0]; !exists || ci != nil {
		t.Fatal("source null insertion")
	}
	if err := m.PinVarsOnRowWithSign(row, 1); err != ErrBounds {
		t.Fatal(err)
	}
	delete(m.Constraints, 0)
	if got, err := m.RowIsObsolete(row, 0); got || err != ErrBounds {
		t.Fatal(got, err)
	}
	if _, ok := m.Constraints[0]; !ok {
		t.Fatal("constraint insertion")
	}
	m.Constraints[0] = FloatConstraint{0, LPRelation(99)}
	got, err := m.RowIsObsolete(row, 0)
	if got || sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(got, err)
	}
	// Empty/zero-only rows need no RHS read while removing terms, so the
	// source default constraint insertion can complete without a numeric guard.
	m = lpFloatSolver(t)
	m.SetRowColumnCoefficient(0, 0, 0)
	m.GetOrCreateColumnInfo(0)
	row, _ = m.AValues.Get2(0)
	if err := m.RemoveFixedOrZeroColumns(); err != nil || row.Len() != 0 {
		t.Fatal(err)
	}
	m = lpFloatSolver(t)
	m.SetRowColumnCoefficient(0, 0, 1)
	m.GetOrCreateColumnInfo(0).SetFixedValue(2)
	row, _ = m.AValues.Get2(0)
	if err := m.RemoveFixedOrZeroColumnsFromRow(0, row); err != ErrBounds || row.Len() != 1 {
		t.Fatal(err)
	}
	m = lpFloatSolver(t)
	m.SetRowColumnCoefficient(0, 0, 1)
	if err := m.Cleanup(); err != ErrBounds {
		t.Fatal(err)
	}
	m = lpPresolveFixture(t, []float64{0}, []float64{0}, []float64{1}, RelationEqual, 0)
	want := errors.New("presolve writer")
	m.Settings.DebugOut = lpPresolveFailWriter{want}
	if err := m.Cleanup(); err != want || m.AValues.Len() != 0 {
		t.Fatal(err)
	}
}

func TestLPFloatPresolveComposedErrorsAndPreparedProblem(t *testing.T) {
	m := lpPresolveFixture(t, []float64{1}, []float64{0}, []float64{1}, RelationEqual, .5)
	row, _ := m.AValues.Get2(0)
	delete(m.Columns, 0)
	if got, err := m.RowIsObsolete(row, 0); got || err != ErrBounds {
		t.Fatal(got, err)
	}
	delete(m.Columns, 0)
	if err := m.PinVarsOnRowWithSign(row, 1); err != ErrBounds {
		t.Fatal(err)
	}
	if ci, ok := m.Columns[0]; !ok || ci != nil {
		t.Fatal("pin null insertion")
	}
	// Both extrema stop at the first unknown bound, so the later missing
	// column is first reached by term elimination, AFTER pending row erasure.
	m = lpFloatSolver(t)
	m.Settings.DebugOut = nil
	m.GetOrCreateColumnInfo(0)
	m.SetRowColumnCoefficient(0, 0, 0)
	// Inserting 2 before 0 makes libstdc++ visit column 0 first.
	m.SetRowColumnCoefficient(1, 2, 1)
	m.SetRowColumnCoefficient(1, 0, 1)
	for i := uint32(0); i < 2; i++ {
		if err := m.AddConstraint(RelationEqual, 0, i); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := m.TryToRemoveSomeRows(); n != 0 || err != lpSolverDomainError() || m.AValues.Len() != 1 {
		t.Fatal(n, err, m.AValues.Len())
	}
	if _, ok := m.AValues.Get2(0); ok {
		t.Fatal("deleted row was restored")
	}
	// Actual retained cleanup -> matrix/RHS assembly -> scaling -> external
	// value and objective path, with one variable fixed by a different row.
	m = lpFloatSolver(t)
	m.Settings.DebugOut = nil
	x, y := m.GetOrCreateColumnInfo(10), m.GetOrCreateColumnInfo(20)
	x.SetLowBound(0)
	x.SetUpperBound(1)
	y.SetLowBound(0)
	y.SetUpperBound(5)
	y.Cost = 7
	m.SetRowColumnCoefficient(0, 10, 1)
	m.SetRowColumnCoefficient(0, 20, 1)
	m.SetRowColumnCoefficient(1, 10, 1)
	if err := m.AddConstraint(RelationEqual, 3, 0); err != nil {
		t.Fatal(err)
	}
	if err := m.AddConstraint(RelationEqual, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := m.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if m.AValues.Len() != 1 || !x.Fixed || x.FixedValue != 1 || m.Constraints[0].RightSide != 2 {
		t.Fatal("presolve result", m.Constraints, x)
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
	if len(m.CoreColumnsToExternal) != 1 || m.CoreColumnsToExternal[0] != 20 || m.B[0] != 2 {
		t.Fatal("prepared problem")
	}
	v, err := m.ColumnValueFromCore(10, nil)
	if err != nil || v != 1 {
		t.Fatal(v, err)
	}
	v, err = m.ColumnValueFromCore(20, []float64{2})
	if err != nil || v != 2 {
		t.Fatal(v, err)
	}
	m.Costs = make([]float64, 1)
	if err := m.SetScaledCost(0); err != nil || m.Costs[0]*2 != 14 {
		t.Fatal(err, m.Costs)
	}
	if err := m.Unscale(); err != nil || m.B[0] != 2 || m.A.Rows[0][0].Value != 1 {
		t.Fatal(err)
	}
}

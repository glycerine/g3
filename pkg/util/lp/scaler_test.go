package lp

import (
	"math"
	"math/rand"
	"testing"
)

func lpScalerFixture(t *testing.T, rows [][]float64, b []float64) *FloatScaler {
	t.Helper()
	n := 0
	if len(rows) != 0 {
		n = len(rows[0])
	}
	a := NewFloatStaticMatrix(uint32(len(rows)), uint32(n))
	for i, row := range rows {
		for j, v := range row {
			if v != 0 {
				if err := a.Set(uint32(i), uint32(j), v); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	s, err := NewSettings()
	if err != nil {
		t.Fatal(err)
	}
	var cs []float64
	m, err := NewFloatScaler(&b, a, &s.ScalingMinimum, &s.ScalingMaximum, &cs, s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func lpScalerNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > 2e-12*math.Max(1, math.Abs(want)) {
		t.Fatalf("got %.17g want %.17g", got, want)
	}
}

func TestLPFloatScalerIndependentSolutionOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(904114))
	for world := 0; world < 160; world++ {
		nr, nc := 1+rng.Intn(9), 1+rng.Intn(9)
		rows := make([][]float64, nr)
		b := make([]float64, nr)
		x := make([]float64, nc)
		for j := range x {
			x[j] = float64(rng.Intn(17)-8) / 8
		}
		for i := range rows {
			rows[i] = make([]float64, nc)
			for j := range rows[i] {
				if j != i%nc && rng.Intn(3) == 0 {
					continue
				}
				v := math.Ldexp(float64(1+rng.Intn(7)), rng.Intn(21)-10)
				if rng.Intn(2) == 0 {
					v = -v
				}
				rows[i][j] = v
				b[i] += float64(v * x[j])
			}
		}
		m := lpScalerFixture(t, rows, b)
		m.Settings.RepsInScaler = uint32(world % 5)
		ok, err := m.Scale()
		if err != nil || !ok {
			t.Fatal(world, ok, err)
		}
		// Independent original problem solution, transformed by column scales.
		// Every final row AND nonempty column must have a unit maximum.
		for i, row := range m.A.Rows {
			var dot, max float64
			for _, c := range row {
				dot += c.Value * x[c.Column] / (*m.ColumnScale)[c.Column]
				max = math.Max(max, math.Abs(c.Value))
			}
			lpScalerNear(t, dot, (*m.B)[i])
			lpScalerNear(t, max, 1)
		}
		for j, col := range m.A.Columns {
			var max float64
			for _, c := range col {
				max = math.Max(max, math.Abs(m.A.Rows[c.Row][c.Offset].Value))
			}
			if len(col) != 0 {
				lpScalerNear(t, max, 1)
			}
			if !((*m.ColumnScale)[j] > 0) {
				t.Fatal("invalid scale", world, j)
			}
		}
	}
}

func TestLPFloatScalerGeometricMeansAndNormalization(t *testing.T) {
	for _, rows := range [][][]float64{
		{{2, 32}, {4, 8}}, // row ratio larger
		{{2, 4}, {32, 8}}, // column ratio larger
		{{2, 8}, {8, 2}},  // equality selects columns first
	} {
		m := lpScalerFixture(t, rows, []float64{7, -9})
		rr, _ := m.MaxRatioOnRows()
		cr, _ := m.MaxRatioOnColumns()
		// Build independent simultaneous diagonal row/column products. The
		// second factor is determined from the already scaled dense entries.
		rf, cf := []float64{1, 1}, []float64{1, 1}
		rowFactors := func() {
			for i := range rows {
				v0, v1 := math.Abs(rows[i][0]*cf[0]), math.Abs(rows[i][1]*cf[1])
				rf[i] = 1 / math.Sqrt(math.Min(v0, v1)*math.Max(v0, v1))
			}
		}
		colFactors := func() {
			for j := range rows[0] {
				v0, v1 := math.Abs(rows[0][j]*rf[0]), math.Abs(rows[1][j]*rf[1])
				cf[j] = 1 / math.Sqrt(math.Min(v0, v1)*math.Max(v0, v1))
			}
		}
		if rr > cr {
			rowFactors()
			colFactors()
		} else {
			colFactors()
			rowFactors()
		}
		if err := m.ScaleOnceForRatio(); err != nil {
			t.Fatal(err)
		}
		for i, row := range m.A.Rows {
			for _, c := range row {
				lpScalerNear(t, c.Value, rows[i][c.Column]*rf[i]*cf[c.Column])
			}
		}
		lpScalerNear(t, (*m.B)[0], 7*rf[0])
		lpScalerNear(t, (*m.B)[1], -9*rf[1])
		for j := range cf {
			lpScalerNear(t, (*m.ColumnScale)[j], cf[j])
		}
	}
	// Exact strict cutoff: a geometric mean of .01 is NOT skipped.
	m := lpScalerFixture(t, [][]float64{{.01, 0, 0}, {0, .005, 0}, {0, 0, 10}}, []float64{1, 2, 3})
	if err := m.ScaleRowsWithGeometricMean(); err != nil {
		t.Fatal(err)
	}
	lpScalerNear(t, m.A.Rows[0][0].Value, 1)
	lpScalerNear(t, (*m.B)[0], 100)
	lpScalerNear(t, m.A.Rows[1][0].Value, .005)
	lpScalerNear(t, (*m.B)[1], 2)
	m = lpScalerFixture(t, [][]float64{{.01, .005, 10, 20, 0}}, []float64{2})
	if err := m.ScaleColumnsWithGeometricMean(); err != nil {
		t.Fatal(err)
	}
	for j, want := range []float64{100, 1, .1, 1, 1} {
		lpScalerNear(t, (*m.ColumnScale)[j], want)
	}
	for j, want := range []float64{1, .005, 1, 20} {
		lpScalerNear(t, m.A.Rows[0][j].Value, want)
	}
}

func TestLPFloatScalerPowerOfTwoAndBorrowedInputs(t *testing.T) {
	m := lpScalerFixture(t, [][]float64{{.125, 0, 0}, {0, 4, 0}, {0, 0, .75}, {0, 0, 0}}, []float64{.25, -8, 0, 0})
	if err := m.ScaleRows(); err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{.25, .5, .75} {
		lpScalerNear(t, m.A.Rows[i][0].Value, want)
	}
	for i, want := range []float64{.5, -1, 0, 0} {
		lpScalerNear(t, (*m.B)[i], want)
	}
	if err := m.ScaleColumns(); err != nil {
		t.Fatal(err)
	}
	lpScalerNear(t, (*m.ColumnScale)[0], 2)
	// A later power scaling ASSIGNS, it does not accumulate previous factors.
	*m.Minimum = 1
	*m.Maximum = 2
	if err := m.ScaleColumn(0); err != nil {
		t.Fatal(err)
	}
	lpScalerNear(t, (*m.ColumnScale)[0], 2)
	lpScalerNear(t, m.A.Rows[0][0].Value, 1)
	// Source references observe replaced slice headers and mutated thresholds.
	b := []float64{8, 0, 0, 0}
	*m.B = b
	if err := m.ScaleRow(0); err != nil {
		t.Fatal(err)
	}
	lpScalerNear(t, b[0], 2)
	lpScalerNear(t, m.A.Rows[0][0].Value, .25)
	if err := m.ScaleRow(99); err != ErrBounds {
		t.Fatal(err)
	}
	if err := m.ScaleColumn(99); err != ErrBounds {
		t.Fatal(err)
	}
}

func TestLPFloatScalerSourceStopsAndPrefixes(t *testing.T) {
	m := lpScalerFixture(t, [][]float64{{2, 4}}, []float64{6})
	m.Settings.ScaleWithRatio = false
	if ok, err := m.Scale(); ok || err != ErrLPBalanceNotImplemented {
		t.Fatal(ok, err)
	}
	lpScalerNear(t, m.A.Rows[0][0].Value, 2)
	lpScalerNear(t, (*m.B)[0], 6)
	m = lpScalerFixture(t, [][]float64{{0, 0}, {0, 0}}, []float64{.125, 8})
	m.Settings.ScaleWithRatio = false
	if ok, err := m.Scale(); !ok || err != nil {
		t.Fatal(ok, err)
	}
	lpScalerNear(t, (*m.B)[0], .5)
	lpScalerNear(t, (*m.B)[1], 1)
	// Row product publishes before a short RHS access; columns likewise before
	// the final scale-vector write. This is deliberately not transactional.
	for _, op := range []string{"row-max", "row-gm", "column-max", "column-power"} {
		m = lpScalerFixture(t, [][]float64{{4}}, nil)
		var err error
		switch op {
		case "row-max":
			err = m.BringRowMaximumsToOne()
		case "row-gm":
			err = m.ScaleRowsWithGeometricMean()
		case "column-max":
			*m.ColumnScale = nil
			err = m.BringColumnMaximumsToOne()
		case "column-power":
			*m.ColumnScale = nil
			err = m.ScaleColumn(0)
		}
		if err != ErrBounds {
			t.Fatal(op, err)
		}
		lpScalerNear(t, m.A.Rows[0][0].Value, 1)
	}
	m = lpScalerFixture(t, [][]float64{{math.Inf(1)}}, []float64{1})
	if err := m.ScaleRow(0); err != ErrLPScalingStalled {
		t.Fatal(err)
	}
	if err := m.ScaleColumn(0); err != ErrLPScalingStalled {
		t.Fatal(err)
	}
	lpScalerNear(t, (*m.B)[0], 1)
	m = lpScalerFixture(t, [][]float64{{1}}, []float64{1})
	*m.Maximum = -1
	if err := m.ScaleColumn(0); err != ErrLPScalingStalled {
		t.Fatal(err)
	}
	// Selected source debug assertions; release keeps the source arithmetic.
	m = lpScalerFixture(t, [][]float64{{0}}, []float64{0})
	v, err := m.ARatio()
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil || !math.IsNaN(v) {
		t.Fatal(v, err)
	}
	err = m.ScaleRowsWithGeometricMean()
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	_, err = m.MaxRatioOnRows()
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	cs := []float64{7, 8}
	b := []float64{0}
	a := NewFloatStaticMatrix(1, 3)
	s, _ := NewSettings()
	_, err = NewFloatScaler(&b, a, &s.ScalingMinimum, &s.ScalingMaximum, &cs, s)
	if sourceDebugEnabled {
		if err != ErrAssertion || len(cs) != 2 {
			t.Fatal(cs, err)
		}
	} else if err != nil || len(cs) != 3 || cs[0] != 7 || cs[1] != 8 || cs[2] != 1 {
		t.Fatal(cs, err)
	}
}

func TestLPFloatScalerOriginalRatioAndRepetitionLimit(t *testing.T) {
	// A single pass followed by normalization has exact dyadic results.
	m := lpScalerFixture(t, [][]float64{{1, 64, 2}, {16, 2, 32}, {4, 8, 1}}, []float64{1, 2, 3})
	m.Settings.RepsInScaler = 0
	if ok, err := m.Scale(); !ok || err != nil {
		t.Fatal(ok, err)
	}
	for j, want := range []float64{.5, .125, .25} {
		lpScalerNear(t, (*m.ColumnScale)[j], want)
	}
	for i, want := range []float64{.125, .25, 1.5} {
		lpScalerNear(t, (*m.B)[i], want)
	}
	// Independently evaluated dense diagonal products for five passes. Updating
	// the comparison ratio each pass incorrectly stops after four on this input.
	m = lpScalerFixture(t, [][]float64{{.125, 8, .25}, {8, 2, 32}, {.125, 1, 32}}, []float64{1, 2, 3})
	m.Settings.RepsInScaler = 4
	if ok, err := m.Scale(); !ok || err != nil {
		t.Fatal(ok, err)
	}
	for j, want := range []float64{.805245165974627, .10065564574682842, .0963881765879963} {
		lpScalerNear(t, (*m.ColumnScale)[j], want)
	}
	for i, want := range []float64{1.2418578120734842, .31046445301837106, .9726296659882573} {
		lpScalerNear(t, (*m.B)[i], want)
	}
}

func TestLPFloatScalerZeroToleranceIEEEAndResize(t *testing.T) {
	m := lpScalerFixture(t, [][]float64{{1e-13, 0}, {0, 0}}, []float64{2, 3})
	if err := m.BringRowMaximumsToOne(); err != nil {
		t.Fatal(err)
	}
	if err := m.BringColumnMaximumsToOne(); err != nil {
		t.Fatal(err)
	}
	lpScalerNear(t, (*m.B)[0], 2)
	lpScalerNear(t, (*m.ColumnScale)[0], 1)
	if v, err := m.MaxRatioOnColumns(); v != 1 || err != nil {
		t.Fatal(v, err)
	}
	// The outer std::min(t, previous) has order-sensitive NaN behavior.
	for _, row := range [][]float64{{1, math.NaN()}, {math.NaN(), 1}} {
		m = lpScalerFixture(t, [][]float64{{row[0]}, {row[1]}}, []float64{0, 0})
		v, err := m.AMin()
		if err != nil {
			t.Fatal(err)
		}
		if math.IsNaN(row[1]) {
			if !math.IsNaN(v) {
				t.Fatal(v)
			}
		} else if v != 1 {
			t.Fatal(v)
		}
	}
	m = lpScalerFixture(t, [][]float64{{math.NaN()}}, []float64{0})
	if err := m.ScaleColumnsWithGeometricMean(); err != nil {
		t.Fatal(err)
	}
	if !math.IsNaN((*m.ColumnScale)[0]) || !math.IsNaN(m.A.Rows[0][0].Value) {
		t.Fatal("source NaN propagation")
	}
	// Unlike the later scale write, geometric scaling reads the scale first.
	m = lpScalerFixture(t, [][]float64{{4}}, []float64{1})
	*m.ColumnScale = nil
	if err := m.ScaleColumnsWithGeometricMean(); err != ErrBounds {
		t.Fatal(err)
	}
	lpScalerNear(t, m.A.Rows[0][0].Value, 4)
	m = lpScalerFixture(t, [][]float64{{0}}, nil)
	if err := m.ScaleRows(); err != ErrBounds {
		t.Fatal(err)
	}
	m.Settings.ScaleWithRatio = false
	if ok, err := m.Scale(); ok || err != ErrBounds {
		t.Fatal(ok, err)
	}
	if !sourceDebugEnabled {
		cs := []float64{7, 8}
		b := []float64{0}
		a := NewFloatStaticMatrix(1, 1)
		s, _ := NewSettings()
		if _, err := NewFloatScaler(&b, a, &s.ScalingMinimum, &s.ScalingMaximum, &cs, s); err != nil || len(cs) != 1 || cs[0] != 7 {
			t.Fatal(cs, err)
		}
	}
}

func TestLPFloatScalerComposedFailurePrefixes(t *testing.T) {
	for _, rows := range [][][]float64{{{2, 32}, {4, 8}}, {{2, 4}, {32, 8}}} {
		for _, op := range []string{"once-rhs", "once-scale", "ratio-rhs", "normalize-rhs", "normalize-scale"} {
			m := lpScalerFixture(t, rows, []float64{7, 9})
			var err error
			switch op {
			case "once-rhs":
				*m.B = nil
				err = m.ScaleOnceForRatio()
			case "once-scale":
				*m.ColumnScale = nil
				err = m.ScaleOnceForRatio()
			case "ratio-rhs":
				*m.B = nil
				_, err = m.Scale()
			case "normalize-rhs":
				*m.B = nil
				err = m.BringRowsAndColumnsMaximumsToOne()
			case "normalize-scale":
				*m.ColumnScale = nil
				err = m.BringRowsAndColumnsMaximumsToOne()
			}
			if err != ErrBounds {
				t.Fatal(op, rows, err)
			}
		}
	}
	m := lpScalerFixture(t, [][]float64{{math.Inf(1)}}, []float64{0})
	if err := m.ScaleColumns(); err != ErrLPScalingStalled {
		t.Fatal(err)
	}
	m = lpScalerFixture(t, [][]float64{{0}}, []float64{0})
	ok, err := m.Scale()
	if sourceDebugEnabled {
		if ok || err != ErrAssertion {
			t.Fatal(ok, err)
		}
	} else if !ok || err != nil {
		t.Fatal(ok, err)
	}
}

// Finite extreme inputs distinguish the source's literal max*min geometric
// product from a mathematically equivalent overflow-safe geometric mean.
func TestLPFloatScalerGeometricProductExtremes(t *testing.T) {
	for _, value := range []float64{math.SmallestNonzeroFloat64, 1e-200, 1e200, math.MaxFloat64} {
		m := lpScalerFixture(t, [][]float64{{value}}, []float64{-value})
		if err := m.ScaleRowsWithGeometricMean(); err != nil {
			t.Fatal(value, err)
		}
		if value < 1 {
			// The positive product underflows to zero; strict .01 skips it.
			if m.A.Rows[0][0].Value != value || (*m.B)[0] != -value {
				t.Fatal("underflow must skip row", value, m.A.Rows, *m.B)
			}
		} else {
			// The product overflows to infinity. Row multiplication by zero
			// and RHS division by infinity retain the negative RHS zero.
			if m.A.Rows[0][0].Value != 0 || (*m.B)[0] != 0 || !math.Signbit((*m.B)[0]) {
				t.Fatal("overflow row publication", value, m.A.Rows, *m.B)
			}
		}
		m = lpScalerFixture(t, [][]float64{{value}}, []float64{-value})
		if err := m.ScaleColumnsWithGeometricMean(); err != nil {
			t.Fatal(value, err)
		}
		// Underflow skips at the denominator cutoff; overflow instead yields
		// candidate column scale zero and skips at the separate .1 cutoff.
		if m.A.Rows[0][0].Value != value || (*m.B)[0] != -value || (*m.ColumnScale)[0] != 1 {
			t.Fatal("column cutoff before publication", value, m.A.Rows, *m.B, *m.ColumnScale)
		}
	}
}

func TestLPFloatScalerExtremeDyadicRowsAndColumns(t *testing.T) {
	for exponent := -1000; exponent <= 1000; exponent += 25 {
		maximum := math.Ldexp(.75, exponent)
		m := lpScalerFixture(t, [][]float64{{maximum / 2}}, []float64{-maximum})
		*m.Minimum, *m.Maximum = .5, 1
		if err := m.ScaleRow(0); err != nil {
			t.Fatal(exponent, err)
		}
		// RHS, rather than A, determines the row multiplier. All numbers
		// remain exactly dyadic; no implementation loop is used as an oracle.
		if m.A.Rows[0][0].Value != .375 || (*m.B)[0] != -.75 || (*m.ColumnScale)[0] != 1 {
			t.Fatal("row dyadic oracle", exponent, m.A.Rows, *m.B, *m.ColumnScale)
		}
		m = lpScalerFixture(t, [][]float64{{maximum}}, []float64{-maximum})
		*m.Minimum, *m.Maximum = .5, 1
		if err := m.ScaleColumn(0); err != nil {
			t.Fatal(exponent, err)
		}
		if m.A.Rows[0][0].Value != .75 || (*m.ColumnScale)[0] != math.Ldexp(1, -exponent) || (*m.B)[0] != -maximum {
			t.Fatal("column dyadic oracle", exponent, m.A.Rows, *m.B, *m.ColumnScale)
		}
	}
	for _, boundary := range []float64{.5, 1} {
		m := lpScalerFixture(t, [][]float64{{boundary}}, []float64{-boundary})
		*m.Minimum, *m.Maximum = .5, 1
		if err := m.ScaleRow(0); err != nil {
			t.Fatal(err)
		}
		if err := m.ScaleColumn(0); err != nil {
			t.Fatal(err)
		}
		if m.A.Rows[0][0].Value != boundary || (*m.B)[0] != -boundary || (*m.ColumnScale)[0] != 1 {
			t.Fatal("inclusive endpoints", boundary)
		}
	}
}

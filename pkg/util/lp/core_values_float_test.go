package lp

import (
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func lpFloatValueCore(t *testing.T, rows, cols uint32) *FloatCoreSolverBase {
	t.Helper()
	in := lpFloatCoreInput(t, rows, cols)
	in.Settings.SimplexStrategy = StrategyLU
	for i := uint32(0); i < rows; i++ {
		if err := in.A.Set(i, i, 1); err != nil {
			t.Fatal(err)
		}
	}
	m, err := NewFloatCoreSolverBase(in)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func TestLPFloatCoreValueReconstructionIndependentOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(771902))
	for world := 0; world < 100; world++ {
		r, n := 1+rng.Intn(8), 10+rng.Intn(8)
		in := lpFloatCoreInput(t, uint32(r), uint32(n))
		in.Settings.SimplexStrategy = StrategyLU
		if world%2 == 0 {
			in.Settings.DensityThreshold = 0
		} else {
			in.Settings.DensityThreshold = math.Inf(1)
		}
		order := rng.Perm(n)
		for i := 0; i < r; i++ {
			(*in.Basis)[i] = uint32(order[i])
		}
		known := make([]float64, n)
		for j := range known {
			known[j] = float64(rng.Intn(17)-8) / 4
			(*in.X)[j] = known[j]
		}
		for i := 0; i < r; i++ {
			for j := 0; j < n; j++ {
				v := float64(rng.Intn(5) - 2)
				for k := 0; k < r; k++ {
					if j == order[k] {
						v = 0
						if k == i {
							v = float64(i + 1)
						}
					}
				}
				if err := in.A.Set(uint32(i), uint32(j), v); err != nil {
					t.Fatal(err)
				}
				(*in.B)[i] += float64(v * known[j])
			}
			(*in.X)[order[i]] = 99
		}
		m, err := NewFloatCoreSolverBase(in)
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := m.FindXBySolving(); !ok || err != nil {
			t.Fatal(world, ok, err)
		}
		for j, v := range *in.X {
			if math.Abs(v-known[j]) > 1e-10 {
				t.Fatal(world, j, v, known[j])
			}
		}
		// In-place value changes and inverse restoration use actual solved Ed.
		enter := (*in.NBasis)[0]
		if err := m.SolveBd(enter); err != nil {
			t.Fatal(err)
		}
		before := append([]float64(nil), (*in.X)...)
		if err := m.UpdateX(enter, .25); err != nil {
			t.Fatal(err)
		}
		if bad, err := m.AMultXIsOff(); bad || err != nil {
			t.Fatal("nullspace update", world, bad, err)
		}
		if err := m.RestoreX(enter, .25); err != nil {
			t.Fatal(err)
		}
		for j, v := range *in.X {
			if math.Abs(v-before[j]) > 1e-10 {
				t.Fatal("restore", world, j)
			}
		}
		m.Close()
	}
}

func TestLPFloatCoreValueLiteralCorrectionAndTableau(t *testing.T) {
	m := lpFloatValueCore(t, 1, 2)
	(*m.B)[0] = 4
	// A real source coefficient update leaves the old factorization stale.
	// B_old=1, B_current=2: first solve=4, residual=-4, second solve=-4,
	// source subtraction gives8. The retained code does not add the residual.
	if err := m.A.MultiplyRow(0, 2); err != nil {
		t.Fatal(err)
	}
	if err := m.SolveAxEqualsB(); err != nil || (*m.X)[0] != 8 {
		t.Fatal("literal correction sign", *m.X, err)
	}
	if bad, err := m.AMultXIsOff(); !bad || err != nil {
		t.Fatal(bad, err)
	}
	if err := m.InitLU(); err != nil {
		t.Fatal(err)
	}
	if ok, err := m.FindXBySolving(); !ok || err != nil || (*m.X)[0] != 2 {
		t.Fatal(ok, err, *m.X)
	}
	for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts} {
		m := lpFloatValueCore(t, 2, 3)
		m.Settings.SimplexStrategy = strategy
		if err := m.A.Set(0, 2, 2); err != nil {
			t.Fatal(err)
		}
		if err := m.A.Set(1, 2, -3); err != nil {
			t.Fatal(err)
		}
		*m.X = []float64{7, 8, 9}
		m.CopyXB = []float64{100, 200}
		m.Ed.Index = []uint32{99}
		if err := m.UpdateX(2, .5); err != nil || !reflect.DeepEqual(*m.X, []float64{6, 9.5, 9.5}) || !reflect.DeepEqual(m.CopyXB, []float64{100, 200}) {
			t.Fatal("tableau column update", *m.X, err)
		}
	}
	// Restore zero is an early return before any entering/scratch access.
	if err := m.RestoreX(99, math.Copysign(0, -1)); err != nil {
		t.Fatal(err)
	}
}

func TestLPFloatCoreResidualToleranceAndIEEE(t *testing.T) {
	m := lpFloatValueCore(t, 2, 3)
	m.Settings.RefactorTolerance = .125
	*m.B = []float64{10, 0}
	*m.X = []float64{9.75, 0, 0}
	if bad, err := m.AMultXIsOff(); bad || err != nil {
		t.Fatal("strict threshold", bad, err)
	}
	(*m.X)[0] = math.Nextafter(9.75, math.Inf(-1))
	if bad, err := m.AMultXIsOff(); !bad || err != nil {
		t.Fatal(bad, err)
	}
	if bad, err := m.AMultXIsOffOnIndex([]uint32{1}); bad || err != nil {
		t.Fatal("selected rows only", bad, err)
	}
	if bad, err := m.AMultXIsOffOnIndex([]uint32{1, 0}); !bad || err != nil {
		t.Fatal(bad, err)
	}
	(*m.X)[0] = math.NaN()
	if bad, err := m.AMultXIsOff(); bad || err != nil {
		t.Fatal("source NaN comparison", bad, err)
	}
	(*m.B)[0], (*m.X)[0] = math.Inf(1), 0
	if bad, err := m.AMultXIsOff(); bad || err != nil {
		t.Fatal("Inf > Inf is false", bad, err)
	}
	*m.X = nil
	_, err := m.AMultXIsOffOnIndex(nil)
	if sourceDebugEnabled {
		if !errors.Is(err, ErrAssertion) {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
}

func TestLPFloatCoreValuePublicationGuards(t *testing.T) {
	short := lpFloatValueCore(t, 1, 1)
	*short.X = nil
	if err := short.SolveAxEqualsB(); !errors.Is(err, ErrBounds) {
		t.Fatal("first solve before basic-copy domain", err)
	}
	m := lpFloatValueCore(t, 2, 3)
	*m.B = []float64{3, 5}
	*m.Basis = []uint32{99, 1}
	rs := []float64{7, 11}
	if err := m.CopyRsToXB(rs); !errors.Is(err, ErrBounds) || (*m.X)[1] != 11 {
		t.Fatal("descending copy prefix", *m.X, err)
	}
	if err := m.AddDeltaToXB(rs); !errors.Is(err, ErrBounds) || (*m.X)[1] != 0 {
		t.Fatal("descending correction prefix", *m.X, err)
	}
	*m.Basis = []uint32{0, 1}
	if err := m.RsMinusAnX(nil); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.FindErrorInBxB(nil); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.CopyRsToXB(nil); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.AddDeltaToXB(nil); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*m.Basis = nil
	if err := m.CopyRsToXB(rs); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.AddDeltaToXB(rs); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*m.Basis = []uint32{0, 1}
	m.Factorization.Close()
	if err := m.SolveAxEqualsB(); !errors.Is(err, ErrLUClosed) {
		t.Fatal(err)
	}
	if ok, err := m.FindXBySolving(); ok || !errors.Is(err, ErrLUClosed) {
		t.Fatal(ok, err)
	}
	m.Close()
	if err := m.SolveAxEqualsB(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.UpdateX(99, 1); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.RestoreX(99, 1); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	m.Ed.Index = []uint32{0, 9}
	m.Ed.Data = []float64{2, 0}
	*m.X = []float64{7, 8, 9}
	if err := m.UpdateX(2, 1); !errors.Is(err, ErrBounds) || !reflect.DeepEqual(*m.X, []float64{5, 8, 10}) || m.CopyXB[0] != 7 {
		t.Fatal("indexed update prefix", *m.X, err)
	}
	if err := m.RestoreX(2, 1); !errors.Is(err, ErrBounds) || !reflect.DeepEqual(*m.X, []float64{7, 8, 9}) {
		t.Fatal("indexed restore prefix", *m.X, err)
	}
	m.Ed.Index = []uint32{0}
	m.Ed.Data = nil
	if err := m.UpdateX(2, 1); !errors.Is(err, ErrBounds) || m.CopyXB[0] != 7 || (*m.X)[2] != 10 {
		t.Fatal("copy before Ed access", err)
	}
	m.CopyXB = nil
	if err := m.UpdateX(2, 1); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.RestoreX(2, 1); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*m.B = nil
	if _, err := m.AMultXIsOff(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*m.B = []float64{0, 0, 0}
	if _, err := m.AMultXIsOffOnIndex([]uint32{99}); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	// Native truncations qualify source reference/index domains while keeping
	// the last row's completed publication visible.
	*m.Heading = nil
	if err := m.RsMinusAnX(rs); !errors.Is(err, ErrBounds) || rs[1] != 0 {
		t.Fatal(err, rs)
	}
	if err := m.FindErrorInBxB(rs); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*m.Heading = []int32{0, -1, -2}
	*m.X = nil
	if err := m.RsMinusAnX(rs); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*m.Heading = []int32{0, 1, -1}
	if err := m.FindErrorInBxB(rs); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.SolveAxEqualsB(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if _, err := m.AMultXIsOff(); (sourceDebugEnabled && !errors.Is(err, ErrAssertion)) || (!sourceDebugEnabled && !errors.Is(err, ErrBounds)) {
		t.Fatal(err)
	}
	*m.X = []float64{0, 0, 0}
	m.Settings.SimplexStrategy = StrategyTableauRows
	*m.Basis = nil
	if err := m.A.Set(0, 2, 1); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateX(2, 1); !errors.Is(err, ErrBounds) || (*m.X)[2] != 1 {
		t.Fatal(err, *m.X)
	}
	*m.Basis = []uint32{99, 1}
	if err := m.UpdateX(2, 1); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
}

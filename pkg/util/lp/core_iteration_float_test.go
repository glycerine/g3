package lp

import (
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestLPFloatCoreBasisChangeIndependentStateMachine(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	rng := rand.New(rand.NewSource(717181))
	for world := 0; world < 80; world++ {
		in := lpFloatCoreInput(t, 4, 13)
		m, err := NewFloatCoreSolverBase(in)
		if err != nil {
			t.Fatal(err)
		}
		basis := append([]uint32(nil), (*in.Basis)...)
		non := append([]uint32(nil), (*in.NBasis)...)
		m.StartTracingBasisChanges()
		var trace []uint32
		for step := 0; step < 200; step++ {
			beforeTrace := append([]uint32(nil), trace...)
			bi, ni := rng.Intn(len(basis)), rng.Intn(len(non))
			enter, leave := non[ni], basis[bi]
			if step%2 == 0 {
				err = m.ChangeBasis(enter, leave)
			} else {
				err = m.ChangeBasisUnconditionally(enter, leave)
			}
			if err != nil {
				t.Fatal(err)
			}
			basis[bi], non[ni] = enter, leave
			if len(trace) >= 2 && trace[len(trace)-2] == leave && trace[len(trace)-1] == enter {
				trace = trace[:len(trace)-2]
			} else {
				trace = append(trace, enter, leave)
			}
			if !reflect.DeepEqual(basis, *in.Basis) || !reflect.DeepEqual(non, *in.NBasis) || !reflect.DeepEqual(trace, m.Trace) {
				t.Fatal(world, step, "basis/trace state")
			}
			for i, j := range basis {
				if (*in.Heading)[j] != int32(i) {
					t.Fatal("basic inverse")
				}
			}
			for i, j := range non {
				if (*in.Heading)[j] != -int32(i)-1 {
					t.Fatal("nonbasic inverse")
				}
			}
			if step%5 == 0 {
				if err := m.RestoreBasisChange(enter, leave); err != nil {
					t.Fatal(err)
				}
				basis[bi], non[ni] = leave, enter
				trace = beforeTrace
				if len(trace) != len(m.Trace) || (len(trace) > 0 && !reflect.DeepEqual(trace, m.Trace)) {
					t.Fatal("reverse cancels trace")
				}
				if err := m.RestoreBasisChange(enter, leave); err != nil {
					t.Fatal("unchanged restore", err)
				}
			}
			if ok, err := m.BasisHeadingIsCorrect(); !ok || err != nil {
				t.Fatal(ok, err)
			}
		}
		m.StopTracingBasisChanges()
		old := append([]uint32(nil), m.Trace...)
		if err := m.ChangeBasis(non[0], basis[0]); err != nil || !reflect.DeepEqual(old, m.Trace) {
			t.Fatal("disabled trace", err)
		}
		m.StartTracingBasisChanges()
		if len(m.Trace) != 0 || !m.Tracing {
			t.Fatal("restart clears trace")
		}
	}
}

func TestLPFloatCoreReducedCostsAndEnteringColumnIndependentOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(981727))
	for world := 0; world < 64; world++ {
		r := 2 + rng.Intn(6)
		in := lpFloatCoreInput(t, uint32(r), uint32(3*r))
		in.Settings.SimplexStrategy = StrategyLU
		if world%2 == 0 {
			in.Settings.DensityThreshold = 0
		} else {
			in.Settings.DensityThreshold = math.Inf(1)
		}
		base := make([][]float64, r)
		for i := range base {
			base[i] = make([]float64, r)
			for k := range base[i] {
				v := float64(rng.Intn(5) - 2)
				if k == i {
					v = 16
				}
				base[i][k] = v
				for copy := 0; copy < 3; copy++ {
					if err := in.A.Set(uint32(i), uint32(3*k+copy), v*float64(copy+1)); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		order := rng.Perm(r)
		for i, k := range order {
			(*in.Basis)[i] = uint32(3 * k)
		}
		m, err := NewFloatCoreSolverBase(in)
		if err != nil {
			t.Fatal(world, err)
		}
		for round := 0; round < 6; round++ {
			wantY := make([]float64, r)
			for i := range wantY {
				wantY[i] = float64(rng.Intn(9) - 4)
			}
			wantD := make([]float64, 3*r)
			for j := range *in.Costs {
				var c float64
				for i := 0; i < r; i++ {
					c += float64(wantY[i] * base[i][j/3] * float64(j%3+1))
				}
				if (*in.Heading)[j] < 0 {
					wantD[j] = float64(rng.Intn(7) - 3)
					c += wantD[j]
				}
				(*in.Costs)[j] = c
			}
			if err := m.InitReducedCostsForOneIteration(); err != nil {
				t.Fatal(world, round, err)
			}
			for i, y := range m.Y {
				if math.Abs(y-wantY[i]) > 1e-10 {
					t.Fatal("independent cB=YB", world, round, i, y, wantY[i])
				}
			}
			for j, d := range m.D {
				if math.Abs(d-wantD[j]) > 1e-9 {
					t.Fatal("independent c-YA", world, round, j, d, wantD[j])
				}
			}
			bi := rng.Intn(r)
			leaving := (*in.Basis)[bi]
			entering := 3*(leaving/3) + (leaving%3+1)%3
			if err := m.SolveBd(entering); err != nil {
				t.Fatal(err)
			}
			for i, d := range m.Ed.Data {
				want := 0.
				if i == bi {
					want = float64(entering%3+1) / float64(leaving%3+1)
				}
				if math.Abs(d-want) > 1e-9 {
					t.Fatal("independent Bd=Aenter", world, round, i, d, want)
				}
			}
			// Bookkeeping changes do not silently change the numerical owner.
			old := m.Factorization
			if err := m.ChangeBasis(entering, leaving); err != nil || m.Factorization != old {
				t.Fatal(err)
			}
			if err := m.InitLU(); err != nil || !old.closed {
				t.Fatal("actual refactor", err)
			}
		}
		if err := m.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLPFloatCoreIterationPublicationPrefixes(t *testing.T) {
	in := lpFloatCoreInput(t, 2, 4)
	m, err := NewFloatCoreSolverBase(in)
	if err != nil {
		t.Fatal(err)
	}
	*in.Costs = []float64{10, 20, 30, 40}
	y := []float64{9}
	if err := m.FillCB(y); !errors.Is(err, ErrBounds) || y[0] != 10 {
		t.Fatal("fill prefix", y, err)
	}
	y = []float64{9, 9}
	if err := m.SolveYB(y); !errors.Is(err, ErrBounds) || !reflect.DeepEqual(y, []float64{10, 20}) {
		t.Fatal("costs before nil LU", y, err)
	}
	if err := m.InitReducedCostsForOneIteration(); !errors.Is(err, ErrBounds) || !reflect.DeepEqual(m.Y, []float64{10, 20}) {
		t.Fatal(err)
	}
	if err := m.SolveBd(2); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*in.Basis = []uint32{0}
	if err := m.FillCB(y); !errors.Is(err, ErrBounds) || y[0] != 10 {
		t.Fatal(err)
	}
	if err := m.SolveYB(y); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*in.Basis = []uint32{0, 9}
	if err := m.FillCB(y); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*in.Basis = []uint32{0, 1}
	// Descending cost initialization publishes high columns before failure.
	m.D = []float64{9, 9, 9, 9}
	*in.Heading = []int32{0, 1, -1, -2}
	*in.Costs = []float64{10, 20, 30, 40}
	m.Y = []float64{2, 3}
	if err := in.A.Set(1, 2, 5); err != nil {
		t.Fatal(err)
	}
	if err := in.A.Set(0, 3, 7); err != nil {
		t.Fatal(err)
	}
	if err := m.FillReducedCostsFromYByRows(); err != nil || !reflect.DeepEqual(m.D, []float64{0, 0, 15, 26}) {
		t.Fatal(m.D, err)
	}
	// Basic costs are not read in the descending initialization.
	*in.Costs = nil
	*in.Heading = []int32{0, 1, 2, 3}
	m.Y = []float64{0, math.Copysign(0, -1)}
	if err := m.FillReducedCostsFromYByRows(); err != nil || !reflect.DeepEqual(m.D, []float64{0, 0, 0, 0}) {
		t.Fatal(m.D, err)
	}
	*in.Costs = []float64{10, 20, 30, 40}
	*in.Heading = []int32{0, 1, -1, -2}
	m.Y = nil
	if err := m.FillReducedCostsFromYByRows(); !errors.Is(err, ErrBounds) || !reflect.DeepEqual(m.D, []float64{0, 0, 30, 40}) {
		t.Fatal(m.D, err)
	}
	m.Y = []float64{math.NaN(), 0}
	if err := m.FillReducedCostsFromYByRows(); err != nil || !math.IsNaN(m.D[3]) || m.D[2] != 30 {
		t.Fatal("NaN and zero routing", m.D, err)
	}
	m.D = nil
	if err := m.FillReducedCostsFromYByRows(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*in.Heading = []int32{0, 1, 2, 3}
	if err := m.FillReducedCostsFromYByRows(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*in.Heading = nil
	if err := m.FillReducedCostsFromYByRows(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
}

func TestLPFloatCoreIterationStoppedChildAndLiteralAccumulation(t *testing.T) {
	in := lpFloatCoreInput(t, 1, 2)
	if err := in.A.Set(0, 0, 1); err != nil {
		t.Fatal(err)
	}
	in.Settings.SimplexStrategy = StrategyLU
	m, err := NewFloatCoreSolverBase(in)
	if err != nil {
		t.Fatal(err)
	}
	(*in.Costs)[0] = 17
	m.Factorization.Close()
	if err := m.SolveYB(m.Y); !errors.Is(err, ErrLUClosed) || m.Y[0] != 17 {
		t.Fatal("cost prefix before stopped child", err, m.Y)
	}
	if err := m.SolveBd(1); !errors.Is(err, ErrLUClosed) {
		t.Fatal(err)
	}
	if err := m.InitLU(); err != nil {
		t.Fatal(err)
	}
	if err := m.SolveBd(99); err == nil {
		t.Fatal("child column domain")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	in = lpFloatCoreInput(t, 3, 4)
	m, err = NewFloatCoreSolverBase(in)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range []float64{1e16, -1e16, 1} {
		if err := in.A.Set(uint32(i), 3, v); err != nil {
			t.Fatal(err)
		}
		m.Y[i] = 1
	}
	if err := m.FillReducedCostsFromYByRows(); err != nil || m.D[3] != 0 {
		t.Fatal("descending row arithmetic", m.D, err)
	}
	// Duplicate row entries are processed in insertion order, not coalesced.
	in.A.Clear()
	if err := in.A.InitRowColumns(3, 4); err != nil {
		t.Fatal(err)
	}
	for _, v := range []float64{1e16, -1e16, 1} {
		if err := in.A.Set(2, 3, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.FillReducedCostsFromYByRows(); err != nil || m.D[3] != -1 {
		t.Fatal("literal duplicate order", m.D, err)
	}
	m.Y = []float64{0, 0, 0}
	(*in.Costs)[3] = math.Copysign(0, -1)
	if err := m.FillReducedCostsFromYByRows(); err != nil || !math.Signbit(m.D[3]) {
		t.Fatal("zero multiplier preserves negative-zero cost", err, m.D)
	}
}

func TestLPFloatCoreBasisChangeSourceGuardsAndDetachedColumn(t *testing.T) {
	in := lpFloatCoreInput(t, 1, 3)
	m, err := NewFloatCoreSolverBase(in)
	if err != nil {
		t.Fatal(err)
	}
	// LAR removes some nonbasics; the unconditional method restores a missing
	// entering slot before looking up the leaving column.
	*in.NBasis = nil
	m.StartTracingBasisChanges()
	if err := m.ChangeBasisUnconditionally(2, 0); err != nil || !reflect.DeepEqual(*in.Basis, []uint32{2}) || !reflect.DeepEqual(*in.NBasis, []uint32{0}) || !reflect.DeepEqual(*in.Heading, []int32{-1, -2, 0}) {
		t.Fatal(err, *in.Basis, *in.NBasis, *in.Heading)
	}
	if err := m.RestoreBasisChange(2, 0); err != nil || len(m.Trace) != 0 {
		t.Fatal(err)
	}
	if err := m.ChangeBasis(9, 0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.RestoreBasisChange(9, 0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.ChangeBasis(2, 9); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*in.NBasis = nil
	(*in.Heading)[2] = -3
	if err := m.ChangeBasisUnconditionally(2, 9); !errors.Is(err, ErrBounds) || (*in.Heading)[2] != 0 || !reflect.DeepEqual(*in.NBasis, []uint32{2}) {
		t.Fatal("append before leaving bounds", err, *in.Heading, *in.NBasis)
	}
	if err := m.AllocateBasisHeading(); err != nil {
		t.Fatal(err)
	}
	(*in.Heading)[0] = -1
	if err := m.ChangeBasis(2, 0); !errors.Is(err, ErrBounds) || (*in.Heading)[2] != -1 {
		t.Fatal("enter heading before bad basic slot", err, *in.Heading)
	}
	if err := m.AllocateBasisHeading(); err != nil {
		t.Fatal(err)
	}
	*in.NBasis = nil
	if err := m.ChangeBasis(2, 0); !errors.Is(err, ErrBounds) || (*in.Basis)[0] != 2 || (*in.Heading)[0] != -1 {
		t.Fatal("basis before bad nonbasic slot", err, *in.Basis, *in.Heading)
	}
	if err := m.AllocateBasisHeading(); err != nil {
		t.Fatal(err)
	}
	(*in.Heading)[0] = 0
	err = m.ChangeBasis(0, 0)
	if sourceDebugEnabled {
		if !errors.Is(err, ErrAssertion) {
			t.Fatal(err)
		}
	} else if !errors.Is(err, ErrBounds) || (*in.Heading)[0] != 0 {
		t.Fatal(err, *in.Heading)
	}
	if err := m.AllocateBasisHeading(); err != nil {
		t.Fatal(err)
	}
	(*in.Heading)[2] = -1 << 31
	if err := m.ChangeBasis(2, 0); !errors.Is(err, ErrBounds) || (*in.Heading)[2] != -1<<31 {
		t.Fatal("signed unary overflow guard", err)
	}
}

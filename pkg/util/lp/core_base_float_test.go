package lp

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"testing"
)

func lpFloatCoreInput(t *testing.T, rows, cols uint32) FloatCoreInputs {
	t.Helper()
	s, err := NewSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.DebugOut, s.MessageOut = nil, nil
	b, x, costs := make([]float64, rows), make([]float64, cols), make([]float64, cols)
	basis, nb, heading := make([]uint32, rows), []uint32{99}, []int32{88}
	types, low, upper := make([]ColumnType, cols), make([]float64, cols), make([]float64, cols)
	for i := range basis {
		basis[i] = uint32(i)
	}
	return FloatCoreInputs{A: NewFloatStaticMatrix(rows, cols), B: &b, X: &x, Costs: &costs, Basis: &basis, NBasis: &nb, Heading: &heading, Settings: s, ColumnTypes: &types, LowBounds: &low, UpperBounds: &upper}
}

func TestLPFloatCoreConstructionIndependentBasisOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(99812))
	for world := 0; world < 160; world++ {
		r, n := rng.Intn(9), 9+rng.Intn(10)
		in := lpFloatCoreInput(t, uint32(r), uint32(n))
		perm := rng.Perm(n)
		wantHeading := make([]int32, n)
		for j := range wantHeading {
			wantHeading[j] = -1
		}
		for i := 0; i < r; i++ {
			(*in.Basis)[i] = uint32(perm[i])
			wantHeading[perm[i]] = int32(i)
			if err := in.A.Set(uint32(i), uint32(perm[i]), float64(i+1)); err != nil {
				t.Fatal(err)
			}
		}
		var wantNon []uint32
		for j := n - 1; j >= 0; j-- {
			if wantHeading[j] < 0 {
				wantNon = append(wantNon, uint32(j))
				wantHeading[j] = -int32(len(wantNon))
			}
		}
		if world%2 == 0 {
			in.Settings.SimplexStrategy = StrategyLU
		}
		m, err := NewFloatCoreSolverBase(in)
		if err != nil {
			t.Fatal(world, err)
		}
		if !reflect.DeepEqual(*in.Heading, wantHeading) || !reflect.DeepEqual(*in.NBasis, wantNon) {
			t.Fatal(world, *in.Heading, *in.NBasis)
		}
		if m.Status != StatusFeasible || !m.CurrentXIsFeasible() || m.CurrentXIsInfeasible() || m.UsingInfeasCosts || m.TotalIterations != 0 || m.IterationsNoCostGrowth != 0 || m.BasisSortCounter != 0 || m.Tracing || m.PivotedRows != nil || m.LookForFeasibleSolutionOnly || len(m.Trace)+len(m.ColumnsNZ)+len(m.RowsNZ) != 0 {
			t.Fatal("constructor defaults")
		}
		if len(m.Y) != r || len(m.CopyXB) != r || len(m.D) != n || len(m.ColumnNorms) != n || len(m.SteepestEdge) != n || len(m.PivotRow.Data) != n || len(m.PivotRowB1.Data) != r || len(m.W.Data) != r || len(m.Ed.Data) != r {
			t.Fatal("scratch extents")
		}
		if in.Settings.UseLU() {
			if m.Factorization == nil {
				t.Fatal("missing real LU")
			}
			v := make([]float64, r)
			for i := range v {
				v[i] = float64((i + 1) * (i + 3))
			}
			if err := m.Factorization.SolveBy(v); err != nil {
				t.Fatal(err)
			}
			for i, x := range v {
				if math.Abs(x-float64(i+3)) > 1e-10 {
					t.Fatal("real basis solve", i, x)
				}
			}
		} else if m.Factorization != nil {
			t.Fatal("tableau allocated LU")
		}
		ok, err := m.BasisHeadingIsCorrect()
		if !ok || err != nil {
			t.Fatal(ok, err)
		}
		if in.Settings.UseLU() {
			old := m.Factorization
			if err := m.Init(); err != nil || m.Factorization == old || !old.closed {
				t.Fatal("factorization replacement", err)
			}
		}
		// Replacing a caller slice header must be observed, not a stale copy.
		*in.X = append([]float64(nil), (*in.X)...)
		(*in.X)[0] = 123
		if (*m.X)[0] != 123 {
			t.Fatal("borrowed header lost")
		}
		if err := m.Close(); err != nil {
			t.Fatal(err)
		}
		if err := m.Close(); err != nil || m.Factorization != nil {
			t.Fatal(err)
		}
	}
	var absent *FloatCoreSolverBase
	if err := absent.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLPFloatCoreFeasibilityExactDyadicOracle(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	in := lpFloatCoreInput(t, 1, 5)
	in.Settings.PrimalFeasibilityTolerance = .125
	m, err := NewFloatCoreSolverBase(in)
	if err != nil {
		t.Fatal(err)
	}
	// All arithmetic is dyadic and exactly representable; independent rational
	// inequalities specify absolute-plus-relative tolerance without rounding.
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	for bound := -16.; bound <= 16; bound += .5 {
		tol := new(big.Rat).Mul(rat(math.Abs(bound)), rat(.125))
		tol.Add(tol, rat(.125))
		lo, hi := new(big.Rat).Sub(rat(bound), tol), new(big.Rat).Add(rat(bound), tol)
		for x := -20.; x <= 20; x += .0625 {
			below, above := rat(x).Cmp(lo) < 0, rat(x).Cmp(hi) > 0
			if m.BelowBound(x, bound) != below || m.AboveBound(x, bound) != above || m.AtBound(x, bound) != (!below && !above) {
				t.Fatal(x, bound)
			}
		}
	}
	for j := range *in.ColumnTypes {
		(*in.ColumnTypes)[j] = ColumnType(j)
		(*in.LowBounds)[j], (*in.UpperBounds)[j] = -2, 3
	}
	for _, x := range []float64{-10, -2, 0, 3, 10, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for j := range *in.X {
			(*in.X)[j] = x
			want := true
			switch ColumnType(j) {
			case ColumnLowBound:
				want = !m.BelowBound(x, -2)
			case ColumnUpperBound:
				want = !m.AboveBound(x, 3)
			case ColumnFixed, ColumnBoxed:
				want = !m.BelowBound(x, -2) && !m.AboveBound(x, 3)
			}
			ok, err := m.ColumnIsFeasible(uint32(j))
			if err != nil || ok != want {
				t.Fatal(j, x, ok, err)
			}
			if err := m.UpdateColumnInInfSet(uint32(j)); err != nil || m.InfSet.Contains(uint32(j)) == want {
				t.Fatal("infeasibility membership", j, err)
			}
		}
	}
	// All-column scan sees a bad nonbasic value; basic-only scan does not.
	*in.X = []float64{0, -100, 0, 0, 0}
	if ok, err := m.CalcCurrentXIsFeasible(); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, err := m.CalcCurrentXIsFeasibleIncludeNonBasis(); ok || err != nil {
		t.Fatal(ok, err)
	}
	*in.X = []float64{0, 0, 0, 0, 0}
	if ok, err := m.CalcCurrentXIsFeasibleIncludeNonBasis(); !ok || err != nil {
		t.Fatal(ok, err)
	}
	(*in.Basis)[0] = 1
	(*in.X)[1] = -100
	if ok, err := m.CalcCurrentXIsFeasible(); ok || err != nil {
		t.Fatal(ok, err)
	}
	if m.CurrentXIsInfeasible() == m.CurrentXIsFeasible() {
		t.Fatal("cached set complements")
	}
	for _, b := range []float64{-10, 0, 10} {
		lo, hi := b-float64((math.Abs(b)+1)*.125), b+float64((math.Abs(b)+1)*.125)
		if m.BelowBound(lo, b) || !m.BelowBound(math.Nextafter(lo, math.Inf(-1)), b) || m.AboveBound(hi, b) || !m.AboveBound(math.Nextafter(hi, math.Inf(1)), b) {
			t.Fatal("strict boundary", b)
		}
	}
	for _, b := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1)} {
		if !m.AtBound(math.NaN(), b) {
			t.Fatal("source NaN comparisons")
		}
	}
}

func TestLPFloatCoreSourcePublicationAndGuards(t *testing.T) {
	if m, err := NewFloatCoreSolverBase(FloatCoreInputs{}); m != nil || !errors.Is(err, ErrBounds) {
		t.Fatal(m, err)
	}
	in := lpFloatCoreInput(t, 2, 4)
	*in.Basis = []uint32{1, 9}
	m, err := NewFloatCoreSolverBase(in)
	if !errors.Is(err, ErrBounds) || m == nil || !reflect.DeepEqual(*in.Heading, []int32{-1, 0, -1, -1}) || !reflect.DeepEqual(*in.NBasis, []uint32{99}) {
		t.Fatal("heading prefix", err, *in.Heading, *in.NBasis)
	}
	*in.Basis = []uint32{1, 1}
	err = m.AllocateBasisHeading()
	if sourceDebugEnabled {
		if !errors.Is(err, ErrAssertion) {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	// A duplicate basic leaves an extra nonbasic; trim it to pass the
	// preceding debug extent assertion and reach the duplicate predicate.
	*in.NBasis = (*in.NBasis)[:2]
	if ok, err := m.BasisHeadingIsCorrect(); ok || err != nil {
		t.Fatal("duplicate basics", ok, err)
	}
	*in.Basis = []uint32{0, 1}
	if err := m.AllocateBasisHeading(); err != nil {
		t.Fatal(err)
	}
	*in.NBasis = []uint32{2, 2}
	if ok, err := m.BasisHeadingIsCorrect(); ok || err != nil {
		t.Fatal("duplicate nonbasics", ok, err)
	}
	if err := m.AllocateBasisHeading(); err != nil {
		t.Fatal(err)
	}
	(*in.Heading)[0] = 1
	if ok, err := m.BasisHeadingIsCorrect(); ok || err != nil {
		t.Fatal("wrong basic heading", ok, err)
	}
	if err := m.AllocateBasisHeading(); err != nil {
		t.Fatal(err)
	}
	(*in.Heading)[3] = -2
	if ok, err := m.BasisHeadingIsCorrect(); ok || err != nil {
		t.Fatal("wrong nonbasic heading", ok, err)
	}
	// Box check is erased entirely in release, including its vector accesses.
	in = lpFloatCoreInput(t, 0, 1)
	*in.ColumnTypes = []ColumnType{ColumnBoxed}
	*in.LowBounds, *in.UpperBounds = []float64{2}, []float64{1}
	m, err = NewFloatCoreSolverBase(in)
	if sourceDebugEnabled {
		if !errors.Is(err, ErrAssertion) {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	if ok, err := m.BoundsForBoxedAreSetCorrectly(); ok || err != nil {
		t.Fatal(ok, err)
	}
	(*in.LowBounds)[0] = math.NaN()
	if ok, err := m.BoundsForBoxedAreSetCorrectly(); !ok || err != nil {
		t.Fatal(ok, err)
	}
	*in.LowBounds = nil
	if _, err := m.BoundsForBoxedAreSetCorrectly(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if _, err := NewFloatCoreSolverBase(in); (sourceDebugEnabled && !errors.Is(err, ErrBounds)) || (!sourceDebugEnabled && err != nil) {
		t.Fatal("erased boxed assertion reads", err)
	}
	(*in.X)[0] = 100
	if ok, err := m.ColumnIsFeasible(0); ok || err != nil {
		t.Fatal("upper rejection before missing low", ok, err)
	}
	(*in.X)[0] = 0
	if _, err := m.ColumnIsFeasible(0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	for _, typ := range []ColumnType{ColumnBoxed, ColumnUpperBound, ColumnLowBound} {
		*in.ColumnTypes = []ColumnType{typ}
		*in.UpperBounds = nil
		if _, err := m.ColumnIsFeasible(0); !errors.Is(err, ErrBounds) {
			t.Fatal(typ, err)
		}
	}
	*in.ColumnTypes = []ColumnType{99}
	ok, err := m.ColumnIsFeasible(0)
	if ok || (sourceDebugEnabled && !errors.Is(err, ErrAssertion)) || (!sourceDebugEnabled && err != nil) {
		t.Fatal(ok, err)
	}
	if _, err := m.ColumnIsFeasible(1); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.UpdateColumnInInfSet(1); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	in = lpFloatCoreInput(t, 1, 1)
	*in.ColumnTypes = []ColumnType{ColumnLowBound}
	*in.X = []float64{-1}
	m, err = NewFloatCoreSolverBase(in)
	if err != nil || !m.CurrentXIsFeasible() {
		t.Fatal("constructor must not populate inf set", err)
	}
	if err := m.UpdateColumnInInfSet(0); err != nil || !m.CurrentXIsInfeasible() {
		t.Fatal(err)
	}
	(*in.X)[0] = 0
	if err := m.UpdateColumnInInfSet(0); err != nil || !m.CurrentXIsFeasible() {
		t.Fatal(err)
	}
	*in.Basis = nil
	if _, err := m.CalcCurrentXIsFeasible(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if ok, err := m.BasisHeadingIsCorrect(); ok || (sourceDebugEnabled && !errors.Is(err, ErrAssertion)) || (!sourceDebugEnabled && !errors.Is(err, ErrBounds)) {
		t.Fatal("short basis", ok, err)
	}
	*in.X = nil
	if _, err := m.CalcCurrentXIsFeasibleIncludeNonBasis(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
}

func TestLPFloatCoreHeadingInvalidDomainProfiles(t *testing.T) {
	in := lpFloatCoreInput(t, 1, 3)
	m, err := NewFloatCoreSolverBase(in)
	if err != nil {
		t.Fatal(err)
	}
	(*in.Basis)[0] = 9
	if _, err := m.BasisHeadingIsCorrect(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	(*in.Basis)[0] = 0
	(*in.NBasis)[0] = 9
	if _, err := m.BasisHeadingIsCorrect(); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	if err := m.AllocateBasisHeading(); err != nil {
		t.Fatal(err)
	}
	// The source permits fewer nonbasics for dual; a positive heading omitted
	// from that list is checked only by the debug inverse-basic assertion.
	*in.NBasis = nil
	(*in.Heading)[2] = 0
	ok, err := m.BasisHeadingIsCorrect()
	if sourceDebugEnabled {
		if ok || !errors.Is(err, ErrAssertion) {
			t.Fatal(ok, err)
		}
	} else if !ok || err != nil {
		t.Fatal(ok, err)
	}
	*in.Heading = nil
	if _, err := m.BasisHeadingIsCorrect(); (sourceDebugEnabled && !errors.Is(err, ErrAssertion)) || (!sourceDebugEnabled && !errors.Is(err, ErrBounds)) {
		t.Fatal(err)
	}
	// Non-boxed trailing metadata does not cause any bound access.
	*in.ColumnTypes = []ColumnType{ColumnFree, ColumnLowBound, ColumnUpperBound, ColumnFixed}
	*in.LowBounds, *in.UpperBounds = nil, nil
	if ok, err := m.BoundsForBoxedAreSetCorrectly(); !ok || err != nil {
		t.Fatal(ok, err)
	}
}

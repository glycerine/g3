package lp

import (
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestLPFloatPrimalPricingEligibilityIndependentCases(t *testing.T) {
	m := lpFloatPrimalCore(t, 0, 1)
	m.EpsilonOfReducedCost = .125
	*m.LowBounds = []float64{0}
	*m.UpperBounds = []float64{10}
	for _, typ := range []ColumnType{ColumnFree, ColumnLowBound, ColumnUpperBound, ColumnBoxed, ColumnFixed} {
		(*m.ColumnTypes)[0] = typ
		for _, d := range []float64{-2, -.125, 0, .125, 2, math.NaN(), math.Inf(1), math.Inf(-1)} {
			m.D[0] = d
			for _, x := range []float64{0, 4, 5, 6, 10, math.NaN()} {
				(*m.X)[0] = x
				want := false
				switch typ {
				case ColumnFree:
					want = d > .125 || d < -.125
				case ColumnLowBound:
					want = d > .125
				case ColumnUpperBound:
					want = d < -.125
				case ColumnBoxed:
					want = (d > .125 && x < 5) || (d < -.125 && x > 5)
				}
				if got, err := m.ColumnIsBeneficialForEnteringBasis(0); got != want || err != nil {
					t.Fatal(typ, d, x, got, want, err)
				}
			}
			for _, upper := range []bool{false, true} {
				(*m.X)[0] = 0
				if upper || typ == ColumnUpperBound {
					(*m.X)[0] = 10
				}
				if typ == ColumnLowBound {
					(*m.X)[0] = 0
				}
				want := false
				switch typ {
				case ColumnFree:
					want = d > .125 || d < -.125
				case ColumnLowBound:
					want = d < -.125
				case ColumnUpperBound:
					want = d > .125
				case ColumnBoxed:
					if upper {
						want = d > .125
					} else {
						want = d < -.125
					}
				}
				m.UsingInfeasCosts = true
				m.Settings.UseBreakpointsInFeasibilitySearch = true
				if got, err := m.ColumnIsBeneficialForEnteringBasis(0); got != want || err != nil {
					t.Fatal("breakpoint", typ, d, upper, got, want, err)
				}
				m.UsingInfeasCosts = false
			}
		}
	}
}

func TestLPFloatPrimalPricingIndependentOrderAndRotation(t *testing.T) {
	rng := rand.New(rand.NewSource(819911))
	for world := 0; world < 100; world++ {
		m := lpFloatPrimalCore(t, 0, 17)
		ranks := rng.Perm(17)
		for j, r := range ranks {
			(*m.ColumnTypes)[j] = ColumnFree
			m.ColumnNorms[j] = 1
			m.D[j] = float64(r + 1)
			if j%2 == 0 {
				m.D[j] = -m.D[j]
			}
		}
		if err := m.SortNonBasis(); err != nil {
			t.Fatal(err)
		}
		want := make([]uint32, 17)
		for j, r := range ranks {
			want[16-r] = uint32(j)
		}
		if !reflect.DeepEqual(*m.NBasis, want) || !reflect.DeepEqual(m.NonBasisList, want) {
			t.Fatal("independent rank order", world)
		}
		for i, j := range want {
			if (*m.Heading)[j] != -int32(i)-1 || m.SteepestEdge[j] != float64((ranks[j]+1)*(ranks[j]+1)) {
				t.Fatal("inverse/score", world)
			}
		}
		// A full scan always finds the independent maximum, then moves it to
		// the end of the traversal list without changing the nonbasis vector.
		best := want[0]
		for step := 0; step < 45; step++ {
			got, err := m.ChooseEnteringColumn(17)
			if got != int32(best) || err != nil || m.NonBasisList[16] != best || !reflect.DeepEqual(*m.NBasis, want) {
				t.Fatal(world, step, got, err)
			}
			sign := int32(1)
			if m.D[best] < 0 {
				sign = -1
			}
			if m.SignOfEnteringDelta != sign {
				t.Fatal("entering sign")
			}
		}
	}
}

func TestLPFloatPrimalPricingBudgetAndIEEE(t *testing.T) {
	for budget, want := range map[uint32]int32{0: -1, 1: 0, 2: 2, 3: 3} {
		m := lpFloatPrimalCore(t, 0, 4)
		*m.ColumnTypes = []ColumnType{ColumnFree, ColumnFree, ColumnFree, ColumnFree}
		m.D = []float64{2, 1, 3, 4}
		m.ColumnNorms = []float64{1, 1, 1, 1}
		m.NonBasisList = []uint32{0, 1, 2, 3}
		m.BasisSortCounter = 9
		got, err := m.ChooseEnteringColumn(budget)
		if got != want || err != nil {
			t.Fatal("budget counts record improvements", budget, got, want, err)
		}
		counter := uint32(8)
		if budget == 0 {
			counter = 9
		}
		if m.BasisSortCounter != counter {
			t.Fatal("zero budget is before counter", budget)
		}
	}
	m := lpFloatPrimalCore(t, 0, 3)
	*m.ColumnTypes = []ColumnType{ColumnFree, ColumnFree, ColumnFree}
	m.D = []float64{1, 2, 3}
	m.ColumnNorms = []float64{math.NaN(), -1, math.Inf(1)}
	if got, err := m.ChooseEnteringColumn(3); got != -1 || err != nil {
		t.Fatal("unordered/nonpositive prices", got, err)
	}
	m.ColumnNorms[2] = 0
	if got, err := m.ChooseEnteringColumn(3); got != 2 || err != nil {
		t.Fatal("infinite price", got, err)
	}
	m = lpFloatPrimalCore(t, 0, 1)
	(*m.ColumnTypes)[0] = ColumnFree
	m.D[0] = -2
	m.ColumnNorms[0] = 1
	m.UsingInfeasCosts = true
	m.Settings.UseBreakpointsInFeasibilitySearch = true
	if got, err := m.ChooseEnteringColumn(1); got != 0 || err != nil || m.SignOfEnteringDelta != 1 {
		t.Fatal("breakpoint sign reverses", got, err)
	}
}

func TestLPFloatPrimalPricingPublicationAndAssertions(t *testing.T) {
	m := lpFloatPrimalCore(t, 0, 3)
	m.D = []float64{3, 2, 1}
	m.ColumnNorms = []float64{1, 1, 1}
	*m.Heading = (*m.Heading)[:2]
	m.NonBasisList = []uint32{99}
	if err := m.SortNonBasis(); !errors.Is(err, ErrBounds) || !reflect.DeepEqual(m.NonBasisList, []uint32{0, 1}) || !reflect.DeepEqual(*m.NBasis, []uint32{0, 1, 2}) {
		t.Fatal("score/sort precede heading prefix", err, m.NonBasisList, *m.NBasis)
	}
	*m.NBasis = []uint32{0, 9}
	m.SteepestEdge[0] = 0
	if err := m.SortNonBasis(); !errors.Is(err, ErrBounds) || m.SteepestEdge[0] != 9 || !reflect.DeepEqual(m.NonBasisList, []uint32{0, 1}) {
		t.Fatal("score prefix before bad next index", err)
	}
	m.BasisSortCounter = 0
	if _, err := m.ChooseEnteringColumn(1); !errors.Is(err, ErrBounds) || m.BasisSortCounter != 0 {
		t.Fatal(err)
	}
	m.BasisSortCounter = 5
	m.NonBasisList = []uint32{99}
	if _, err := m.ChooseEnteringColumn(1); !errors.Is(err, ErrBounds) || m.BasisSortCounter != 4 {
		t.Fatal("counter before failed eligibility", err)
	}
	if _, err := m.ColumnIsBeneficialForEnteringOnBreakpoints(99); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	m = lpFloatPrimalCore(t, 0, 1)
	(*m.ColumnTypes)[0] = 99
	for _, breakpoints := range []bool{false, true} {
		m.UsingInfeasCosts = breakpoints
		m.Settings.UseBreakpointsInFeasibilitySearch = breakpoints
		got, err := m.ColumnIsBeneficialForEnteringBasis(0)
		if got || (sourceDebugEnabled && !errors.Is(err, ErrAssertion)) || (!sourceDebugEnabled && err != nil) {
			t.Fatal(got, err)
		}
	}
	(*m.ColumnTypes)[0] = ColumnLowBound
	(*m.X)[0] = 4
	(*m.LowBounds)[0] = 0
	got, err := m.ColumnIsBeneficialForEnteringOnBreakpoints(0)
	if sourceDebugEnabled {
		if got || !errors.Is(err, ErrAssertion) {
			t.Fatal(got, err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	(*m.ColumnTypes)[0] = ColumnBoxed
	(*m.UpperBounds)[0] = 10
	got, err = m.ColumnIsBeneficialForEnteringOnBreakpoints(0)
	if sourceDebugEnabled {
		if got || !errors.Is(err, ErrAssertion) {
			t.Fatal(got, err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	*m.X = nil
	if _, err := m.ColumnIsBeneficialForEnteringOnBreakpoints(0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	m.D[0] = 2
	m.UsingInfeasCosts = false
	if _, err := m.ColumnIsBeneficialForEnteringBasis(0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*m.X = []float64{0}
	*m.UpperBounds = nil
	if _, err := m.ColumnIsBeneficialForEnteringBasis(0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	m.UsingInfeasCosts = true
	(*m.ColumnTypes)[0] = ColumnLowBound
	*m.X = nil
	_, err = m.ColumnIsBeneficialForEnteringBasis(0)
	if sourceDebugEnabled {
		if !errors.Is(err, ErrBounds) {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	(*m.ColumnTypes)[0] = ColumnBoxed
	*m.X = []float64{9}
	*m.LowBounds = []float64{0}
	_, err = m.ColumnIsBeneficialForEnteringOnBreakpoints(0)
	if sourceDebugEnabled {
		if !errors.Is(err, ErrBounds) {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	m = lpFloatPrimalCore(t, 0, 2)
	*m.ColumnTypes = []ColumnType{ColumnFixed, ColumnFree}
	m.D = []float64{100, 2}
	m.ColumnNorms = []float64{1, 1}
	m.NonBasisList = []uint32{0, 1}
	m.BasisSortCounter = 5
	if got, err := m.ChooseEnteringColumn(1); got != 1 || err != nil {
		t.Fatal("ineligible candidate does not consume budget", got, err)
	}
	m.ColumnNorms = nil
	if _, err := m.ChooseEnteringColumn(1); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
}

// TestLPFloatPrimalSortNonBasisLibstdcxxTieOrder: sort_non_basis's
// std::sort (steepest edge, descending) on 20 columns with tied
// coefficients must leave g++ 13.3's introsort permutation (util_lp_05-F1).
func TestLPFloatPrimalSortNonBasisLibstdcxxTieOrder(t *testing.T) {
	d := []float64{1, 2, 1, 3, 2, 1, 3, 1, 2, 2, 1, 3, 1, 1, 2, 3, 2, 1, 3, 1}
	m := lpFloatPrimalCore(t, 0, uint32(len(d)))
	for j, v := range d {
		m.ColumnNorms[j] = 1
		m.D[j] = v
	}
	// The constructor lists the nonbasic columns in reverse; start from
	// 0..19 as the C++ control does.
	for i := range *m.NBasis {
		(*m.NBasis)[i] = uint32(i)
	}
	if err := m.SortNonBasis(); err != nil {
		t.Fatal(err)
	}
	want := []uint32{3, 18, 6, 11, 15, 1, 4, 8, 9, 16, 14, 19, 0, 17, 13, 12, 10, 7, 5, 2}
	if !reflect.DeepEqual(*m.NBasis, want) || !reflect.DeepEqual(m.NonBasisList, want) {
		t.Fatal(*m.NBasis, want)
	}
}

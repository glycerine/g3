package lp

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func TestLPExactPrimalPricingIndependentEligibility(t *testing.T) {
	h := newLPPairHarness(t)
	s, e := NewLARSolver(h.m)
	h.must(e)
	_, e = s.AddVar(31)
	h.must(e)
	m := s.Core.RPrimal
	h.must(s.Bounds.Low.Set(0, h.pair("-3/7", "1/11")))
	h.must(s.Bounds.Upper.Set(0, h.pair("5/13", "-1/17")))
	lowX, lowY := big.NewRat(-3, 7), big.NewRat(1, 11)
	upX, upY := big.NewRat(5, 13), big.NewRat(-1, 17)
	count := 0
	for _, typ := range []ColumnType{ColumnFree, ColumnFixed, ColumnLowBound, ColumnUpperBound, ColumnBoxed} {
		h.must(s.Bounds.Types.Set(0, typ))
		for _, ds := range []string{"-19/23", "-1/10000000", "0", "1/10000000", "29/31"} {
			d := mustLPRat(t, ds)
			h.must(h.m.SetString(&m.D.Data[0], ds))
			for _, xs := range []string{"-3/7", "0", "5/13"} {
				for _, ys := range []string{"-1/5", "1/11", "-1/17", "1/5"} {
					x, y := mustLPRat(t, xs), mustLPRat(t, ys)
					h.must(s.Core.RX.Data[0].Copy(h.m, h.pair(xs, ys)))
					cmpLow := x.Cmp(lowX)
					if cmpLow == 0 {
						cmpLow = y.Cmp(lowY)
					}
					cmpUp := x.Cmp(upX)
					if cmpUp == 0 {
						cmpUp = y.Cmp(upY)
					}
					want := false
					switch typ {
					case ColumnFree:
						want = d.Sign() != 0
					case ColumnLowBound:
						want = d.Sign() > 0 || (d.Sign() < 0 && cmpLow > 0)
					case ColumnUpperBound:
						want = d.Sign() < 0 || (d.Sign() > 0 && cmpUp < 0)
					case ColumnBoxed:
						want = (d.Sign() > 0 && cmpUp < 0) || (d.Sign() < 0 && cmpLow > 0)
					}
					got, err := m.ColumnIsBeneficialForEnteringBasis(0)
					h.must(err)
					if got != want {
						t.Fatal(typ, ds, xs, ys, got, want)
					}
					count++
				}
			}
			for _, atUpper := range []bool{false, true} {
				bound := &s.Bounds.Low.values[0]
				if atUpper || typ == ColumnUpperBound {
					bound = &s.Bounds.Upper.values[0]
				}
				if typ == ColumnLowBound {
					bound = &s.Bounds.Low.values[0]
				}
				h.must(s.Core.RX.Data[0].Copy(h.m, bound))
				m.UsingInfeasCosts = true
				s.Settings.UseBreakpointsInFeasibilitySearch = true
				eps := big.NewRat(1, 10000000)
				neg := new(big.Rat).Neg(eps)
				want := false
				switch typ {
				case ColumnFree:
					want = d.Cmp(eps) > 0 || d.Cmp(neg) < 0
				case ColumnLowBound:
					want = d.Cmp(neg) < 0
				case ColumnUpperBound:
					want = d.Cmp(eps) > 0
				case ColumnBoxed:
					if atUpper {
						want = d.Cmp(eps) > 0
					} else {
						want = d.Cmp(neg) < 0
					}
				}
				got, err := m.ColumnIsBeneficialForEnteringBasis(0)
				h.must(err)
				if got != want {
					t.Fatal("breakpoints", typ, ds, atUpper, got, want)
				}
				m.UsingInfeasCosts = false
				s.Settings.UseBreakpointsInFeasibilitySearch = false
			}
		}
	}
	if count != 300 {
		t.Fatal(count)
	}
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPExactPrimalPricingIndependentScanRotationAndRNG(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	h := newLPPairHarness(t)
	rng := rand.New(rand.NewPCG(8771, 481))
	for world := 0; world < 160; world++ {
		s := lpNewLARLUPreparedModel(t, h, lpExactIdentity(7))
		m := s.Core.RPrimal
		for j := 0; j < 9; j++ {
			_, e := s.AddVar(uint32(100 + j))
			h.must(e)
		}
		h.must(m.D.resizeData(16))
		m.ColumnsNZ = make([]uint32, 16)
		m.BasisSortCounter = 3
		order := rng.Perm(9)
		m.NonBasisList = nil
		for _, j := range order {
			m.NonBasisList = append(m.NonBasisList, uint32(j+7))
		}
		signs := make([]int, 16)
		for j := 7; j < 16; j++ {
			m.ColumnsNZ[j] = uint32(rng.IntN(8))
			signs[j] = rng.IntN(3) - 1
			h.must(h.m.SetInt32(&m.D.Data[j], int32(signs[j])))
		}
		seed := uint32(world + 19)
		s.Settings.RandomSeed(seed)
		oracle := z3rand.New(seed)
		budget := uint32(world % 5)
		remaining := budget
		best, position := uint32(8), -1
		before := append([]uint32(nil), m.NonBasisList...)
		for i, j := range before {
			if remaining == 0 {
				break
			}
			if signs[j] == 0 {
				continue
			}
			if m.ColumnsNZ[j] < best {
				best, position = m.ColumnsNZ[j], i
				remaining--
			} else if m.ColumnsNZ[j] == best && oracle.Next()%2 == 0 {
				position = i
			}
		}
		got, e := m.ChooseEnteringColumn(budget)
		h.must(e)
		want := int32(-1)
		expected := append([]uint32(nil), before...)
		if position >= 0 {
			want = int32(before[position])
			expected = append(append(expected[:position:position], expected[position+1:]...), uint32(want))
		}
		if got != want || !reflect.DeepEqual(m.NonBasisList, expected) {
			t.Fatal(world, got, want, m.NonBasisList, expected)
		}
		if position >= 0 && m.SignOfEnteringDelta != int32(signs[want]) {
			t.Fatal("entering direction")
		}
		wantCounter := uint32(2)
		if budget == 0 {
			wantCounter = 3
		}
		if m.BasisSortCounter != wantCounter {
			t.Fatal("zero budget changed counter")
		}
		if s.Settings.RandomNext() != oracle.Next() {
			t.Fatal("random consumption", world)
		}
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPExactPrimalPricingSortSourcesAndCounter(t *testing.T) {
	for _, tableau := range []bool{false, true} {
		h := newLPPairHarness(t)
		s := lpNewLARLUPreparedModel(t, h, lpExactIdentity(4))
		m := s.Core.RPrimal
		for j := 0; j < 4; j++ {
			_, e := s.AddVar(uint32(100 + j))
			h.must(e)
		}
		h.must(m.D.resizeData(8))
		m.ColumnsNZ = make([]uint32, 8)
		for j := uint32(4); j < 8; j++ {
			m.ColumnsNZ[j] = 8 - j
			h.must(h.m.SetInt32(&m.D.Data[j], 1))
			for row := uint32(0); row < j-3; row++ {
				h.must(m.A.Set(row, j, &h.pair("1", "0").X))
			}
		}
		if tableau {
			s.Settings.SimplexStrategy = StrategyTableauCosts
		}
		h.must(m.SortNonBasis())
		want := []uint32{7, 6, 5, 4}
		if tableau {
			want = []uint32{4, 5, 6, 7}
		}
		if !reflect.DeepEqual(*m.NBasis, want) || !reflect.DeepEqual(m.NonBasisList, want) {
			t.Fatal(tableau, *m.NBasis, want)
		}
		for i, j := range want {
			if (*m.Heading)[j] != -int32(i)-1 {
				t.Fatal("heading", i, j)
			}
		}
		got, e := m.ChooseEnteringColumn(1)
		h.must(e)
		if got != int32(want[0]) || m.BasisSortCounter != 20 {
			t.Fatal("sort counter reset", got, m.BasisSortCounter)
		}
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPExactPrimalPricingPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		for _, op := range []string{"bound", "epsilon-positive", "epsilon-negative", "breakpoint-low", "breakpoint-boxed", "choose"} {
			success, failures := false, 0
			for ordinal := int64(0); ordinal < 200; ordinal++ {
				done := false
				t.Run(fmt.Sprintf("%t/%s/%d", direct, op, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					s, e := NewLARSolver(h.m)
					h.must(e)
					_, e = s.AddVar(31)
					h.must(e)
					m := s.Core.RPrimal
					huge := strings.Repeat("1234567890", 100)
					a := huge + "1/7"
					b := huge + "3/11"
					h.must(h.m.SetString(&m.D.Data[0], a))
					if op == "bound" || op == "choose" {
						h.must(s.Bounds.Types.Set(0, ColumnUpperBound))
						h.must(s.Bounds.Upper.Set(0, h.pair(b, "0")))
						h.must(s.Core.RX.Data[0].Copy(h.m, h.pair(a, "0")))
						m.BasisSortCounter = 1
						m.ColumnsNZ = []uint32{0}
						m.NonBasisList = []uint32{0}
					} else {
						h.must(h.m.SetString(&m.EpsilonOfReducedCost, b))
						m.UsingInfeasCosts = true
						s.Settings.UseBreakpointsInFeasibilitySearch = true
						if op == "breakpoint-low" || op == "breakpoint-boxed" {
							kind := ColumnLowBound
							if op == "breakpoint-boxed" {
								kind = ColumnBoxed
							}
							h.must(s.Bounds.Types.Set(0, kind))
							h.must(s.Bounds.Low.Set(0, h.pair("0", "0")))
							h.must(s.Bounds.Upper.Set(0, h.pair("1", "0")))
						}
						if op == "epsilon-negative" || op == "breakpoint-low" || op == "breakpoint-boxed" {
							h.must(h.m.Neg(&m.D.Data[0]))
						}
					}
					h.budget.FailAfter(ordinal)
					var err error
					if op == "choose" {
						_, err = m.ChooseEnteringColumn(1)
					} else {
						_, err = m.ColumnIsBeneficialForEnteringBasis(0)
					}
					h.budget.FailAfter(-1)
					if err == nil {
						success = true
						done = true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if m.PairCoreSolverBase.calls.pending() && m.calls.pending() {
							t.Fatal("primal adopted base comparison")
						}
						if _, e = m.ColumnIsBeneficialForEnteringBasis(0); e != ErrExactVectorPending {
							t.Fatal("lost stopped owner")
						}
					}
					h.must(s.Close())
					lpExactVectorNamedCensus(t, h, nil)
				})
				if done {
					break
				}
			}
			if !success || failures == 0 {
				t.Fatal("incomplete campaign", direct, op, success, failures)
			}
		}
	}
}

func TestLPExactPrimalPricingSourceGuardsAndPrefixes(t *testing.T) {
	for _, op := range []string{"kind", "D", "X", "bound", "off-low", "off-upper", "boxed-interior", "sort-key", "sort-heading", "scan-count", "zero-sort", "normal-D", "low-extent", "breakpoint-X", "breakpoint-low", "breakpoint-upper", "breakpoint-kind", "tableau-key", "sort-in-scan"} {
		t.Run(op, func(t *testing.T) {
			h := newLPPairHarness(t)
			s, e := NewLARSolver(h.m)
			h.must(e)
			_, e = s.AddVar(31)
			h.must(e)
			m := s.Core.RPrimal
			h.must(s.Bounds.Low.Set(0, h.pair("0", "0")))
			h.must(s.Bounds.Upper.Set(0, h.pair("2", "0")))
			h.must(h.m.SetInt32(&m.D.Data[0], 1))
			m.ColumnsNZ = []uint32{0}
			m.NonBasisList = []uint32{0}
			m.BasisSortCounter = 3
			var err error
			want := ErrBounds
			switch op {
			case "normal-D":
				_, err = m.ColumnIsBeneficialForEnteringBasis(99)
			case "low-extent":
				h.must(s.Bounds.Types.Set(0, ColumnLowBound))
				h.must(h.m.SetInt32(&m.D.Data[0], -1))
				saved := s.Bounds.Low.values
				s.Bounds.Low.values = nil
				_, err = m.ColumnIsBeneficialForEnteringBasis(0)
				s.Bounds.Low.values = saved
			case "breakpoint-X":
				h.must(s.Bounds.Types.Set(0, ColumnBoxed))
				saved := s.Core.RX.Data
				s.Core.RX.Data = nil
				_, err = m.ColumnIsBeneficialForEnteringOnBreakpoints(0)
				s.Core.RX.Data = saved
			case "breakpoint-low":
				h.must(s.Bounds.Types.Set(0, ColumnBoxed))
				saved := s.Bounds.Low.values
				s.Bounds.Low.values = nil
				_, err = m.ColumnIsBeneficialForEnteringOnBreakpoints(0)
				s.Bounds.Low.values = saved
			case "breakpoint-upper":
				h.must(s.Bounds.Types.Set(0, ColumnBoxed))
				h.must(s.Core.RX.Data[0].Copy(h.m, h.pair("1", "0")))
				saved := s.Bounds.Upper.values
				s.Bounds.Upper.values = nil
				_, err = m.ColumnIsBeneficialForEnteringOnBreakpoints(0)
				s.Bounds.Upper.values = saved
				want = nil
				if sourceDebugEnabled {
					want = ErrBounds
				}
			case "breakpoint-kind":
				h.must(s.Bounds.Types.Set(0, ColumnType(99)))
				_, err = m.ColumnIsBeneficialForEnteringOnBreakpoints(0)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
			case "tableau-key":
				*m.NBasis = []uint32{99}
				err = m.SortNonBasis()
			case "sort-in-scan":
				*m.NBasis = []uint32{99}
				m.BasisSortCounter = 0
				_, err = m.ChooseEnteringColumn(1)
			case "kind":
				h.must(s.Bounds.Types.Set(0, ColumnType(99)))
				_, err = m.ColumnIsBeneficialForEnteringBasis(0)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
			case "D":
				_, err = m.ColumnIsBeneficialForEnteringOnBreakpoints(99)
			case "X":
				h.must(s.Bounds.Types.Set(0, ColumnUpperBound))
				saved := s.Core.RX.Data
				s.Core.RX.Data = nil
				_, err = m.ColumnIsBeneficialForEnteringBasis(0)
				s.Core.RX.Data = saved
			case "bound":
				h.must(s.Bounds.Types.Set(0, ColumnUpperBound))
				saved := s.Bounds.Upper.values
				s.Bounds.Upper.values = nil
				_, err = m.ColumnIsBeneficialForEnteringBasis(0)
				s.Bounds.Upper.values = saved
			case "off-low", "off-upper", "boxed-interior":
				kind := ColumnLowBound
				if op == "off-upper" {
					kind = ColumnUpperBound
				}
				if op == "boxed-interior" {
					kind = ColumnBoxed
				}
				h.must(s.Bounds.Types.Set(0, kind))
				h.must(s.Core.RX.Data[0].Copy(h.m, h.pair("1", "0")))
				_, err = m.ColumnIsBeneficialForEnteringOnBreakpoints(0)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
			case "sort-key":
				m.ColumnsNZ = nil
				s.Settings.SimplexStrategy = StrategyLU
				err = m.SortNonBasis()
			case "sort-heading":
				s.Core.RHeading = nil
				err = m.SortNonBasis()
				if len(m.NonBasisList) != 0 {
					t.Fatal("list cleared before heading publication")
				}
			case "scan-count":
				m.ColumnsNZ = nil
				_, err = m.ChooseEnteringColumn(1)
				if m.BasisSortCounter != 2 {
					t.Fatal("counter prefix")
				}
			case "zero-sort":
				_, e = s.AddVar(67)
				h.must(e)
				m.ColumnsNZ = []uint32{0, 2}
				*m.NBasis = []uint32{1, 0}
				s.Settings.SimplexStrategy = StrategyLU
				err = m.SortNonBasis()
				want = nil
				if len(m.NonBasisList) != 2 || m.NonBasisList[0] == m.NonBasisList[1] {
					t.Fatal("zero-comparator permutation")
				}
				for i, j := range m.NonBasisList {
					if (*m.Heading)[j] != -int32(i)-1 {
						t.Fatal("zero-comparator heading")
					}
				}
			}
			if err != want {
				t.Fatal(op, err, want)
			}
			if err != nil {
				for _, call := range []func() error{m.SortNonBasis, func() error { _, e := m.ColumnIsBeneficialForEnteringBasis(0); return e }, func() error { _, e := m.ColumnIsBeneficialForEnteringOnBreakpoints(0); return e }, func() error { _, e := m.ChooseEnteringColumn(1); return e }} {
					if e := call(); e != ErrExactVectorPending {
						t.Fatal("lost stopped owner", e)
					}
				}
			}
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLPExactPrimalPricingOlderComparisonOwner(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, op := range []string{"bound", "epsilon", "choose", "breakpoint"} {
		matched := false
		for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
			t.Run(fmt.Sprintf("%s/%d", op, ordinal), func(t *testing.T) {
				h := newLPPairHarness(t)
				s, e := NewLARSolver(h.m)
				h.must(e)
				_, e = s.AddVar(31)
				h.must(e)
				m := s.Core.RPrimal
				v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
				h.must(s.Bounds.Types.Set(0, ColumnUpperBound))
				h.must(s.Bounds.Upper.Set(0, v))
				h.must(h.m.Copy(&s.Core.RX.Data[0].X, &v.Y))
				h.must(h.m.SetInt32(&m.D.Data[0], 1))
				m.BasisSortCounter = 1
				m.ColumnsNZ = []uint32{0}
				m.NonBasisList = []uint32{0}
				if op == "epsilon" {
					h.must(s.Bounds.Types.Set(0, ColumnFree))
					m.UsingInfeasCosts = true
					s.Settings.UseBreakpointsInFeasibilitySearch = true
					h.must(h.m.SetInt32(&m.D.Data[0], -1))
				}
				if op == "breakpoint" {
					h.must(s.Bounds.Types.Set(0, ColumnLowBound))
					h.must(s.Bounds.Low.Set(0, h.pair("0", "0")))
					h.must(s.Core.RX.Data[0].Copy(h.m, h.pair("0", "0")))
					m.UsingInfeasCosts = true
					s.Settings.UseBreakpointsInFeasibilitySearch = true
					h.must(h.m.SetInt32(&m.D.Data[0], -1))
				}
				var caller exactEtaCalls
				before, e := caller.begin(h.m)
				h.must(e)
				h.budget.FailAfter(ordinal)
				_, err := h.m.Compare(&v.X, &v.Y)
				h.budget.FailAfter(-1)
				err = caller.after(h.m, before, err)
				older := h.m.ComparisonInvocationState()
				if older.Active {
					matched = true
					if !lpExactPriorityPhysicalRefusal(err) {
						t.Fatal(err)
					}
					if op == "choose" {
						_, err = m.ChooseEnteringColumn(1)
					} else {
						_, err = m.ColumnIsBeneficialForEnteringBasis(0)
					}
					if err == nil || m.calls.pending() || m.PairCoreSolverBase.calls.pending() {
						t.Fatal("adopted older comparison", err)
					}
				}
				h.must(s.Close())
				if older.Active && h.m.ComparisonInvocationState() != older {
					t.Fatal("consumed caller comparison")
				}
				h.must(caller.cleanupRaw(h.m))
				h.must(caller.cleanupRoots(h.m))
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
		if !matched {
			t.Fatal("no actual caller comparison", op)
		}
	}
}

func TestLPExactPrimalPricingBreakpointDirection(t *testing.T) {
	for _, sign := range []int32{-1, 1} {
		h := newLPPairHarness(t)
		s, e := NewLARSolver(h.m)
		h.must(e)
		_, e = s.AddVar(31)
		h.must(e)
		m := s.Core.RPrimal
		h.must(h.m.SetInt32(&m.D.Data[0], sign))
		m.NonBasisList = []uint32{0}
		m.ColumnsNZ = []uint32{0}
		m.BasisSortCounter = 1
		m.UsingInfeasCosts = true
		s.Settings.UseBreakpointsInFeasibilitySearch = true
		j, e := m.ChooseEnteringColumn(1)
		h.must(e)
		if j != 0 || m.SignOfEnteringDelta != -sign {
			t.Fatal(j, m.SignOfEnteringDelta, sign)
		}
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

// TestLPExactPrimalSortNonBasisLibstdcxxOrder: AUDIT.md util_lp_05-F1.
// sort_non_basis_rational's std::sort on counts [3,0,2]: libstdc++
// __insertion_sort sees comp(2, 3) and moves column 6 to the front, giving
// [6,4,5]; Go's sort.Slice left [4,5,6]. Expected order from a g++ 13.3
// program running the same comparator (see nbasis_libstdcxx_sort.go).
func TestLPExactPrimalSortNonBasisLibstdcxxOrder(t *testing.T) {
	h := newLPPairHarness(t)
	s := lpNewLARLUPreparedModel(t, h, lpExactIdentity(4))
	m := s.Core.RPrimal
	for j := 0; j < 3; j++ {
		_, e := s.AddVar(uint32(100 + j))
		h.must(e)
	}
	m.ColumnsNZ = []uint32{0, 0, 0, 0, 3, 0, 2}
	if want := []uint32{4, 5, 6}; !reflect.DeepEqual(*m.NBasis, want) {
		t.Fatal("setup", *m.NBasis)
	}
	h.must(m.SortNonBasis())
	want := []uint32{6, 4, 5}
	if !reflect.DeepEqual(*m.NBasis, want) || !reflect.DeepEqual(m.NonBasisList, want) {
		t.Fatal(*m.NBasis, want)
	}
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

// TestLPSortNonBasisLibstdcxxTieOrder pins sort_non_basis_rational's
// comparator on 20 tie-heavy counts (introsort path) to the permutation
// g++ 13.3 std::sort produces (util_lp_05-F1).
func TestLPSortNonBasisLibstdcxxTieOrder(t *testing.T) {
	counts := []uint32{2, 0, 1, 3, 0, 2, 1, 1, 0, 2, 3, 1, 2, 0, 1, 2, 1, 3, 0, 2}
	nbasis := make([]uint32, len(counts))
	for j := range nbasis {
		nbasis[j] = uint32(j)
	}
	sortNonBasisByColumnCount(nbasis, func(j uint32) uint32 { return counts[j] })
	want := []uint32{2, 19, 10, 18, 6, 7, 16, 11, 14, 5, 9, 0, 12, 15, 17, 13, 8, 4, 3, 1}
	if !reflect.DeepEqual(nbasis, want) {
		t.Fatal(nbasis, want)
	}
}

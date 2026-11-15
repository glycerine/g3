package lp

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestLPRationalObjectiveIndependentCostsAndReconstruction(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 160; world++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		const n = 6
		h.must(m.Costs.resizeData(n))
		h.must(m.ColumnScale.resizeData(n))
		x := NewIndexedRationalVector(h.m, n)
		costs, scales, values := make([]*big.Rat, n), make([]*big.Rat, n), make([]*big.Rat, n)
		for j := 0; j < n; j++ {
			ext := uint32(20 + 7*j)
			c, err := m.GetOrCreateColumnInfo(ext)
			h.must(err)
			c.Index = uint32(j)
			m.CoreColumnsToExternal[uint32(j)] = ext
			numerator := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), uint(81+world)), big.NewInt(int64(j+1)))
			if (world+j)%2 == 0 {
				numerator.Neg(numerator)
			}
			costs[j] = new(big.Rat).SetFrac(numerator, big.NewInt(17))
			if world%11 == 0 {
				costs[j] = new(big.Rat)
			}
			scales[j] = big.NewRat(int64(world-77+j), 23)
			values[j] = big.NewRat(int64(3*world-17*j), 29)
			h.must(c.SetCost(&h.pair(costs[j].RatString(), "0").X))
			h.must(m.ColumnScale.copySlot(&m.ColumnScale.Data[j], &h.pair(scales[j].RatString(), "0").X))
			h.must(x.copySlot(&x.Data[j], &h.pair(values[j].RatString(), "0").X))
			switch j {
			case 1, 2:
				h.must(c.SetLowBound(&h.pair("-7/13", "0").X))
			}
			if j == 2 || j == 3 {
				h.must(c.SetUpperBound(&h.pair("11/19", "0").X))
			}
			if j == 4 || j == 5 {
				h.must(c.SetFixedValue(&h.pair("31/37", "0").X))
			}
			if j == 5 {
				c.Index = InvalidColumnIndex
				c.UpperSet = true
			}
		}
		out := h.pair("91", "0")
		for j := 0; j < n; j++ {
			want := new(big.Rat).Mul(values[j], scales[j])
			switch j {
			case 1, 2:
				want.Add(want, big.NewRat(-7, 13))
			case 3:
				want.Sub(big.NewRat(11, 19), want)
			case 4, 5:
				want.SetFrac64(31, 37)
			}
			h.must(m.ColumnValueFromCore(uint32(20+7*j), x.Data, &out.X))
			lpSparseRationalCheck(t, h, &out.X, want)
			if j < 4 {
				h.must(m.SetScaledCost(uint32(j)))
				want.Mul(costs[j], scales[j])
				if j == 3 {
					want.Neg(want)
				}
				lpSparseRationalCheck(t, h, &m.Costs.Data[j], want)
			}
			lpSparseRationalCheck(t, h, &x.Data[j], values[j])
		}
		h.must(m.FlipCosts())
		for j := 0; j < n; j++ {
			c, _ := m.Columns.Get2(uint32(20 + 7*j))
			lpSparseRationalCheck(t, h, &c.Cost, new(big.Rat).Neg(costs[j]))
		}
		h.must(m.FlipCosts())
		for j := 0; j < n; j++ {
			c, _ := m.Columns.Get2(uint32(20 + 7*j))
			lpSparseRationalCheck(t, h, &c.Cost, costs[j])
		}
		h.must(m.Close())
		h.must(x.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalObjectiveSourceLookupAndFixedPrefixes(t *testing.T) {
	for _, kind := range []string{"empty", "absent", "unmapped", "fixed", "nil-column", "scale-assert", "scale-native-read", "missing-maps", "default-external", "fixed-cost", "short-costs", "short-scale", "signed-index", "short-core", "nil-scale", "nil-costs", "nil-output", "nil-flip"} {
		t.Run(kind, func(t *testing.T) {
			h := newLPPairHarness(t)
			m := lpRationalInputModel(t, h)
			out := h.pair("91", "0")
			one := h.pair("7/13", "0")
			c, err := m.GetOrCreateColumnInfo(10)
			h.must(err)
			c.Index = 0
			h.must(c.SetCost(&one.X))
			h.must(m.ColumnScale.resizeData(1))
			h.must(m.ColumnScale.copySlot(&m.ColumnScale.Data[0], &one.X))
			h.must(m.Costs.resizeData(1))
			m.CoreColumnsToExternal[0] = 10
			var want error
			switch kind {
			case "empty":
				m.Columns.Del(10)
				err = m.FlipCosts()
			case "absent":
				err = m.ColumnValueFromCore(999, nil, &out.X)
				lpSparseRationalCheck(t, h, &out.X, new(big.Rat))
			case "unmapped":
				c.Index = InvalidColumnIndex
				err = m.ColumnValueFromCore(10, nil, &out.X)
				lpSparseRationalCheck(t, h, &out.X, new(big.Rat))
			case "fixed":
				h.must(c.SetFixedValue(&one.X))
				err = m.ColumnValueFromCore(10, nil, &out.X)
				lpSparseRationalCheck(t, h, &out.X, big.NewRat(7, 13))
			case "nil-column":
				m.Columns.Set(10, nil)
				err = m.ColumnValueFromCore(10, nil, &out.X)
				want = ErrBounds
			case "scale-assert":
				err = m.SetScaledCost(1)
				want = ErrBounds
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				if _, ok := m.CoreColumnsToExternal[1]; ok == sourceDebugEnabled {
					t.Fatal("wrong scale assertion lookup prefix")
				}
			case "scale-native-read":
				h.must(m.ColumnScale.resizeData(0))
				err = m.SetScaledCost(0)
				want = ErrBounds
				if sourceDebugEnabled {
					want = ErrAssertion
				} else {
					if m.work == nil {
						t.Fatal("missing native cost-copy prefix")
					}
					lpSparseRationalCheck(t, h, &m.work.Data[0], big.NewRat(7, 13))
				}
			case "missing-maps":
				delete(m.CoreColumnsToExternal, 0)
				err = m.SetScaledCost(0)
				want = ErrBounds
				if v, ok := m.CoreColumnsToExternal[0]; !ok || v != 0 {
					t.Fatal("missing mapping prefix")
				}
				if v, ok := m.Columns.Get2(0); !ok || v != nil {
					t.Fatal("missing column prefix")
				}
			case "default-external":
				delete(m.CoreColumnsToExternal, 0)
				m.Columns.Set(0, c)
				err = m.SetScaledCost(0)
				lpSparseRationalCheck(t, h, &m.Costs.Data[0], big.NewRat(49, 169))
			case "fixed-cost":
				h.must(c.SetFixedValue(&one.X))
				c.UpperSet = true
				err = m.SetScaledCost(0)
				if sourceDebugEnabled {
					want = ErrAssertion
					if m.work == nil {
						t.Fatal("fixed assertion before getter")
					}
					lpSparseRationalCheck(t, h, &m.work.Data[0], big.NewRat(-7, 13))
				} else {
					lpSparseRationalCheck(t, h, &m.Costs.Data[0], big.NewRat(-49, 169))
				}
			case "short-costs":
				h.must(m.Costs.resizeData(0))
				err = m.SetScaledCost(0)
				want = ErrBounds
				if m.work == nil {
					t.Fatal("extent check erased source product")
				}
				lpSparseRationalCheck(t, h, &m.work.Data[2], big.NewRat(49, 169))
			case "short-scale":
				h.must(m.ColumnScale.resizeData(0))
				err = m.ColumnValueFromCore(10, []numeral.Rational{one.X}, &out.X)
				want = ErrBounds
			case "signed-index":
				c.Index = 0x80000001
				err = m.ColumnValueFromCore(10, nil, &out.X)
				want = ErrBounds
			case "short-core":
				err = m.ColumnValueFromCore(10, nil, &out.X)
				want = ErrBounds
			case "nil-scale":
				owner := m.ColumnScale
				m.ColumnScale = nil
				err = m.SetScaledCost(0)
				m.ColumnScale = owner
				want = ErrBounds
			case "nil-costs":
				owner := m.Costs
				m.Costs = nil
				err = m.SetScaledCost(0)
				m.Costs = owner
				want = ErrBounds
			case "nil-output":
				err = m.ColumnValueFromCore(999, nil, nil)
				want = ErrBounds
			case "nil-flip":
				// The new key 20 is visited before column 10 in libstdc++
				// order, so column 10's cost is not flipped.
				m.Columns.Set(20, nil)
				err = m.FlipCosts()
				want = ErrBounds
				lpSparseRationalCheck(t, h, &c.Cost, big.NewRat(7, 13))
			}
			if err != want {
				t.Fatal(kind, err, want)
			}
			if err != nil && m.FlipCosts() != ErrExactVectorPending {
				t.Fatal("stopped owner accepted retry")
			}
			// Restore dictionary ownership after deliberately invalidating a pointer.
			if kind == "empty" || kind == "nil-column" {
				m.Columns.Set(10, c)
			}
			if kind == "default-external" {
				m.Columns.Del(0)
			}
			h.must(m.Close())
			lpExactVectorNamedCensus(t, h, nil)
			if m.FlipCosts() != ErrExactVectorPending || m.SetScaledCost(0) != ErrExactVectorPending || m.ColumnValueFromCore(0, nil, &out.X) != ErrExactVectorPending {
				t.Fatal("closed entry")
			}
		})
	}
}

func TestLPRationalObjectivePhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		for _, phase := range []string{"flip", "scaled", "scaled-flipped", "free", "low", "upper", "fixed"} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 450 && !done; ordinal++ {
				if !t.Run(fmt.Sprintf("direct%t/%s/%d", direct, phase, ordinal), func(t *testing.T) {
					h := newLPPairHarness(t)
					if direct {
						h = lpNewDirectSparseHarness(t)
					}
					m := lpRationalInputModel(t, h)
					large := h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")
					other := h.pair(strings.Repeat("9876543210", 20)+"1/17", "0")
					out := h.pair("91", "0")
					x := NewIndexedRationalVector(h.m, 1)
					h.must(x.copySlot(&x.Data[0], &large.X))
					h.must(m.ColumnScale.resizeData(1))
					h.must(m.ColumnScale.copySlot(&m.ColumnScale.Data[0], &other.X))
					h.must(m.Costs.resizeData(1))
					for j := uint32(10); j < 12; j++ {
						c, e := m.GetOrCreateColumnInfo(j)
						h.must(e)
						c.Index = 0
						h.must(c.SetCost(&large.X))
						if phase == "low" {
							h.must(c.SetLowBound(&other.X))
						}
						if phase == "upper" || phase == "scaled-flipped" {
							h.must(c.SetUpperBound(&other.X))
						}
						if phase == "fixed" {
							h.must(c.SetFixedValue(&other.X))
						}
					}
					m.CoreColumnsToExternal[0] = 10
					h.budget.FailAfter(ordinal)
					var err error
					switch phase {
					case "flip":
						err = m.FlipCosts()
					case "scaled", "scaled-flipped":
						err = m.SetScaledCost(0)
					default:
						err = m.ColumnValueFromCore(10, x.Data, &out.X)
					}
					h.budget.FailAfter(-1)
					if err == nil {
						done = true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if m.FlipCosts() != ErrExactVectorPending {
							t.Fatal("stopped retry")
						}
					}
					h.must(m.Close())
					h.must(x.Close())
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !done || direct && failures == 0 {
				t.Fatal("incomplete physical campaign", direct, phase, failures)
			}
		}
	}
}

func TestLPRationalObjectiveActualMPSPreparation(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	text := "NAME COSTS\nROWS\n N OBJ\n E ROW\nCOLUMNS\n" + lpExactMPSPair("X", "OBJ", "7/13") + lpExactMPSPair("X", "ROW", "3/17") + lpExactMPSPair("Y", "OBJ", "-11/19") + lpExactMPSPair("Y", "ROW", "5/23") + "RHS\n" + lpExactMPSPair("R", "ROW", "29/31") + "BOUNDS\n" + lpMPSBoundLine("FR", "X") + lpMPSBoundLine("FR", "Y") + lpMPSBoundLine("UP", "Y 37/41") + "ENDATA\n"
	r := lpReadExactMPS(t, h, text)
	h.must(r.fillRationalSolver(m))
	h.must(m.FillMatrixAAndInitRightSide())
	h.must(m.FillB())
	h.must(m.Scale())
	h.must(m.Costs.resizeData(m.A.ColumnCount()))
	x := NewIndexedRationalVector(h.m, m.A.ColumnCount())
	out := h.pair("91", "0")
	for j := uint32(0); j < m.A.ColumnCount(); j++ {
		h.must(m.SetScaledCost(j))
		h.must(x.copySlot(&x.Data[j], &h.pair("2/7", "0").X))
		ext := m.CoreColumnsToExternal[j]
		h.must(m.ColumnValueFromCore(ext, x.Data, &out.X))
		want, cost := big.NewRat(2, 7), big.NewRat(7, 13)
		switch ext {
		case lpTestNamedColumn(t, m.NamesToColumns, "X"):
		case lpTestNamedColumn(t, m.NamesToColumns, "Y"):
			// Source FR is sticky: a later UP does not re-enable bounds.
			cost.SetFrac64(-11, 19)
			c, _ := m.Columns.Get2(ext)
			if c.LowSet || c.UpperSet {
				t.Fatal("source free bound was overwritten")
			}
		default:
			t.Fatal("unexpected populated MPS column", ext)
		}
		lpSparseRationalCheck(t, h, &m.Costs.Data[j], cost)
		lpSparseRationalCheck(t, h, &out.X, want)
	}
	h.must(m.Close())
	h.must(x.Close())
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalObjectiveBorrowedCoreAndAlias(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	foreign := newLPPairHarness(t)
	c, err := m.GetOrCreateColumnInfo(10)
	h.must(err)
	c.Index = 0
	h.must(c.SetLowBound(&h.pair("7/13", "0").X))
	h.must(m.ColumnScale.resizeData(1))
	h.must(m.ColumnScale.copySlot(&m.ColumnScale.Data[0], &h.pair("3/17", "0").X))
	x := NewIndexedRationalVector(h.m, 1)
	h.must(x.copySlot(&x.Data[0], &h.pair("11/19", "0").X))
	// The return-value assignment occurs after reading the borrowed core slot.
	h.must(m.ColumnValueFromCore(10, x.Data, &x.Data[0]))
	lpSparseRationalCheck(t, h, &x.Data[0], new(big.Rat).Add(big.NewRat(33, 323), big.NewRat(7, 13)))
	foreignValue := foreign.pair(strings.Repeat("9876543210", 20)+"/19", "0")
	out := h.pair("91", "0")
	if err = m.ColumnValueFromCore(10, []numeral.Rational{foreignValue.X}, &out.X); err == nil {
		t.Fatal("foreign core accepted")
	}
	h.must(m.Close())
	h.must(x.Close())
	lpExactVectorNamedCensus(t, h, nil)
	lpSparseRationalCheck(t, foreign, &foreignValue.X, func() *big.Rat { v, _ := new(big.Rat).SetString(strings.Repeat("9876543210", 20) + "/19"); return v }())
	lpExactVectorNamedCensus(t, foreign, nil)
}

func TestLPRationalObjectivePreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		large := h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")
		h.must(m.SetCostForColumn(10, &large.X))
		v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
		var caller exactEtaCalls
		b, err := caller.begin(h.m)
		h.must(err)
		h.budget.FailAfter(ordinal)
		_, err = h.m.Compare(&v.X, &v.Y)
		h.budget.FailAfter(-1)
		err = caller.after(h.m, b, err)
		older := h.m.ComparisonInvocationState()
		if older.Active {
			matched = true
			if !lpExactPriorityPhysicalRefusal(err) {
				t.Fatal(err)
			}
			_ = m.FlipCosts()
			if m.calls.comparison != 0 {
				t.Fatal("adopted caller comparison")
			}
		}
		h.must(m.Close())
		if older.Active && h.m.ComparisonInvocationState() != older {
			t.Fatal("consumed caller comparison")
		}
		h.must(caller.cleanupRaw(h.m))
		h.must(caller.cleanupRoots(h.m))
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !matched {
		t.Fatal("no actual older comparison")
	}
}

package lp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func lpRationalCornerExtrema(a, lo, up []*big.Rat) (min, max *big.Rat, argmin, argmax []*big.Rat) {
	for mask := 0; mask < 1<<len(a); mask++ {
		sum := new(big.Rat)
		point := make([]*big.Rat, len(a))
		for j := range a {
			point[j] = lo[j]
			if mask&(1<<j) != 0 {
				point[j] = up[j]
			}
			sum.Add(sum, new(big.Rat).Mul(a[j], point[j]))
		}
		if min == nil || sum.Cmp(min) < 0 {
			min = new(big.Rat).Set(sum)
			argmin = point
		}
		if max == nil || sum.Cmp(max) > 0 {
			max = new(big.Rat).Set(sum)
			argmax = point
		}
	}
	return
}
func lpRationalPresolveRow(t *testing.T, h *lpPairHarness, m *RationalLPSolver, a, lo, up []*big.Rat) *StdUnorderedRationalMap {
	t.Helper()
	for j := range a {
		h.must(m.SetRowColumnCoefficient(100, uint32(10+7*j), &h.pair(a[j].RatString(), "0").X))
		h.must(m.SetLowBound(uint32(10+7*j), &h.pair(lo[j].RatString(), "0").X))
		h.must(m.SetUpperBound(uint32(10+7*j), &h.pair(up[j].RatString(), "0").X))
	}
	row, _ := m.AValues.Get2(100)
	return row
}
func TestLPRationalPresolveIndependentCornerExtrema(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 200; world++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		n := 1 + world%4
		a, lo, up := make([]*big.Rat, n), make([]*big.Rat, n), make([]*big.Rat, n)
		for j := range a {
			numerator := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), uint(81+world)), big.NewInt(int64(j+1)))
			if (world+j)%2 == 0 {
				numerator.Neg(numerator)
			}
			a[j] = new(big.Rat).SetFrac(numerator, big.NewInt(17))
			if world%7 == 0 && j == n-1 {
				a[j] = new(big.Rat)
			}
			lo[j] = big.NewRat(int64(world-91-j*3), 19)
			up[j] = new(big.Rat).Add(lo[j], big.NewRat(int64(j+2), 23))
		}
		row := lpRationalPresolveRow(t, h, m, a, lo, up)
		min, max, _, _ := lpRationalCornerExtrema(a, lo, up)
		out := h.pair("91", "0")
		known, err := m.MinimalRowValue(row, &out.X)
		h.must(err)
		if !known {
			t.Fatal("boxed minimum unknown")
		}
		lpSparseRationalCheck(t, h, &out.X, min)
		known, err = m.MaximalRowValue(row, &out.X)
		h.must(err)
		if !known {
			t.Fatal("boxed maximum unknown")
		}
		lpSparseRationalCheck(t, h, &out.X, max)
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalPresolveUnknownBoundsAndZeroRead(t *testing.T) {
	for _, maximum := range []bool{false, true} {
		for flags := 0; flags < 4; flags++ {
			for _, coefficient := range []int64{-1, 0, 1} {
				h := newLPPairHarness(t)
				m := lpRationalInputModel(t, h)
				out := h.pair("91", "0")
				lowFirst := coefficient > 0
				if maximum {
					lowFirst = coefficient < 0
				}
				if lowFirst && flags&1 == 0 || !lowFirst && flags&2 == 0 {
					// Inserted first, the missing column 30 is visited last
					// in libstdc++ order (20 10 30): the walk must stop at
					// the unknown bound of 20 before reaching it.
					h.must(m.SetRowColumnCoefficient(100, 30, &h.pair("1", "0").X))
				}
				h.must(m.SetRowColumnCoefficient(100, 10, &h.pair("2", "0").X))
				h.must(m.SetLowBound(10, &h.pair("3", "0").X))
				h.must(m.SetUpperBound(10, &h.pair("5", "0").X))
				h.must(m.SetRowColumnCoefficient(100, 20, &h.pair(fmt.Sprint(coefficient), "0").X))
				h.must(m.SetFixedValue(20, &h.pair("99", "0").X))
				if flags&1 != 0 {
					h.must(m.SetLowBound(20, &h.pair("-7", "0").X))
				}
				if flags&2 != 0 {
					h.must(m.SetUpperBound(20, &h.pair("11", "0").X))
				}
				low := coefficient > 0
				if maximum {
					low = coefficient < 0
				}
				wantKnown := flags&2 != 0
				if low {
					wantKnown = flags&1 != 0
				}
				row, _ := m.AValues.Get2(100)
				var known bool
				var err error
				if maximum {
					known, err = m.MaximalRowValue(row, &out.X)
				} else {
					known, err = m.MinimalRowValue(row, &out.X)
				}
				h.must(err)
				if known != wantKnown {
					t.Fatal(maximum, flags, coefficient, known)
				}
				base := int64(6)
				if maximum {
					base = 10
				}
				// libstdc++ visits column 20 before 10, so an unknown bound
				// stops the walk before column 10's contribution.
				want := new(big.Rat)
				if wantKnown {
					endpoint := int64(11)
					if low {
						endpoint = -7
					}
					want.Add(big.NewRat(base, 1), big.NewRat(coefficient*endpoint, 1))
				}
				lpSparseRationalCheck(t, h, &out.X, want)
				if _, exists := m.Columns.Get2(30); exists {
					t.Fatal("unknown bound reached later column")
				}
				h.must(m.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
}

func TestLPRationalPresolveIndependentRowClassificationAndPinning(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 80; world++ {
		n := 2 + world%3
		a, lo, up := make([]*big.Rat, n), make([]*big.Rat, n), make([]*big.Rat, n)
		for j := range a {
			a[j] = big.NewRat(int64(1+world*3+j), 17)
			if j%2 == 0 {
				a[j].Neg(a[j])
			}
			lo[j] = big.NewRat(int64(world-j-31), 19)
			up[j] = new(big.Rat).Add(lo[j], big.NewRat(int64(j+2), 23))
		}
		min, max, argmin, argmax := lpRationalCornerExtrema(a, lo, up)
		delta := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(100), nil))
		positions := []*big.Rat{new(big.Rat).Sub(min, delta), min, new(big.Rat).Quo(new(big.Rat).Add(min, max), big.NewRat(2, 1)), max, new(big.Rat).Add(max, delta)}
		for _, relation := range []LPRelation{RelationLessOrEqual, RelationEqual, RelationGreaterOrEqual} {
			for position, rhs := range positions {
				h := newLPPairHarness(t)
				m := lpRationalInputModel(t, h)
				row := lpRationalPresolveRow(t, h, m, a, lo, up)
				h.must(m.AddConstraint(relation, &h.pair(rhs.RatString(), "0").X, 100))
				m.Settings.RefactorTolerance = []float64{0, -1, 1e100, math.Inf(1), math.NaN()}[world%5]
				obsolete, err := m.RowIsObsolete(row, 100)
				h.must(err)
				bad := relation != RelationGreaterOrEqual && rhs.Cmp(min) < 0 || relation != RelationLessOrEqual && rhs.Cmp(max) > 0
				pinLow := !bad && relation != RelationGreaterOrEqual && rhs.Cmp(min) == 0
				pinUp := !bad && relation != RelationLessOrEqual && rhs.Cmp(max) == 0
				if obsolete != (bad || pinLow || pinUp) || (m.Status == StatusInfeasible) != bad {
					t.Fatal(world, relation, position, obsolete, m.Status)
				}
				for j := range a {
					c, _ := m.Columns.Get2(uint32(10 + 7*j))
					if c.Fixed != (pinLow || pinUp) {
						t.Fatal("fixed publication", world, j)
					}
					if pinLow {
						lpSparseRationalCheck(t, h, &c.FixedValue, argmin[j])
					}
					if pinUp {
						lpSparseRationalCheck(t, h, &c.FixedValue, argmax[j])
					}
				}
				h.must(m.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
}

func TestLPRationalPresolveZeroRowsAndUnknownRelations(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, relation := range []LPRelation{RelationLessOrEqual, RelationEqual, RelationGreaterOrEqual} {
			for _, rhs := range []int64{-1, 0, 1} {
				h := newLPPairHarness(t)
				m := lpRationalInputModel(t, h)
				row := NewStdUnorderedRationalMap()
				if explicit {
					h.must(m.SetRowColumnCoefficient(100, 10, &h.pair("0", "0").X))
					row, _ = m.AValues.Get2(100)
				}
				h.must(m.AddConstraint(relation, &h.pair(fmt.Sprint(rhs), "0").X, 100))
				obsolete, err := m.RowIsObsolete(row, 100)
				h.must(err)
				bad := rhs != 0
				if relation == RelationLessOrEqual {
					bad = rhs < 0
				}
				if relation == RelationGreaterOrEqual {
					bad = rhs > 0
				}
				if !obsolete || (m.Status == StatusInfeasible) != bad {
					t.Fatal(relation, rhs, obsolete, m.Status)
				}
				if m.Columns.Len() != 0 {
					t.Fatal("zero row inspected bounds")
				}
				h.must(m.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	h.must(m.AddConstraint(LPRelation(99), &h.pair("7/13", "0").X, 100))
	c, _ := m.Constraints.Get2(100)
	saved := c.RightSide
	c.RightSide = nil
	obsolete, err := m.RowIsObsolete(nil, 100)
	want := error(nil)
	if sourceDebugEnabled {
		want = ErrAssertion
	}
	if obsolete || err != want || m.work != nil {
		t.Fatal("unknown kind reads neither RHS nor row", err)
	}
	c.RightSide = saved
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalPresolveFixedEliminationAndRelease(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 120; world++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		rhs := big.NewRat(int64(world-51), 19)
		want := new(big.Rat).Set(rhs)
		h.must(m.AddConstraint(RelationEqual, &h.pair(rhs.RatString(), "0").X, 100))
		var removed []*numeral.Rational
		for j := 0; j < 5; j++ {
			n := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), uint(81+world)), big.NewInt(int64(j+1)))
			a := new(big.Rat).SetFrac(n, big.NewInt(17))
			if j == 3 {
				a = new(big.Rat)
			}
			h.must(m.SetRowColumnCoefficient(100, uint32(j), &h.pair(a.RatString(), "0").X))
			_, err := m.GetOrCreateColumnInfo(uint32(j))
			h.must(err)
			if j%2 == 0 {
				v := big.NewRat(int64(world+j-31), 23)
				h.must(m.SetFixedValue(uint32(j), &h.pair(v.RatString(), "0").X))
				want.Sub(want, new(big.Rat).Mul(a, v))
			}
			if j%2 == 0 || j == 3 {
				row, _ := m.AValues.Get2(100)
				v, _ := row.Get2(uint32(j))
				removed = append(removed, v)
			}
		}
		row, _ := m.AValues.Get2(100)
		h.must(m.RemoveFixedOrZeroColumnsFromRow(100, row))
		if row.Len() != 1 {
			t.Fatal("remaining extent", row.Len())
		}
		if _, ok := row.Get2(1); !ok {
			t.Fatal("remaining identity")
		}
		c, _ := m.Constraints.Get2(100)
		lpSparseRationalCheck(t, h, c.RightSide, want)
		for _, v := range removed {
			zero, err := h.m.IsZero(v)
			h.must(err)
			if !zero {
				t.Fatal("erased coefficient retained its numerical owner")
			}
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	// The exact default RHS is initialized zero; its relation is indeterminate.
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	h.must(m.SetFixedValue(10, &h.pair("2", "0").X))
	h.must(m.SetRowColumnCoefficient(100, 10, &h.pair("7/13", "0").X))
	row, _ := m.AValues.Get2(100)
	h.must(m.RemoveFixedOrZeroColumnsFromRow(100, row))
	c, _ := m.Constraints.Get2(100)
	lpSparseRationalCheck(t, h, c.RightSide, big.NewRat(-14, 13))
	if c.relationSet || row.Len() != 0 {
		t.Fatal("default constraint relation")
	}
	if _, err := m.RowIsObsolete(row, 100); err != ErrBounds {
		t.Fatal("read indeterminate relation", err)
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalPresolveDeferredErasureAndFailurePrefixes(t *testing.T) {
	for _, phase := range []string{"rows", "columns", "pin"} {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		switch phase {
		case "rows":
			m.AValues.Set(100, NewStdUnorderedRationalMap())
			m.AValues.Set(101, NewStdUnorderedRationalMap())
			h.must(m.AddConstraint(RelationEqual, &h.pair("0", "0").X, 100))
			h.must(m.AddConstraint(RelationEqual, &h.pair("1", "0").X, 101))
			n, err := m.TryToRemoveSomeRows()
			h.must(err)
			if n != 0 || m.AValues.Len() != 2 || m.Status != StatusInfeasible {
				t.Fatal("infeasible pass applied queued row erasures")
			}
		case "columns":
			h.must(m.AddConstraint(RelationEqual, &h.pair("7", "0").X, 100))
			h.must(m.SetFixedValue(10, &h.pair("2", "0").X))
			// Inserting 20 before 10 makes libstdc++ visit column 10 first.
			h.must(m.SetRowColumnCoefficient(100, 20, &h.pair("1", "0").X))
			h.must(m.SetRowColumnCoefficient(100, 10, &h.pair("3", "0").X))
			row, _ := m.AValues.Get2(100)
			want := ErrBounds
			if sourceDebugEnabled {
				want = ErrAssertion
			}
			if err := m.RemoveFixedOrZeroColumnsFromRow(100, row); err != want || row.Len() != 2 {
				t.Fatal("late elimination", err, row.Len())
			}
			c, _ := m.Constraints.Get2(100)
			lpSparseRationalCheck(t, h, c.RightSide, big.NewRat(1, 1))
		case "pin":
			// Inserting 20 before 10 makes libstdc++ visit column 10 first.
			h.must(m.SetRowColumnCoefficient(100, 20, &h.pair("1", "0").X))
			h.must(m.SetRowColumnCoefficient(100, 10, &h.pair("3", "0").X))
			h.must(m.SetUpperBound(10, &h.pair("2", "0").X))
			row, _ := m.AValues.Get2(100)
			if err := m.PinVarsOnRowWithSign(row, &h.pair("1", "0").X); err != ErrBounds {
				t.Fatal(err)
			}
			c, _ := m.Columns.Get2(10)
			if !c.Fixed {
				t.Fatal("late pin error erased first fixed flag")
			}
			lpSparseRationalCheck(t, h, &c.FixedValue, big.NewRat(2, 1))
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

// The first pass removes several rows; its pinned x then fixes a later
// inequality, which is removed on pass two. Source output counts passes.
func lpRationalCleanupChain(t *testing.T, h *lpPairHarness, m *RationalLPSolver) {
	t.Helper()
	h.must(m.SetLowBound(10, &h.pair("0", "0").X))
	h.must(m.SetUpperBound(10, &h.pair("2", "0").X))
	for i := uint32(100); i < 103; i++ {
		h.must(m.SetRowColumnCoefficient(i, 10, &h.pair("1", "0").X))
		h.must(m.AddConstraint(RelationEqual, &h.pair("0", "0").X, i))
	}
	h.must(m.SetRowColumnCoefficient(103, 10, &h.pair("1", "0").X))
	h.must(m.AddConstraint(RelationLessOrEqual, &h.pair("1", "0").X, 103))
}
func TestLPRationalPresolveCleanupPassesAndDiagnostics(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	lpRationalCleanupChain(t, h, m)
	var out bytes.Buffer
	m.Settings.DebugOut = &out
	h.must(m.Cleanup())
	if m.AValues.Len() != 0 || m.Constraints.Len() != 4 || out.String() != "deleted 2 rows\n" {
		t.Fatal("source cleanup count", out.String(), m.AValues.Len())
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
	refused := errors.New("cleanup diagnostic refused")
	done := false
	for accepted := 0; accepted < 30 && !done; accepted++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		lpRationalCleanupChain(t, h, m)
		w := &lpValidationWriter{allowed: accepted, err: refused}
		m.Settings.DebugOut = w
		err := m.Cleanup()
		if err == nil {
			done = true
		} else if err != refused {
			t.Fatal(err)
		}
		if m.AValues.Len() != 0 {
			t.Fatal("diagnostic refusal preceded completed cleanup")
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !done {
		t.Fatal("diagnostic sequence incomplete")
	}
}

func TestLPRationalPresolvePhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		for _, phase := range []string{"minimum", "maximum", "pin", "obsolete-low", "obsolete-up", "remove", "cleanup"} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 650 && !done; ordinal++ {
				if !t.Run(fmt.Sprintf("direct%t/%s/%d", direct, phase, ordinal), func(t *testing.T) {
					h := newLPPairHarness(t)
					if direct {
						h = lpNewDirectSparseHarness(t)
					}
					m := lpRationalInputModel(t, h)
					m.Settings.DebugOut = nil
					text := strings.Repeat("1234567890", 20) + "7/13"
					highText := "3" + text
					large, upper := h.pair(text, "0"), h.pair(highText, "0")
					out := h.pair("91", "0")
					for j := uint32(10); j < 12; j++ {
						h.must(m.SetRowColumnCoefficient(100, j, &large.X))
						h.must(m.SetLowBound(j, &large.X))
						h.must(m.SetUpperBound(j, &upper.X))
					}
					v, _ := new(big.Rat).SetString(text)
					u, _ := new(big.Rat).SetString(highText)
					endpoint := v
					if phase == "obsolete-up" {
						endpoint = u
					}
					rhs := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(v, endpoint))
					rel := RelationEqual
					if phase == "obsolete-up" {
						rel = RelationGreaterOrEqual
					}
					h.must(m.AddConstraint(rel, &h.pair(rhs.RatString(), "0").X, 100))
					if phase == "remove" {
						for j := uint32(10); j < 12; j++ {
							h.must(m.SetFixedValue(j, &large.X))
						}
					}
					row, _ := m.AValues.Get2(100)
					h.budget.FailAfter(ordinal)
					var err error
					switch phase {
					case "minimum":
						_, err = m.MinimalRowValue(row, &out.X)
					case "maximum":
						_, err = m.MaximalRowValue(row, &out.X)
					case "pin":
						err = m.PinVarsOnRowWithSign(row, &large.X)
					case "obsolete-low", "obsolete-up":
						_, err = m.RowIsObsolete(row, 100)
					case "remove":
						err = m.RemoveFixedOrZeroColumns()
					case "cleanup":
						err = m.Cleanup()
					}
					h.budget.FailAfter(-1)
					if err == nil {
						done = true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if m.Cleanup() != ErrExactVectorPending {
							t.Fatal("stopped presolve accepted retry")
						}
					}
					h.must(m.Close())
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !done || direct && failures == 0 {
				t.Fatal("incomplete physical presolve campaign", direct, phase, failures)
			}
		}
	}
}

func TestLPRationalPresolvePreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		large := h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")
		h.must(m.SetLowBound(10, &large.X))
		h.must(m.SetRowColumnCoefficient(100, 10, &large.X))
		h.must(m.AddConstraint(RelationEqual, &large.X, 100))
		row, _ := m.AValues.Get2(100)
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
			_, _ = m.RowIsObsolete(row, 100)
			if m.calls.comparison != 0 {
				t.Fatal("presolve adopted older comparison")
			}
		}
		h.must(m.Close())
		if older.Active && h.m.ComparisonInvocationState() != older {
			t.Fatal("presolve consumed older comparison")
		}
		h.must(caller.cleanupRaw(h.m))
		h.must(caller.cleanupRoots(h.m))
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !matched {
		t.Fatal("no actual older comparison")
	}
}

func TestLPRationalPresolvePinSignsAndMissingBoundAssertions(t *testing.T) {
	for _, sign := range []int64{-2, 0, 2} {
		for _, a := range []int64{-3, 0, 3} {
			for flags := 0; flags < 4; flags++ {
				h := newLPPairHarness(t)
				m := lpRationalInputModel(t, h)
				h.must(m.SetRowColumnCoefficient(100, 10, &h.pair(fmt.Sprint(a), "0").X))
				_, err := m.GetOrCreateColumnInfo(10)
				h.must(err)
				if flags&1 != 0 {
					h.must(m.SetLowBound(10, &h.pair("-7", "0").X))
				}
				if flags&2 != 0 {
					h.must(m.SetUpperBound(10, &h.pair("11", "0").X))
				}
				row, _ := m.AValues.Get2(100)
				upper := a*sign > 0
				set := flags&1 != 0
				v := int64(-7)
				if upper {
					set = flags&2 != 0
					v = 11
				}
				if !set {
					v = 0
				}
				err = m.PinVarsOnRowWithSign(row, &h.pair(fmt.Sprint(sign), "0").X)
				c, _ := m.Columns.Get2(10)
				if sourceDebugEnabled && !set {
					if err != ErrAssertion || c.Fixed {
						t.Fatal(sign, a, flags, err, c.Fixed)
					}
				} else {
					h.must(err)
					if !c.Fixed {
						t.Fatal("pin flag")
					}
					lpSparseRationalCheck(t, h, &c.FixedValue, big.NewRat(v, 1))
				}
				h.must(m.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
	// Zero extrema use an upper bound, but zero pin products select lower.
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	// Inserting 20 before 10 makes libstdc++ visit column 10 first.
	h.must(m.SetRowColumnCoefficient(100, 20, &h.pair("0", "0").X))
	h.must(m.SetRowColumnCoefficient(100, 10, &h.pair("1", "0").X))
	h.must(m.SetLowBound(10, &h.pair("0", "0").X))
	h.must(m.SetUpperBound(10, &h.pair("2", "0").X))
	h.must(m.SetUpperBound(20, &h.pair("9", "0").X))
	h.must(m.AddConstraint(RelationEqual, &h.pair("0", "0").X, 100))
	row, _ := m.AValues.Get2(100)
	obsolete, err := m.RowIsObsolete(row, 100)
	if sourceDebugEnabled {
		if err != ErrAssertion || obsolete {
			t.Fatal(err, obsolete)
		}
	} else {
		h.must(err)
		if !obsolete {
			t.Fatal("native zero pin result")
		}
	}
	c, _ := m.Columns.Get2(10)
	if !c.Fixed {
		t.Fatal("first pin was lost")
	}
	lpSparseRationalCheck(t, h, &c.FixedValue, new(big.Rat))
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalPresolveOneSidedClassification(t *testing.T) {
	for _, maximumKnown := range []bool{false, true} {
		for _, relation := range []LPRelation{RelationEqual, RelationLessOrEqual, RelationGreaterOrEqual} {
			for offset := int64(-1); offset <= 1; offset++ {
				h := newLPPairHarness(t)
				m := lpRationalInputModel(t, h)
				h.must(m.SetRowColumnCoefficient(100, 10, &h.pair("1", "0").X))
				h.must(m.SetRowColumnCoefficient(100, 20, &h.pair("-1", "0").X))
				h.must(m.SetLowBound(20, &h.pair("-1", "0").X))
				h.must(m.SetUpperBound(20, &h.pair("3", "0").X))
				endpoint := int64(-3)
				if maximumKnown {
					h.must(m.SetUpperBound(10, &h.pair("5", "0").X))
					endpoint = 6
				} else {
					h.must(m.SetLowBound(10, &h.pair("0", "0").X))
				}
				h.must(m.AddConstraint(relation, &h.pair(fmt.Sprint(endpoint+offset), "0").X, 100))
				row, _ := m.AValues.Get2(100)
				obsolete, err := m.RowIsObsolete(row, 100)
				h.must(err)
				used := relation != RelationGreaterOrEqual
				if maximumKnown {
					used = relation != RelationLessOrEqual
				}
				bad := used && offset < 0
				if maximumKnown {
					bad = used && offset > 0
				}
				pin := used && offset == 0
				if obsolete != (bad || pin) || (m.Status == StatusInfeasible) != bad {
					t.Fatal(maximumKnown, relation, offset, obsolete, m.Status)
				}
				h.must(m.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
}

func TestLPRationalPresolveReachedGuards(t *testing.T) {
	for phase := 0; phase < 15; phase++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		out := h.pair("91", "0")
		v := h.pair("7/13", "0")
		row := NewStdUnorderedRationalMap()
		var err error
		switch phase {
		case 0:
			_, err = m.MinimalRowValue(nil, &out.X)
		case 1:
			_, err = m.MaximalRowValue(row, nil)
		case 2:
			err = m.PinVarsOnRowWithSign(row, nil)
		case 3:
			err = m.PinVarsOnRowWithSign(nil, &v.X)
		case 4:
			row.Set(10, nil)
			_, err = m.MinimalRowValue(row, &out.X)
		case 5:
			h.must(m.SetRowColumnCoefficient(100, 10, &v.X))
			row, _ = m.AValues.Get2(100)
			_, err = m.MaximalRowValue(row, &out.X)
		case 6:
			_, err = m.RowIsObsolete(row, 100)
			c, exists := m.Constraints.Get2(100)
			if !exists || c.relationSet {
				t.Fatal("default insertion")
			}
		case 7:
			m.Constraints.Set(100, nil)
			_, err = m.RowIsObsolete(row, 100)
		case 8:
			h.must(m.AddConstraint(RelationEqual, &v.X, 100))
			c, _ := m.Constraints.Get2(100)
			c.RightSide = nil
			_, err = m.RowIsObsolete(row, 100)
		case 9:
			h.must(m.AddConstraint(RelationEqual, &v.X, 100))
			_, err = m.RowIsObsolete(nil, 100)
		case 10:
			h.must(m.AddConstraint(RelationEqual, &v.X, 100))
			row.Set(10, nil)
			_, err = m.RowIsObsolete(row, 100)
		case 11:
			err = m.RemoveFixedOrZeroColumnsFromRow(100, nil)
		case 12:
			m.Constraints.Set(100, nil)
			err = m.RemoveFixedOrZeroColumnsFromRow(100, row)
		case 13:
			h.must(m.SetRowColumnCoefficient(100, 10, &v.X))
			h.must(m.SetLowBound(10, &v.X))
			row, _ = m.AValues.Get2(100)
			h.must(m.AddConstraint(RelationLessOrEqual, &v.X, 100))
			m.Settings = nil
			_, err = m.RowIsObsolete(row, 100)
		case 14:
			row.Set(10, nil)
			err = m.PinVarsOnRowWithSign(row, &v.X)
		}
		if err != ErrBounds {
			t.Fatal(phase, err)
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	for _, op := range []string{"minimum", "maximum", "pin", "row", "remove-row", "remove-all", "try", "cleanup"} {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		m.pinWork = NewIndexedRationalVector(h.m, 0)
		var err error
		switch op {
		case "minimum":
			_, err = m.MinimalRowValue(nil, nil)
		case "maximum":
			_, err = m.MaximalRowValue(nil, nil)
		case "pin":
			err = m.PinVarsOnRowWithSign(nil, nil)
		case "row":
			_, err = m.RowIsObsolete(nil, 0)
		case "remove-row":
			err = m.RemoveFixedOrZeroColumnsFromRow(0, nil)
		case "remove-all":
			err = m.RemoveFixedOrZeroColumns()
		case "try":
			_, err = m.TryToRemoveSomeRows()
		case "cleanup":
			err = m.Cleanup()
		}
		if err != ErrExactVectorPending {
			t.Fatal(op, err)
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalPresolveCleanupOutputModes(t *testing.T) {
	for _, mode := range []string{"one-pass", "nil-output", "no-pass", "nil-settings", "flush", "short-write"} {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		var out bytes.Buffer
		var flush *lpLARConstraintFlushWriter
		m.Settings.DebugOut = &out
		want := error(nil)
		if mode != "no-pass" {
			for i := uint32(100); i < 103; i++ {
				m.AValues.Set(i, NewStdUnorderedRationalMap())
				h.must(m.AddConstraint(RelationEqual, &h.pair("0", "0").X, i))
			}
		}
		switch mode {
		case "nil-output":
			m.Settings.DebugOut = nil
		case "nil-settings":
			m.Settings = nil
			want = ErrBounds
		case "flush":
			want = errors.New("cleanup flush refused")
			flush = &lpLARConstraintFlushWriter{err: want}
			m.Settings.DebugOut = flush
		case "short-write":
			want = io.ErrShortWrite
			m.Settings.DebugOut = &larConstraintPrefixWriter{remaining: 2}
		}
		err := m.Cleanup()
		if err != want || m.AValues.Len() != 0 {
			t.Fatal(mode, err)
		}
		if mode == "one-pass" && out.String() != "deleted one row\n" {
			t.Fatal("source pass count", out.String())
		}
		if flush != nil && (flush.flush != 1 || flush.output.String() != "deleted one row\n") {
			t.Fatal("cleanup flush sequence")
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalPresolveActualMPSPreparationAndLoopCondition(t *testing.T) {
	for _, extraZeroRow := range []bool{false, true} {
		h := newLPPairHarness(t)
		text := "NAME MODEL\nROWS\n N COST\n E FIX\n L OPEN\n"
		if extraZeroRow {
			text += " E ZERO\n"
		}
		text += "COLUMNS\n" + lpExactMPSPair("x", "FIX", "1") + lpExactMPSPair("x", "COST", "1") + lpExactMPSPair("y", "OPEN", "3/17") + lpExactMPSPair("y", "COST", "2")
		// An empty declared row alone does not create an AValues entry. This
		// explicit zero makes the source population actually publish that row.
		if extraZeroRow {
			text += lpExactMPSPair("x", "ZERO", "0")
		}
		text += "RHS\n" + lpExactMPSPair("RHS1", "FIX", "7/13") + lpExactMPSPair("RHS1", "OPEN", "11/19") + "BOUNDS\n" + lpMPSBoundLine("FX", "x 7/13") + lpMPSBoundLine("UP", "y 5/7") + "ENDATA\n"
		r := lpReadExactMPS(t, h, text)
		m := lpRationalInputModel(t, h)
		h.must(r.fillRationalSolver(m))
		var out bytes.Buffer
		m.Settings.DebugOut = &out
		h.must(m.Cleanup())
		if extraZeroRow {
			if m.AValues.Len() != 1 || out.String() != "deleted 2 rows\n" {
				t.Fatal("two-pass MPS cleanup", m.AValues.Len(), out.String())
			}
		} else {
			row, exists := m.AValues.Get2(0)
			if !exists || row.Len() != 0 || m.AValues.Len() != 2 || out.Len() != 0 {
				t.Fatal("zero deleted rows must stop despite fixed-column progress")
			}
		}
		x, _ := m.Columns.Get2(0)
		if !x.Fixed || x.Index != InvalidColumnIndex {
			t.Fatal("fixed external column")
		}
		lpSparseRationalCheck(t, h, &x.FixedValue, big.NewRat(7, 13))
		h.must(m.FillMatrixAAndInitRightSide())
		h.must(m.FillB())
		h.must(m.Scale())
		coreRow := m.ExternalRowsToCore[1]
		if m.A.ColumnCount() != 1 || m.CoreColumnsToExternal[0] != 1 {
			t.Fatal("remaining structural column")
		}
		lpSparseRationalCheck(t, h, &m.A.Rows[coreRow][0].Value, big.NewRat(3, 17))
		lpSparseRationalCheck(t, h, &m.B.Data[coreRow], big.NewRat(11, 19))
		if !extraZeroRow {
			lpSparseRationalCheck(t, h, &m.B.Data[m.ExternalRowsToCore[0]], new(big.Rat))
		}
		h.must(m.Close())
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalPresolveDeclaredAndExplicitZeroMPSRows(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		h := newLPPairHarness(t)
		text := "NAME MODEL\nROWS\n N COST\n L OPEN\n E EMPTY\nCOLUMNS\n" + lpExactMPSPair("x", "OPEN", "1")
		if explicit {
			text += lpExactMPSPair("x", "EMPTY", "0")
		}
		text += "RHS\n" + lpExactMPSPair("RHS1", "OPEN", "1") + lpExactMPSPair("RHS1", "EMPTY", "1") + "ENDATA\n"
		r := lpReadExactMPS(t, h, text)
		m := lpRationalInputModel(t, h)
		h.must(r.fillRationalSolver(m))
		m.Settings.DebugOut = nil
		h.must(m.Cleanup())
		// Cleanup visits AValues, not Constraints. A coefficient-free declared
		// equality 0=1 is skipped; an explicit stored zero exposes infeasibility.
		if (m.Status == StatusInfeasible) != explicit {
			t.Fatal("source empty-row visitation", explicit, m.Status)
		}
		_, exists := m.AValues.Get2(1)
		if exists != explicit {
			t.Fatal("source population row publication")
		}
		c, _ := m.Constraints.Get2(1)
		lpSparseRationalCheck(t, h, c.RightSide, big.NewRat(1, 1))
		h.must(m.Close())
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalPresolveForeignBorrowedNumerals(t *testing.T) {
	for _, op := range []string{"minimum", "maximum", "pin", "obsolete", "remove"} {
		h := newLPPairHarness(t)
		foreign := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		text := strings.Repeat("1234567890", 20) + "7/13"
		v := foreign.pair(text, "0")
		out := h.pair("91", "0")
		row := NewStdUnorderedRationalMap()
		row.Set(10, &v.X)
		h.must(m.SetLowBound(10, &h.pair("1", "0").X))
		h.must(m.SetUpperBound(10, &h.pair("2", "0").X))
		h.must(m.AddConstraint(RelationEqual, &h.pair("17", "0").X, 100))
		if op == "remove" {
			h.must(m.SetFixedValue(10, &h.pair("2", "0").X))
		}
		var err error
		switch op {
		case "minimum":
			_, err = m.MinimalRowValue(row, &out.X)
		case "maximum":
			_, err = m.MaximalRowValue(row, &out.X)
		case "pin":
			err = m.PinVarsOnRowWithSign(row, &v.X)
		case "obsolete":
			_, err = m.RowIsObsolete(row, 100)
		case "remove":
			err = m.RemoveFixedOrZeroColumnsFromRow(100, row)
		}
		if err == nil || lpExactPriorityPhysicalRefusal(err) {
			t.Fatal("foreign numerical owner", op, err)
		}
		h.must(m.Close())
		want, _ := new(big.Rat).SetString(text)
		lpSparseRationalCheck(t, foreign, &v.X, want)
		lpExactVectorNamedCensus(t, h, nil)
		lpExactVectorNamedCensus(t, foreign, nil)
	}
}

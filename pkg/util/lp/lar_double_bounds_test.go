package lp

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"
)

func TestLARStrictDeltaIndependentRationalModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 400; world++ {
		h := newLPPairHarness(t)
		c := lpNewCoreScopeFixture(t, h)
		lx := big.NewRat(int64(world%17-8), 7)
		ux := new(big.Rat).Add(lx, big.NewRat(int64(world%11), 13))
		ly := big.NewRat(int64(world%13-6), 11)
		uy := big.NewRat(int64(world%7-3), 17)
		if lx.Cmp(ux) == 0 && ly.Cmp(uy) > 0 {
			ly, uy = uy, ly
		}
		initial := big.NewRat(int64(1+world%9), 5)
		want := new(big.Rat).Set(initial)
		if lx.Cmp(ux) < 0 && ly.Cmp(uy) > 0 {
			limit := new(big.Rat).Quo(new(big.Rat).Sub(ux, lx), new(big.Rat).Sub(ly, uy))
			if limit.Cmp(want) < 0 {
				want.Set(limit)
			}
		}
		delta := h.pair(initial.RatString(), "0")
		h.must(c.UpdateDelta(&delta.X, h.pair(lx.RatString(), ly.RatString()), h.pair(ux.RatString(), uy.RatString())))
		lpSparseRationalCheck(t, h, &delta.X, want)
		left := new(big.Rat).Add(lx, new(big.Rat).Mul(want, ly))
		right := new(big.Rat).Add(ux, new(big.Rat).Mul(want, uy))
		if left.Cmp(right) > 0 {
			t.Fatal("independent substituted bounds", world)
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARDoubleBoundsIndependentConversionModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 240; world++ {
		h := newLPPairHarness(t)
		c := lpNewCoreScopeFixture(t, h)
		kinds := []ColumnType{ColumnFree, ColumnLowBound, ColumnUpperBound, ColumnFixed, ColumnBoxed}
		lows, ups := make([][2]*big.Rat, 3), make([][2]*big.Rat, 3)
		delta := big.NewRat(1, 1)
		for j := 0; j < 3; j++ {
			kind := kinds[(world+j)%5]
			c.Bounds.Types.values[j] = kind
			lx := big.NewRat(int64(world%13-6), 7)
			ly := big.NewRat(int64(world%11+1), 13)
			ux := new(big.Rat).Add(lx, big.NewRat(int64(1+j), int64(1000000+world*17)))
			uy := new(big.Rat).Neg(ly)
			if kind == ColumnFixed {
				ux.Set(lx)
				uy.Set(ly)
			}
			lows[j], ups[j] = [2]*big.Rat{lx, ly}, [2]*big.Rat{ux, uy}
			h.must(c.Bounds.Low.Set(uint32(j), h.pair(lx.RatString(), ly.RatString())))
			h.must(c.Bounds.Upper.Set(uint32(j), h.pair(ux.RatString(), uy.RatString())))
			c.DLow[j], c.DUpper[j] = 91, 92
			if kind == ColumnBoxed {
				candidate := new(big.Rat).Quo(new(big.Rat).Sub(ux, lx), new(big.Rat).Sub(ly, uy))
				if candidate.Cmp(delta) < 0 {
					delta.Set(candidate)
				}
			}
		}
		actual := h.pair("0", "0")
		h.must(c.FindDeltaForStrictBoxedBounds(&actual.X))
		lpSparseRationalCheck(t, h, &actual.X, delta)
		// Source converts numerator and denominator separately, then divides.
		convert := func(r *big.Rat) float64 { n, _ := r.Num().Float64(); d, _ := r.Denom().Float64(); return n / d }
		df := convert(delta)
		if df > 0.000001 {
			df = 0.000001
		}
		wantLow, wantUp := []float64{91, 91, 91}, []float64{92, 92, 92}
		wantErr := error(nil)
		for j := 0; j < 3; j++ {
			kind := kinds[(world+j)%5]
			lowSet := kind != ColumnFree && kind != ColumnUpperBound
			upperSet := kind != ColumnFree && kind != ColumnLowBound
			if lowSet {
				wantLow[j] = convert(lows[j][0]) + float64(df*convert(lows[j][1]))
			}
			if upperSet {
				wantUp[j] = convert(ups[j][0]) + float64(df*convert(ups[j][1]))
			}
			if sourceDebugEnabled && lowSet && upperSet && !(wantUp[j] >= wantLow[j]) {
				wantErr = ErrAssertion
				break
			}
		}
		if err := c.GetBoundsForDoubleSolver(); err != wantErr {
			t.Fatal("IEEE source assertion", world, err, wantErr)
		}
		for j := 0; j < 3; j++ {
			if math.Float64bits(c.DLow[j]) != math.Float64bits(wantLow[j]) || math.Float64bits(c.DUpper[j]) != math.Float64bits(wantUp[j]) {
				t.Fatal("publication prefix", world, j, c.DLow[j], wantLow[j], c.DUpper[j], wantUp[j])
			}
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARStrictDeltaPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		success, failures := false, 0
		for ordinal := int64(0); ordinal < 1000; ordinal++ {
			done := false
			t.Run(fmt.Sprintf("%t/%d", direct, ordinal), func(t *testing.T) {
				var h *lpPairHarness
				if direct {
					h = lpNewDirectSparseHarness(t)
				} else {
					h = newLPPairHarness(t)
				}
				c := lpNewCoreScopeFixture(t, h)
				c.Bounds.Types.values[0] = ColumnBoxed
				huge := strings.Repeat("1234567890", 20)
				h.must(c.Bounds.Low.Set(0, h.pair("0", huge+"7/13")))
				h.must(c.Bounds.Upper.Set(0, h.pair(huge+"3/17", "0")))
				h.budget.FailAfter(ordinal)
				err := c.GetBoundsForDoubleSolver()
				h.budget.FailAfter(-1)
				if err == nil {
					success, done = true, true
				} else {
					failures++
					if !lpExactPriorityPhysicalRefusal(err) {
						t.Fatal(err)
					}
					if c.GetBoundsForDoubleSolver() != ErrExactVectorPending {
						t.Fatal("stopped retry")
					}
				}
				h.must(c.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
			if done {
				break
			}
		}
		if !success || (direct && failures == 0) {
			t.Fatal("physical campaign", direct, success, failures)
		}
	}
}

func TestLARDoubleBoundsReachedPrefixes(t *testing.T) {
	for _, phase := range []string{"stopped", "short-types", "short-low", "short-upper", "invalid-type"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpNewCoreScopeFixture(t, h)
			types, low, up := c.Bounds.Types.values, c.Bounds.Low.values, c.Bounds.Upper.values
			c.Bounds.Types.values[0], c.Bounds.Types.values[1], c.Bounds.Types.values[2] = ColumnLowBound, ColumnLowBound, ColumnUpperBound
			h.must(c.Bounds.Low.Set(0, h.pair("3", "1")))
			c.DLow = []float64{91, 91, 91}
			c.DUpper = []float64{92, 92, 92}
			switch phase {
			case "stopped":
				c.stopped = true
			case "short-types":
				c.Bounds.Types.values = c.Bounds.Types.values[:1]
			case "short-low":
				c.Bounds.Low.values = c.Bounds.Low.values[:1]
			case "short-upper":
				c.Bounds.Upper.values = c.Bounds.Upper.values[:1]
			case "invalid-type":
				c.Bounds.Types.values[1] = ColumnType(99)
			}
			err := c.GetBoundsForDoubleSolver()
			succeeds := phase == "invalid-type" && !sourceDebugEnabled
			if (err == nil) != succeeds {
				t.Fatal(err, succeeds)
			}
			if phase != "stopped" && phase != "short-types" && c.DLow[0] != 3.000001 {
				t.Fatal("completed first bound", c.DLow)
			}
			if !succeeds && c.GetBoundsForDoubleSolver() != ErrExactVectorPending {
				t.Fatal("stopped retry")
			}
			c.Bounds.Types.values, c.Bounds.Low.values, c.Bounds.Upper.values = types, low, up
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLARStrictDeltaAssertionAndOutputPrefixes(t *testing.T) {
	for _, phase := range []string{"precondition", "postcondition", "nil-output", "late-bound"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpNewCoreScopeFixture(t, h)
			delta := h.pair("17", "0")
			var err error
			bounds := c.Bounds.Upper.values
			switch phase {
			case "precondition":
				err = c.UpdateDelta(&delta.X, h.pair("2", "0"), h.pair("1", "0"))
			case "postcondition":
				h.must(h.m.SetInt32(&delta.X, -1))
				err = c.UpdateDelta(&delta.X, h.pair("0", "0"), h.pair("0", "1"))
			case "nil-output":
				err = c.FindDeltaForStrictBoxedBounds(nil)
			case "late-bound":
				c.Bounds.Types.values[2] = ColumnBoxed
				c.Bounds.Upper.values = c.Bounds.Upper.values[:2]
				err = c.FindDeltaForStrictBoxedBounds(&delta.X)
			}
			succeeds := (phase == "precondition" || phase == "postcondition") && !sourceDebugEnabled
			if (err == nil) != succeeds {
				t.Fatal(err, succeeds)
			}
			want := int64(17)
			if phase == "postcondition" {
				want = -1
			}
			lpSparseRationalCheck(t, h, &delta.X, big.NewRat(want, 1))
			c.Bounds.Upper.values = bounds
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLARStrictDeltaPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpNewCoreScopeFixture(t, h)
			v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
			lower, upper := h.pair("0", "0"), h.pair("0", "0")
			h.must(h.m.Copy(&lower.X, &v.Y))
			h.must(h.m.Copy(&upper.X, &v.X))
			delta := h.pair("1", "0")
			var caller exactEtaCalls
			before, err := caller.begin(h.m)
			h.must(err)
			h.budget.FailAfter(ordinal)
			_, err = h.m.Compare(&v.X, &v.Y)
			h.budget.FailAfter(-1)
			err = caller.after(h.m, before, err)
			older := h.m.ComparisonInvocationState()
			if older.Active {
				matched = true
				if !lpExactPriorityPhysicalRefusal(err) {
					t.Fatal(err)
				}
				err = c.UpdateDelta(&delta.X, lower, upper)
				if err == nil || c.calls.comparison != 0 {
					t.Fatal("adopted caller comparison", err)
				}
			}
			h.must(c.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("consumed caller comparison")
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
	if !matched {
		t.Fatal("no actual older comparison")
	}
}

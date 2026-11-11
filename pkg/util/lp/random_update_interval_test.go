package lp

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

func TestLARRandomIntervalBoundIntersection(t *testing.T) {
	for world := 0; world < 60; world++ {
		h := newLPPairHarness(t)
		r := NewRandomUpdateInterval(h.m)
		var lo, hi [2]*big.Rat
		for step := 0; step < 20; step++ {
			x := big.NewRat(int64((world*7+step*11)%31-15), int64(1+step%7))
			y := big.NewRat(int64((world*13+step*3)%17-8), int64(1+world%5))
			v := h.pair(x.RatString(), y.RatString())
			low := step%2 == 0
			if low {
				h.must(r.SetLowBound(v))
			} else {
				h.must(r.SetUpperBound(v))
			}
			current := hi
			if low {
				current = lo
			}
			cmp := 0
			if current[0] != nil {
				cmp = x.Cmp(current[0])
				if cmp == 0 {
					cmp = y.Cmp(current[1])
				}
			}
			if current[0] == nil || low && cmp >= 0 || !low && cmp <= 0 {
				current = [2]*big.Rat{new(big.Rat).Set(x), new(big.Rat).Set(y)}
			}
			if low {
				lo = current
			} else {
				hi = current
			}
			if lo[0] != nil {
				h.check(&r.Low, lo[0], lo[1])
			}
			if hi[0] != nil {
				h.check(&r.Upper, hi[0], hi[1])
			}
			empty, err := r.IsEmpty()
			h.must(err)
			wantEmpty := false
			if lo[0] != nil && hi[0] != nil {
				c := lo[0].Cmp(hi[0])
				if c == 0 {
					c = lo[1].Cmp(hi[1])
				}
				wantEmpty = c >= 0
			}
			if empty != wantEmpty {
				t.Fatal(world, step, empty, wantEmpty)
			}
			contains, err := r.Contains(v)
			h.must(err)
			wantContains := true
			for k, bound := range [][2]*big.Rat{lo, hi} {
				if bound[0] == nil {
					continue
				}
				c := x.Cmp(bound[0])
				if c == 0 {
					c = y.Cmp(bound[1])
				}
				if k == 0 && c < 0 || k == 1 && c > 0 {
					wantContains = false
				}
			}
			if contains != wantContains {
				t.Fatal(world, step, "contains", contains, wantContains)
			}
		}
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	// A singleton is empty to the shift search while both endpoints are included.
	h := newLPPairHarness(t)
	r := NewRandomUpdateInterval(h.m)
	v := h.pair("3/7", "1/11")
	h.must(r.SetLowBound(v))
	h.must(r.SetUpperBound(v))
	empty, err := r.IsEmpty()
	h.must(err)
	inside, err := r.Contains(v)
	h.must(err)
	if !empty || !inside {
		t.Fatal(empty, inside)
	}
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARRandomIntervalBasicIndependentOracle(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 80; world++ {
		for _, typ := range []ColumnType{ColumnFree, ColumnLowBound, ColumnUpperBound, ColumnBoxed, ColumnFixed} {
			h := newLPPairHarness(t)
			s := lpNewRemovalModel(t, h, "2", "3", false)
			c := s.Core
			a := big.NewRat(int64(1+world%11), int64(1+world%7))
			if world%2 != 0 {
				a.Neg(a)
			}
			n := [2]*big.Rat{big.NewRat(int64(world%9-4), 7), big.NewRat(int64(world%5-2), 11)}
			x := [2]*big.Rat{big.NewRat(int64(world%13-6), 17), big.NewRat(int64(world%7-3), 19)}
			lower := [2]*big.Rat{new(big.Rat).Sub(x[0], big.NewRat(1+int64(world%3), 23)), new(big.Rat).Sub(x[1], big.NewRat(2, 29))}
			upper := [2]*big.Rat{new(big.Rat).Add(x[0], big.NewRat(2+int64(world%5), 31)), new(big.Rat).Add(x[1], big.NewRat(3, 37))}
			nb := h.pair(n[0].RatString(), n[1].RatString())
			h.must(c.RX.Data[2].Copy(h.m, h.pair(x[0].RatString(), x[1].RatString())))
			h.must(c.Bounds.Low.Set(2, h.pair(lower[0].RatString(), lower[1].RatString())))
			h.must(c.Bounds.Upper.Set(2, h.pair(upper[0].RatString(), upper[1].RatString())))
			h.must(c.Bounds.Types.Set(2, typ))
			r := NewRandomUpdateInterval(h.m)
			h.must(r.DiminishForBasic(c, nb, 2, &h.pair(a.RatString(), "0").X))
			lowSet, upperSet := typ == ColumnBoxed || typ == ColumnFixed, typ == ColumnBoxed || typ == ColumnFixed
			var wantLow, wantUpper [2]*big.Rat
			endpoint := func(bound [2]*big.Rat) (out [2]*big.Rat) {
				for k := range out {
					out[k] = new(big.Rat).Add(n[k], new(big.Rat).Quo(new(big.Rat).Sub(x[k], bound[k]), a))
				}
				return
			}
			fromLower, fromUpper := endpoint(lower), endpoint(upper)
			switch typ {
			case ColumnLowBound:
				if a.Sign() > 0 {
					upperSet = true
					wantUpper = fromLower
				} else {
					lowSet = true
					wantLow = fromLower
				}
			case ColumnUpperBound:
				if a.Sign() > 0 {
					lowSet = true
					wantLow = fromUpper
				} else {
					upperSet = true
					wantUpper = fromUpper
				}
			case ColumnBoxed:
				if a.Sign() > 0 {
					wantLow, wantUpper = fromUpper, fromLower
				} else {
					wantLow, wantUpper = fromLower, fromUpper
				}
			case ColumnFixed:
				wantLow, wantUpper = n, n
			}
			if r.LowSet != lowSet || r.UpperSet != upperSet {
				t.Fatal(world, typ, r.LowSet, r.UpperSet)
			}
			if lowSet {
				h.check(&r.Low, wantLow[0], wantLow[1])
			}
			if upperSet {
				h.check(&r.Upper, wantUpper[0], wantUpper[1])
			}
			h.must(r.Close())
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARRandomIntervalDrawIndependentLattice(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := uint32(0); world < 120; world++ {
		for _, mode := range []string{"free", "low", "upper", "boxed", "fixed", "reversed"} {
			h := newLPPairHarness(t)
			r := NewRandomUpdateInterval(h.m)
			s, err := NewSettings()
			h.must(err)
			s.RandomSeed(world*971 + 17)
			oracle, err := NewSettings()
			h.must(err)
			oracle.RandomSeed(world*971 + 17)
			lo := [2]*big.Rat{big.NewRat(-17, 13), big.NewRat(3, 7)}
			hi := [2]*big.Rat{big.NewRat(31, 19), big.NewRat(-5, 11)}
			if mode == "fixed" {
				hi = lo
			}
			if mode == "reversed" {
				lo, hi = hi, lo
			}
			if mode != "free" && mode != "upper" {
				h.must(r.SetLowBound(h.pair(lo[0].RatString(), lo[1].RatString())))
			}
			if mode != "free" && mode != "low" {
				h.must(r.SetUpperBound(h.pair(hi[0].RatString(), hi[1].RatString())))
			}
			for step := 0; step < 4; step++ {
				draw := int64(oracle.RandomNext() % 100000)
				want := [2]*big.Rat{new(big.Rat), new(big.Rat)}
				for k := range want {
					switch mode {
					case "free":
						if k == 0 {
							want[k].SetInt64(draw)
						}
					case "low":
						want[k].Set(lo[k])
						if k == 0 {
							want[k].Add(want[k], big.NewRat(draw, 1))
						}
					case "upper":
						want[k].Set(hi[k])
						if k == 0 {
							want[k].Sub(want[k], big.NewRat(draw, 1))
						}
					default:
						want[k].Add(lo[k], new(big.Rat).Mul(big.NewRat(draw, 100000), new(big.Rat).Sub(hi[k], lo[k])))
					}
				}
				out := h.pair("97", "101")
				h.must(r.Draw(s, out))
				h.check(out, want[0], want[1])
			}
			if s.RandomNext() != oracle.RandomNext() {
				t.Fatal("RNG consumption")
			}
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARRandomIntervalActualTableauAndNonunitLU(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 80; world++ {
		for _, tableau := range []bool{false, true} {
			h := newLPPairHarness(t)
			a := big.NewRat(int64(1+world%9), int64(1+world%7))
			if world%2 != 0 {
				a.Neg(a)
			}
			basis := big.NewRat(int64(1+world%5), int64(1+world%3))
			if tableau {
				basis.SetInt64(1)
			}
			s := lpPreparedLARPivot(t, h, a.RatString(), "0", basis.RatString())
			c := s.Core
			if tableau {
				s.Settings.SimplexStrategy = StrategyTableauRows
				if world%3 == 0 {
					s.Settings.SimplexStrategy = StrategyTableauCosts
				}
			}
			coeff := new(big.Rat).Quo(a, basis)
			h.must(c.RX.Data[0].Copy(h.m, h.pair("2", "1/7")))
			x := [2]*big.Rat{new(big.Rat).Mul(new(big.Rat).Neg(coeff), big.NewRat(2, 1)), new(big.Rat).Mul(new(big.Rat).Neg(coeff), big.NewRat(1, 7))}
			h.must(c.RX.Data[2].Copy(h.m, h.pair(x[0].RatString(), x[1].RatString())))
			h.must(c.Bounds.Types.Set(0, ColumnFree))
			h.must(c.Bounds.Types.Set(2, ColumnBoxed))
			low := new(big.Rat).Sub(x[0], big.NewRat(3, 11))
			upper := new(big.Rat).Add(x[0], big.NewRat(5, 13))
			h.must(c.Bounds.Low.Set(2, h.pair(low.RatString(), x[1].RatString())))
			h.must(c.Bounds.Upper.Set(2, h.pair(upper.RatString(), x[1].RatString())))
			h.must(c.PrefixR())
			r, err := c.FindRandomShiftInterval(0)
			h.must(err)
			l := new(big.Rat).Add(big.NewRat(2, 1), new(big.Rat).Quo(new(big.Rat).Sub(x[0], upper), coeff))
			u := new(big.Rat).Add(big.NewRat(2, 1), new(big.Rat).Quo(new(big.Rat).Sub(x[0], low), coeff))
			if coeff.Sign() < 0 {
				l, u = u, l
			}
			if !r.LowSet || !r.UpperSet {
				t.Fatal("missing actual basic limits")
			}
			h.check(&r.Low, l, big.NewRat(1, 7))
			h.check(&r.Upper, u, big.NewRat(1, 7))
			inside, err := r.Contains(&c.RX.Data[0])
			h.must(err)
			if !inside {
				t.Fatal("lost current feasible value")
			}
			h.must(r.Close())
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARRandomIntervalPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		for _, op := range []string{"low", "upper", "empty", "contains", "basic", "find-tableau", "find-LU", "draw"} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 1000 && !done; ordinal++ {
				passed := t.Run(fmt.Sprintf("%v/%s/%d", direct, op, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					large := strings.Repeat("1234567890", 20) + "7/13"
					s := lpPreparedLARPivot(t, h, "3/7", "0", "5/11")
					c := s.Core
					if op == "find-tableau" {
						s.Settings.SimplexStrategy = StrategyTableauRows
					}
					h.must(c.PrefixR())
					v := h.pair(large, large)
					r := NewRandomUpdateInterval(h.m)
					h.must(c.RX.Data[2].Copy(h.m, v))
					h.must(c.Bounds.Low.Set(2, h.pair("-3/7", "-1/13")))
					h.must(c.Bounds.Upper.Set(2, h.pair(large, large)))
					h.must(c.Bounds.Types.Set(2, ColumnBoxed))
					if op == "empty" || op == "contains" || op == "draw" {
						h.must(r.SetLowBound(v))
						h.must(r.SetUpperBound(h.pair(large, "1/7")))
					}
					out := h.pair("97", "101")
					random := s.Settings.random
					drawIndex := random.Next() % 100000
					nextAfterDraw := random.Next()
					h.budget.FailAfter(ordinal)
					var err error
					var predicate bool
					switch op {
					case "low":
						err = r.SetLowBound(v)
					case "upper":
						err = r.SetUpperBound(v)
					case "empty":
						predicate, err = r.IsEmpty()
					case "contains":
						predicate, err = r.Contains(v)
					case "basic":
						err = r.DiminishForBasic(c, &c.RX.Data[0], 2, &v.X)
					case "find-tableau", "find-LU":
						h.must(r.Close())
						r, err = c.FindRandomShiftInterval(0)
					case "draw":
						err = r.Draw(s.Settings, out)
					}
					h.budget.FailAfter(-1)
					if op == "draw" && s.Settings.RandomNext() != nextAfterDraw {
						t.Fatal("draw did not consume exactly one RNG value on this prefix")
					}
					if err == nil {
						switch op {
						case "low":
							if !r.LowSet {
								t.Fatal("missing low endpoint")
							}
							h.check(&r.Low, mustLPRat(t, large), mustLPRat(t, large))
						case "upper":
							if !r.UpperSet {
								t.Fatal("missing upper endpoint")
							}
							h.check(&r.Upper, mustLPRat(t, large), mustLPRat(t, large))
						case "empty":
							if !predicate {
								t.Fatal("wrong interval predicate", op)
							}
						case "contains":
							if predicate {
								t.Fatal("reversed interval contains low endpoint")
							}
						case "draw":
							lo := mustLPRat(t, large)
							hi := big.NewRat(1, 7)
							fraction := new(big.Rat).SetFrac64(int64(drawIndex), 100000)
							y := new(big.Rat).Add(lo, new(big.Rat).Mul(fraction, new(big.Rat).Sub(hi, lo)))
							h.check(out, lo, y)
						}
						done = true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
					}
					if r != nil {
						h.must(r.Close())
					}
					h.must(s.Close())
					lpExactVectorNamedCensus(t, h, nil)
				})
				if !passed {
					t.FailNow()
				}
			}
			// Equality may finish without allocation; direct copies/arithmetic must
			// still exercise refusal. Pooled operations can reuse warmed solve storage.
			noPhysical := !direct || op == "empty" || op == "contains"
			if !done || failures == 0 && !noPhysical {
				t.Fatal("incomplete campaign", direct, op, done, failures)
			}
		}
	}
}

func TestLARRandomIntervalReachedGuards(t *testing.T) {
	for _, op := range []string{"nil-manager", "nil-low", "nil-upper", "nil-contains", "nil-draw-settings", "nil-draw-output", "stopped-low", "stopped-upper", "stopped-empty", "stopped-contains", "stopped-draw", "stopped-basic", "stopped-core", "nil-core", "foreign-core", "nil-nb", "nil-coefficient", "missing-type", "missing-x", "missing-heading", "nonbasic", "zero-free", "unknown", "bad-lower", "bad-upper", "negative-delta", "find-column", "find-type", "find-low", "find-upper", "find-unknown", "find-basis", "missing-LU", "get-stopped"} {
		t.Run(op, func(t *testing.T) {
			h := newLPPairHarness(t)
			s := lpNewRemovalModel(t, h, "2", "3", false)
			c := s.Core
			r := NewRandomUpdateInterval(h.m)
			v := h.pair("0", "0")
			a := h.pair("2", "0")
			other := newLPPairHarness(t)
			foreign, err := NewLARCoreSolver(other.m, s.Settings, nil)
			h.must(err)
			want := error(ErrBounds)
			switch op {
			case "nil-manager":
				r = NewRandomUpdateInterval(nil)
				_, err = r.IsEmpty()
				want = ErrExactVectorOwner
			case "nil-low":
				err = r.SetLowBound(nil)
			case "nil-upper":
				err = r.SetUpperBound(nil)
			case "nil-contains":
				_, err = r.Contains(nil)
			case "nil-draw-settings":
				err = r.Draw(nil, v)
			case "nil-draw-output":
				err = r.Draw(s.Settings, nil)
			case "stopped-low":
				r.stopped = true
				err = r.SetLowBound(v)
				want = ErrExactVectorPending
			case "stopped-upper":
				r.stopped = true
				err = r.SetUpperBound(v)
				want = ErrExactVectorPending
			case "stopped-empty":
				r.stopped = true
				_, err = r.IsEmpty()
				want = ErrExactVectorPending
			case "stopped-contains":
				r.stopped = true
				_, err = r.Contains(v)
				want = ErrExactVectorPending
			case "stopped-draw":
				r.stopped = true
				err = r.Draw(s.Settings, v)
				want = ErrExactVectorPending
			case "stopped-basic":
				r.stopped = true
				err = r.DiminishForBasic(c, v, 2, &a.X)
				want = ErrExactVectorPending
			case "stopped-core":
				c.stopped = true
				err = r.DiminishForBasic(c, v, 2, &a.X)
				want = ErrExactVectorPending
			case "nil-core":
				err = r.DiminishForBasic(nil, v, 2, &a.X)
				want = ErrExactVectorOwner
			case "foreign-core":
				err = r.DiminishForBasic(foreign, v, 2, &a.X)
				want = ErrExactVectorOwner
			case "nil-nb":
				err = r.DiminishForBasic(c, nil, 2, &a.X)
			case "nil-coefficient":
				err = r.DiminishForBasic(c, v, 2, nil)
			case "missing-type":
				err = r.DiminishForBasic(c, v, 99, &a.X)
			case "missing-x":
				saved := c.RX.Data
				c.RX.Data = c.RX.Data[:2]
				err = r.DiminishForBasic(c, v, 2, &a.X)
				c.RX.Data = saved
			case "missing-heading":
				c.RHeading = c.RHeading[:2]
				err = r.DiminishForBasic(c, v, 2, &a.X)
				if !sourceDebugEnabled {
					want = nil
				}
			case "nonbasic":
				c.RHeading[2] = -1
				err = r.DiminishForBasic(c, v, 2, &a.X)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
			case "zero-free":
				err = r.DiminishForBasic(c, v, 2, &v.X)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
			case "unknown":
				h.must(c.Bounds.Types.Set(2, ColumnType(77)))
				err = r.DiminishForBasic(c, v, 2, &a.X)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
			case "bad-lower":
				h.must(c.Bounds.Types.Set(2, ColumnLowBound))
				saved := c.Bounds.Low.values
				c.Bounds.Low.values = c.Bounds.Low.values[:2]
				err = r.DiminishForBasic(c, v, 2, &a.X)
				c.Bounds.Low.values = saved
			case "bad-upper":
				h.must(c.Bounds.Types.Set(2, ColumnUpperBound))
				saved := c.Bounds.Upper.values
				c.Bounds.Upper.values = c.Bounds.Upper.values[:2]
				err = r.DiminishForBasic(c, v, 2, &a.X)
				c.Bounds.Upper.values = saved
			case "negative-delta":
				h.must(c.Bounds.Types.Set(2, ColumnLowBound))
				h.must(c.Bounds.Low.Set(2, h.pair("1", "0")))
				err = r.DiminishForBasic(c, v, 2, &a.X)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
			case "find-column":
				h.must(r.Close())
				r, err = c.FindRandomShiftInterval(99)
			case "find-type":
				c.Bounds.Types.values = nil
				h.must(r.Close())
				r, err = c.FindRandomShiftInterval(0)
			case "find-low":
				h.must(c.Bounds.Types.Set(0, ColumnLowBound))
				saved := c.Bounds.Low.values
				c.Bounds.Low.values = nil
				h.must(r.Close())
				r, err = c.FindRandomShiftInterval(0)
				c.Bounds.Low.values = saved
			case "find-upper":
				h.must(c.Bounds.Types.Set(0, ColumnUpperBound))
				saved := c.Bounds.Upper.values
				c.Bounds.Upper.values = nil
				h.must(r.Close())
				r, err = c.FindRandomShiftInterval(0)
				c.Bounds.Upper.values = saved
			case "find-unknown":
				h.must(c.Bounds.Types.Set(0, ColumnType(77)))
				h.must(r.Close())
				r, err = c.FindRandomShiftInterval(0)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
			case "find-basis":
				c.RBasis = nil
				h.must(r.Close())
				r, err = c.FindRandomShiftInterval(0)
			case "missing-LU":
				s.Settings.SimplexStrategy = StrategyLU
				_, err = c.GetColumnIterator(0)
			case "get-stopped":
				c.stopped = true
				_, err = c.GetColumnIterator(0)
				want = ErrExactVectorPending
			}
			if err != want {
				t.Fatal(err, want)
			}
			if r != nil {
				h.must(r.Close())
			}
			h.must(s.Close())
			other.must(foreign.Close())
			lpExactVectorNamedCensus(t, h, nil)
			lpExactVectorNamedCensus(t, other, nil)
		})
	}
}

func TestLARRandomIntervalEmptyStopsBeforeLaterColumn(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		h := newLPPairHarness(t)
		s := lpNewRemovalModel(t, h, "2", "3", true)
		c := s.Core
		// The first actual basic row makes the interval empty. The later damaged
		// row must not be inspected, even if the interval was already empty.
		if fixed {
			h.must(c.Bounds.Types.Set(2, ColumnFixed))
		} else {
			h.must(c.Bounds.Types.Set(0, ColumnBoxed))
			h.must(c.Bounds.Low.Set(0, h.pair("3", "0")))
			h.must(c.Bounds.Upper.Set(0, h.pair("2", "0")))
		}
		c.RA.Columns[0][1].Row = 999
		r, err := c.FindRandomShiftInterval(0)
		h.must(err)
		empty, err := r.IsEmpty()
		h.must(err)
		if !empty {
			t.Fatal("did not stop on empty interval")
		}
		h.must(r.Close())
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARRandomIntervalPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			h := newLPPairHarness(t)
			r := NewRandomUpdateInterval(h.m)
			value := h.pair("3/7", "5/11")
			h.must(r.SetLowBound(value))
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
				_, err = r.Contains(value)
				if r.calls.comparison != 0 {
					t.Fatal("adopted older comparison", err)
				}
			}
			h.must(r.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("consumed older comparison")
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

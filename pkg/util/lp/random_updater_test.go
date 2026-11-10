package lp

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestLARRandomUpdaterFreeIndependentTrace(t *testing.T) {
	for world := 0; world < 120; world++ {
		h := newLPPairHarness(t)
		s, err := NewLARSolver(h.m)
		h.must(err)
		n := world % 9
		want := make([]int64, n)
		counts := map[int64]uint32{}
		var columns []uint32
		for j := 0; j < n; j++ {
			_, err = s.AddVar(uint32(17 + j))
			h.must(err)
			want[j] = int64((world + j) % 3)
			h.must(h.m.SetInt64(&s.Core.RX.Data[j].X, want[j]))
			columns = append(columns, uint32(j))
			counts[want[j]]++
			if (world+j)%3 == 0 {
				columns = append(columns, uint32(j))
				counts[want[j]]++
			}
		}
		for j := len(columns) - 1; j > 0; j-- {
			columns[j], columns[j/2] = columns[j/2], columns[j]
		}
		s.Settings.RandomSeed(uint32(world*317 + 19))
		oracle := s.Settings.random
		for j := 0; j < n && n > len(counts); j++ {
			counts[want[j]]--
			if counts[want[j]] == 0 {
				delete(counts, want[j])
			}
			want[j] = int64(oracle.Next() % 100000)
			counts[want[j]]++
		}
		u, err := NewRandomUpdater(s.Core, columns)
		h.must(err)
		h.must(u.Update())
		for j := range want {
			h.check(&s.Core.RX.Data[j], big.NewRat(want[j], 1), big.NewRat(0, 1))
		}
		if len(counts) != u.distinct {
			t.Fatal("distinct value map", u.distinct, counts)
		}
		if s.Settings.RandomNext() != oracle.Next() {
			t.Fatal("sorted traversal RNG sequence")
		}
		for _, bucket := range u.values {
			for _, entry := range bucket {
				x := mustLPRat(t, lpPairText(t, h, &entry.value)[0])
				if counts[x.Num().Int64()] != entry.count {
					t.Fatal("duplicate-request occurrence count", counts, entry.count)
				}
			}
		}
		h.must(u.Update()) // A second pass sees the retained value map and set.
		h.must(u.Close())
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func lpPairText(t *testing.T, h *lpPairHarness, p *RationalPair) [2]string {
	t.Helper()
	x, err := h.m.String(&p.X)
	h.must(err)
	y, err := h.m.String(&p.Y)
	h.must(err)
	return [2]string{x, y}
}

func TestLARRandomUpdaterActualSolveFeasibility(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts, StrategyLU} {
		for world := 0; world < 30; world++ {
			for _, strict := range []bool{false, true} {
				h := newLPPairHarness(t)
				s, _ := lpLARFullSolveFixture(t, h, strategy, world, false, strict)
				status, err := s.FindFeasibleSolution()
				h.must(err)
				if status != StatusOptimal {
					t.Fatal(status)
				}
				// Include the term and direct column duplicates, exactly as retained callers
				// can do. The core owns actual model values and the real solved basis.
				h.must(s.RandomUpdate([]uint32{LARTermStartIndex, 1, 0, LARTermStartIndex}))
				for j := range s.Core.RX.Data {
					p := &s.Core.RX.Data[j]
					typ := s.Bounds.Types.values[j]
					v := lpPairText(t, h, p)
					value := [2]*big.Rat{mustLPRat(t, v[0]), mustLPRat(t, v[1])}
					compare := func(bound *RationalPair) int {
						text := lpPairText(t, h, bound)
						cmp := value[0].Cmp(mustLPRat(t, text[0]))
						if cmp == 0 {
							cmp = value[1].Cmp(mustLPRat(t, text[1]))
						}
						return cmp
					}
					if (typ == ColumnLowBound || typ == ColumnBoxed || typ == ColumnFixed) && compare(&s.Bounds.Low.values[j]) < 0 {
						t.Fatal("lower", strategy, world, j)
					}
					if (typ == ColumnUpperBound || typ == ColumnBoxed || typ == ColumnFixed) && compare(&s.Bounds.Upper.values[j]) > 0 {
						t.Fatal("upper", strategy, world, j)
					}
				}
				for row, cells := range s.Core.RA.Rows {
					sum := [2]*big.Rat{new(big.Rat), new(big.Rat)}
					for _, cell := range cells {
						a, err := h.m.String(&cell.Value)
						h.must(err)
						v := lpPairText(t, h, &s.Core.RX.Data[cell.Column])
						coeff := mustLPRat(t, a)
						for k := range sum {
							sum[k].Add(sum[k], new(big.Rat).Mul(coeff, mustLPRat(t, v[k])))
						}
					}
					rhs := lpPairText(t, h, &s.Core.RightSides.Data[row])
					for k := range sum {
						if sum[k].Cmp(mustLPRat(t, rhs[k])) != 0 {
							t.Fatal("independent A*x=b", strategy, world, row, k, sum[k], rhs[k])
						}
					}
				}
				model := NewLARRationalModel(h.m)
				h.must(s.GetModel(model))
				for _, constraint := range s.Constraints {
					ok, err := s.ConstraintHolds(constraint, model)
					h.must(err)
					if !ok {
						t.Fatal("original constraint", strategy, world)
					}
				}
				h.must(model.Close())
				h.must(s.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
}

func TestLARRandomUpdaterBasicExpansionAndValueCounts(t *testing.T) {
	h := newLPPairHarness(t)
	s := lpNewRemovalModel(t, h, "2", "3", true)
	// Each basic row contributes its actual nonbasic columns, and duplicate basic
	// requests increment occurrence counts even when the variable set is unchanged.
	u, err := NewRandomUpdater(s.Core, []uint32{3, 2, 0, 2})
	h.must(err)
	if len(u.vars) != 2 || u.distinct != 1 {
		t.Fatal(u.vars, u.distinct)
	}
	for _, bucket := range u.values {
		if len(bucket) != 1 || bucket[0].count != 7 {
			t.Fatal("source duplicate count", bucket)
		}
	}
	h.must(u.removeValue(&s.Core.RX.Data[0]))
	h.must(u.addValue(h.pair("1/3", "2/5")))
	if u.distinct != 2 {
		t.Fatal(u.distinct)
	}
	h.must(u.removeValue(h.pair("1/3", "2/5")))
	if u.distinct != 1 {
		t.Fatal("erase unique value")
	}
	// Equality resolves hash buckets, including explicit collisions.
	key, err := u.valueKey(&s.Core.RX.Data[0])
	h.must(err)
	other := &randomUpdateValue{value: NewRationalPair(), count: 9}
	h.must(other.value.Copy(h.m, h.pair("9", "11")))
	u.owned = append(u.owned, other)
	u.values[key] = append([]*randomUpdateValue{other}, u.values[key]...)
	h.must(u.addValue(&s.Core.RX.Data[0]))
	if other.count != 9 {
		t.Fatal("hash collision treated as equal")
	}
	h.must(u.removeValue(&s.Core.RX.Data[0]))
	if other.count != 9 {
		t.Fatal("remove collision")
	}
	h.must(u.Close())
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARRandomUpdaterSourceCutoffAndEmpty(t *testing.T) {
	for _, rows := range []int{0, 1, 49, 50, 51} {
		for _, fixed := range []bool{false, true} {
			h := newLPPairHarness(t)
			s, err := NewLARSolver(h.m)
			h.must(err)
			for j := 0; j < 2; j++ {
				_, err = s.AddVar(uint32(j))
				h.must(err)
			}
			a := h.pair("2", "0")
			for row := 0; row < rows; row++ {
				_, err = s.AddTerm([]RationalSparseEntry{{Index: 0, Value: a.X}}, &a.Y)
				h.must(err)
			}
			if fixed {
				h.must(s.Bounds.Types.Set(0, ColumnFixed))
			}
			random := s.Settings.random
			u, err := NewRandomUpdater(s.Core, []uint32{0, 1})
			h.must(err)
			h.must(u.Update())
			// Skipped first column leaves the second free column as the next chance.
			shift0 := rows < 50 && !fixed
			first := int64(random.Next() % 100000)
			if shift0 {
				h.check(&s.Core.RX.Data[0], big.NewRat(first, 1), big.NewRat(0, 1))
				h.check(&s.Core.RX.Data[1], big.NewRat(0, 1), big.NewRat(0, 1))
			} else {
				h.check(&s.Core.RX.Data[0], big.NewRat(0, 1), big.NewRat(0, 1))
				h.check(&s.Core.RX.Data[1], big.NewRat(first, 1), big.NewRat(0, 1))
			}
			if s.Settings.RandomNext() != random.Next() {
				t.Fatal("cutoff/empty consumed RNG", rows, fixed)
			}
			h.must(u.Close())
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARRandomUpdaterTermExpansion(t *testing.T) {
	h := newLPPairHarness(t)
	s := lpNewRemovalModel(t, h, "2", "3", false)
	columns := []uint32{99}
	h.must(s.FillVarSetForRandomUpdate([]uint32{1, LARTermStartIndex, 0, LARTermStartIndex}, &columns))
	if !reflect.DeepEqual(columns, []uint32{99, 1, 1, 0, 0, 1, 0}) {
		t.Fatal(columns)
	}
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARRandomUpdaterPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, direct := range []bool{false, true} {
		for _, op := range []string{"constructor", "constructor-basic", "free-shift", "bounded-shift", "growing-shift", "tableau-shift", "LU-shift", "outer"} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 1600 && !done; ordinal++ {
				passed := t.Run(fmt.Sprintf("%v/%s/%d", direct, op, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					var s *LARSolver
					var err error
					large := strings.Repeat("1234567890", 20) + "7/13"
					if op == "LU-shift" || op == "tableau-shift" || op == "outer" || op == "constructor-basic" {
						basisText := "5/11"
						if op != "LU-shift" {
							basisText = "1"
						}
						s = lpPreparedLARPivot(t, h, "-3/7", "0", basisText)
						if op != "LU-shift" {
							s.Settings.SimplexStrategy = StrategyTableauRows
						}
						h.must(s.Core.PrefixR())
						for j := uint32(0); j < 3; j++ {
							h.must(s.Bounds.Types.Set(j, ColumnFree))
						}
						// Two duplicate nonbasic values force actual traversal. The basic value
						// satisfies the real equation before the first shift.
						h.must(s.Core.RX.Data[0].Copy(h.m, h.pair(large, "1/17")))
						h.must(s.Core.RX.Data[1].Copy(h.m, &s.Core.RX.Data[0]))
						basis := big.NewRat(1, 1)
						if op == "LU-shift" {
							basis = big.NewRat(5, 11)
						}
						coeff := new(big.Rat).Quo(big.NewRat(3, 7), basis)
						value := new(big.Rat).Mul(mustLPRat(t, large), coeff)
						eps := new(big.Rat).Mul(big.NewRat(1, 17), coeff)
						h.must(s.Core.RX.Data[2].Copy(h.m, h.pair(value.RatString(), eps.RatString())))
					} else {
						s, err = NewLARSolver(h.m)
						h.must(err)
						for j := 0; j < 2; j++ {
							_, err = s.AddVar(uint32(j))
							h.must(err)
							initial := h.pair(large, "1/17")
							if op == "growing-shift" {
								initial = h.pair("0", "0")
							}
							h.must(s.Core.RX.Data[j].Copy(h.m, initial))
						}
						if op == "bounded-shift" || op == "growing-shift" {
							for j := uint32(0); j < 2; j++ {
								h.must(s.Bounds.Types.Set(j, ColumnBoxed))
								h.must(s.Bounds.Low.Set(j, h.pair("-7/13", "-3/19")))
								h.must(s.Bounds.Upper.Set(j, h.pair(large, "2/17")))
							}
						}
					}
					var u *RandomUpdater
					if op != "constructor" && op != "constructor-basic" && op != "outer" {
						u, err = NewRandomUpdater(s.Core, []uint32{0, 1})
						h.must(err)
					}
					h.budget.FailAfter(ordinal)
					switch op {
					case "constructor-basic":
						u, err = NewRandomUpdater(s.Core, []uint32{2, 1, 2})
					case "constructor":
						u, err = NewRandomUpdater(s.Core, []uint32{0, 1, 0})
					case "outer":
						err = s.RandomUpdate([]uint32{0, 1})
					default:
						err = u.Update()
					}
					h.budget.FailAfter(-1)
					if err == nil {
						done = true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if u != nil && u.Update() != ErrExactVectorPending {
							t.Fatal("unfinished updater accepted retry")
						}
					}
					if u != nil {
						h.must(u.Close())
					}
					h.must(s.Close())
					lpExactVectorNamedCensus(t, h, nil)
				})
				if !passed {
					t.FailNow()
				}
			}
			if !done || failures == 0 && direct {
				t.Fatal("incomplete allocation campaign", direct, op, done, failures)
			}
		}
	}
}

func TestLARRandomUpdaterIndependentBoundedShifts(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 60; world++ {
		for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts, StrategyLU} {
			h := newLPPairHarness(t)
			a := big.NewRat(int64(1+world%11), int64(1+world%7))
			if world%2 != 0 {
				a.Neg(a)
			}
			basis := big.NewRat(1, 1)
			if strategy == StrategyLU {
				basis = big.NewRat(int64(1+world%5), int64(1+world%3))
			}
			s := lpPreparedLARPivot(t, h, a.RatString(), "2/3", basis.RatString())
			c := s.Core
			s.Settings.SimplexStrategy = strategy
			h.must(c.PrefixR())
			coeff := new(big.Rat).Quo(a, basis)
			low := [2]*big.Rat{big.NewRat(-3, 7), big.NewRat(-1, 11)}
			upper := [2]*big.Rat{big.NewRat(5, 13), big.NewRat(2, 17)}
			basicLow := [2]*big.Rat{big.NewRat(-1, 17), big.NewRat(-2, 19)}
			basicUpper := [2]*big.Rat{big.NewRat(2, 23), big.NewRat(3, 29)}
			for j := uint32(0); j < 2; j++ {
				h.must(c.Bounds.Types.Set(j, ColumnBoxed))
				h.must(c.Bounds.Low.Set(j, h.pair(low[0].RatString(), low[1].RatString())))
				h.must(c.Bounds.Upper.Set(j, h.pair(upper[0].RatString(), upper[1].RatString())))
			}
			h.must(c.Bounds.Types.Set(2, ColumnBoxed))
			h.must(c.Bounds.Low.Set(2, h.pair(basicLow[0].RatString(), basicLow[1].RatString())))
			h.must(c.Bounds.Upper.Set(2, h.pair(basicUpper[0].RatString(), basicUpper[1].RatString())))
			endpoint := func(v [2]*big.Rat) (out [2]*big.Rat) {
				for k := range out {
					out[k] = new(big.Rat).Quo(new(big.Rat).Neg(v[k]), coeff)
				}
				return
			}
			restrictedLow, restrictedUpper := endpoint(basicUpper), endpoint(basicLow)
			if coeff.Sign() < 0 {
				restrictedLow, restrictedUpper = restrictedUpper, restrictedLow
			}
			compare := func(x, y [2]*big.Rat) int {
				cmp := x[0].Cmp(y[0])
				if cmp == 0 {
					cmp = x[1].Cmp(y[1])
				}
				return cmp
			}
			if compare(restrictedLow, low) > 0 {
				low = restrictedLow
			}
			if compare(restrictedUpper, upper) < 0 {
				upper = restrictedUpper
			}
			s.Settings.RandomSeed(uint32(world*117 + 37))
			oracle := s.Settings.random
			draw := big.NewRat(int64(oracle.Next()%100000), 100000)
			want := [2]*big.Rat{}
			for k := range want {
				want[k] = new(big.Rat).Add(low[k], new(big.Rat).Mul(draw, new(big.Rat).Sub(upper[k], low[k])))
			}
			if want[0].Sign() == 0 && want[1].Sign() == 0 {
				t.Fatal("fixture needs a nonzero first shift")
			}
			h.must(s.RandomUpdate([]uint32{1, 0}))
			h.check(&c.RX.Data[0], want[0], want[1])
			h.check(&c.RX.Data[1], big.NewRat(0, 1), big.NewRat(0, 1))
			basic := [2]*big.Rat{}
			for k := range basic {
				basic[k] = new(big.Rat).Mul(new(big.Rat).Neg(coeff), want[k])
			}
			h.check(&c.RX.Data[2], basic[0], basic[1])
			if compare(basic, basicLow) < 0 || compare(basic, basicUpper) > 0 {
				t.Fatal("independent clipped basic feasibility")
			}
			if s.Settings.RandomNext() != oracle.Next() {
				t.Fatal("shift did not stop after creating distinct values")
			}
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARRandomUpdaterReachedGuardsAndPrefixes(t *testing.T) {
	for _, op := range []string{"nil-core", "stopped-core", "column", "row", "row-column", "short-x", "fill-nil", "fill-term", "outer-term", "outer-stopped", "update-stopped", "remove-missing", "pre-contains", "pre-feasible", "final-equation", "late-row", "count-wrap"} {
		t.Run(op, func(t *testing.T) {
			h := newLPPairHarness(t)
			s := lpNewRemovalModel(t, h, "2", "3", false)
			c := s.Core
			var u *RandomUpdater
			var err error
			want := error(ErrBounds)
			switch op {
			case "nil-core":
				u, err = NewRandomUpdater(nil, nil)
			case "stopped-core":
				c.stopped = true
				u, err = NewRandomUpdater(c, nil)
				want = ErrExactVectorPending
			case "column":
				u, err = NewRandomUpdater(c, []uint32{0, 99})
				if u == nil || len(u.vars) != 1 {
					t.Fatal("constructor lost earlier prefix")
				}
			case "row":
				c.RHeading[2] = 99
				u, err = NewRandomUpdater(c, []uint32{2})
			case "row-column":
				c.RA.Rows[0][0].Column = 99
				u, err = NewRandomUpdater(c, []uint32{2})
			case "short-x":
				saved := c.RX.Data
				c.RX.Data = c.RX.Data[:1]
				u, err = NewRandomUpdater(c, []uint32{1})
				c.RX.Data = saved
			case "fill-nil":
				err = s.FillVarSetForRandomUpdate(nil, nil)
			case "fill-term":
				out := []uint32{97}
				err = s.FillVarSetForRandomUpdate([]uint32{0, LARTermStartIndex + 99}, &out)
				if !reflect.DeepEqual(out, []uint32{97, 0}) {
					t.Fatal(out)
				}
			case "outer-term":
				err = s.RandomUpdate([]uint32{LARTermStartIndex + 99})
			case "outer-stopped":
				s.stopped = true
				err = s.RandomUpdate(nil)
				want = ErrExactVectorPending
			case "update-stopped":
				u, err = NewRandomUpdater(c, []uint32{0, 1})
				h.must(err)
				u.stopped = true
				err = u.Update()
				want = ErrExactVectorPending
			case "remove-missing":
				u, err = NewRandomUpdater(c, nil)
				h.must(err)
				err = u.removeValue(h.pair("3", "5"))
				if sourceDebugEnabled {
					want = ErrAssertion
				}
			case "count-wrap":
				u, err = NewRandomUpdater(c, []uint32{0})
				h.must(err)
				for _, bucket := range u.values {
					bucket[0].count = ^uint32(0)
				}
				h.must(u.addValue(&c.RX.Data[0]))
				for _, bucket := range u.values {
					if bucket[0].count != 0 {
						t.Fatal("uint32 increment did not wrap")
					}
				}
				h.must(u.removeValue(&c.RX.Data[0]))
				for _, bucket := range u.values {
					if bucket[0].count != ^uint32(0) {
						t.Fatal("uint32 decrement did not wrap")
					}
				}
				want = nil
			default:
				u, err = NewRandomUpdater(c, []uint32{0, 1})
				h.must(err)
				u.interval = NewRandomUpdateInterval(h.m)
				u.interval.column, err = c.GetColumnIterator(0)
				h.must(err)
				h.must(u.interval.findForColumn(c, 0))
				if op == "pre-contains" {
					h.must(u.interval.SetLowBound(h.pair("1", "0")))
				}
				if op == "pre-feasible" {
					h.must(c.Bounds.Types.Set(0, ColumnLowBound))
					h.must(c.Bounds.Low.Set(0, h.pair("1", "0")))
				}
				if op == "final-equation" {
					h.must(c.RightSides.Data[0].Copy(h.m, h.pair("1", "0")))
				}
				if op == "late-row" {
					c.RA.Columns[0][0].Row = 99
				}
				if op == "final-equation" {
					var output string
					output = lpCaptureFeasibleStdout(t, func() { err = u.shiftVar(0) })
					if sourceDebugEnabled && !strings.Contains(output, "precise x is off") {
						t.Fatal("missing actual residual diagnostic", output)
					}
				} else {
					err = u.shiftVar(0)
				}
				if op == "late-row" {
					want = ErrBounds
				} else {
					want = nil
					if sourceDebugEnabled {
						want = ErrAssertion
					}
				}
			}
			if err != want {
				t.Fatal(err, want)
			}
			if u != nil {
				h.must(u.Close())
			}
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLARRandomUpdaterPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			h := newLPPairHarness(t)
			s, err := NewLARSolver(h.m)
			h.must(err)
			for j := 0; j < 2; j++ {
				_, err = s.AddVar(uint32(j))
				h.must(err)
			}
			u, err := NewRandomUpdater(s.Core, []uint32{0, 1})
			h.must(err)
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
				err = u.Update()
				if u.calls.comparison != 0 {
					t.Fatal("adopted older comparison", err)
				}
			}
			h.must(u.Close())
			h.must(s.Close())
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

func TestLARRandomUpdaterDuplicateRequestsPreserveSourceStop(t *testing.T) {
	h := newLPPairHarness(t)
	s, err := NewLARSolver(h.m)
	h.must(err)
	for j := uint32(0); j < 4; j++ {
		_, err = s.AddVar(j)
		h.must(err)
		if j >= 2 {
			h.must(h.m.SetInt32(&s.Core.RX.Data[j].X, 1))
		}
	}
	u, err := NewRandomUpdater(s.Core, []uint32{0, 0, 1, 2, 3})
	h.must(err)
	oracle := s.Settings.random
	first, second := int64(oracle.Next()%100000), int64(oracle.Next()%100000)
	h.must(u.Update())
	for j, want := range []int64{first, second, 1, 1} {
		h.check(&s.Core.RX.Data[j], big.NewRat(want, 1), big.NewRat(0, 1))
	}
	// The extra request retains a count for zero after both zero-valued columns
	// move. Source stops at four dictionary keys although two model values still
	// coincide; do not repair that behavior by deduplicating value additions.
	if u.distinct != 4 {
		t.Fatal(u.distinct)
	}
	key, err := u.valueKey(h.pair("0", "0"))
	h.must(err)
	if len(u.values[key]) != 1 || u.values[key][0].count != 1 {
		t.Fatal("lost retained duplicate count")
	}
	if s.Settings.RandomNext() != oracle.Next() {
		t.Fatal("source early stop read another RNG value")
	}
	h.must(u.Close())
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

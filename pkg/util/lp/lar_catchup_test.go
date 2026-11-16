package lp

import (
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Distinct Vandermonde columns make every square basis nonsingular. Row
// scalings and rational column scalings vary the actual LU arithmetic.
func lpLARCatchUpFixture(t *testing.T, h *lpPairHarness, rows, cols, world int) (*LARCoreSolver, [][]*big.Rat) {
	t.Helper()
	s, e := NewSettings()
	h.must(e)
	s.SimplexStrategy = StrategyLU
	s.DebugOut, s.MessageOut = nil, nil
	if world%2 == 0 {
		s.DensityThreshold = 0
	} else {
		s.DensityThreshold = 1e100
	}
	c, e := NewLARCoreSolver(h.m, s, nil)
	h.must(e)
	for j := 0; j < cols; j++ {
		lpCoreScopeAddColumn(t, h, c, false)
	}
	a := make([][]*big.Rat, rows)
	for i := 0; i < rows; i++ {
		h.must(c.RA.AddRow())
		c.DA.AddRow()
		a[i] = make([]*big.Rat, cols)
		for j := 0; j < cols; j++ {
			n := int64(1)
			for k := 0; k < i; k++ {
				n *= int64(j + 1)
			}
			n *= int64(2 + i + world%3)
			if i%2 == 1 {
				n = -n
			}
			a[i][j] = big.NewRat(n, int64(1+(world+j)%5))
			h.must(c.RA.Set(uint32(i), uint32(j), &h.pair(a[i][j].RatString(), "0").X))
			value, _ := a[i][j].Float64()
			h.must(c.DA.Set(uint32(i), uint32(j), value))
		}
		c.RBasis = append(c.RBasis, uint32(i))
		c.DBasis = append(c.DBasis, uint32(i))
	}
	h.must(c.RPrimal.InitBasisHeadingAndNonBasicColumns())
	h.must(c.DPrimal.InitBasisHeadingAndNonBasicColumns())
	h.must(InitPairFactorization(&c.RPrimal.Factorization, c.RA, c.RBasis, s))
	h.must(InitFloatFactorization(&c.DPrimal.Factorization, c.DA, c.DBasis, s))
	return c, a
}

func lpCheckLARCatchUpBasis(t *testing.T, basis, nonbasis []uint32, heading []int32, want []uint32) {
	t.Helper()
	actual := append([]uint32(nil), basis...)
	wanted := append([]uint32(nil), want...)
	sort.Slice(actual, func(i, j int) bool { return actual[i] < actual[j] })
	sort.Slice(wanted, func(i, j int) bool { return wanted[i] < wanted[j] })
	if !reflect.DeepEqual(actual, wanted) {
		t.Fatal("independent final membership", basis, want)
	}
	seen := map[uint32]bool{}
	for i, j := range basis {
		if seen[j] || heading[j] != int32(i) {
			t.Fatal("basis topology", basis, heading)
		}
		seen[j] = true
	}
	for i, j := range nonbasis {
		if seen[j] || heading[j] != -int32(i)-1 {
			t.Fatal("nonbasis topology", nonbasis, heading)
		}
		seen[j] = true
	}
	if len(seen) != len(heading) {
		t.Fatal("lost column", seen, heading)
	}
}

func lpCheckLARCatchUpSolve(t *testing.T, h *lpPairHarness, c *LARCoreSolver, a [][]*big.Rat, floating bool) {
	t.Helper()
	basis := c.RBasis
	if floating {
		basis = c.DBasis
	}
	rows := len(basis)
	xs, ys := make([]*big.Rat, rows), make([]*big.Rat, rows)
	for i := 0; i < rows; i++ {
		xs[i] = big.NewRat(int64(3*i-7), int64(i+2))
		ys[i] = big.NewRat(int64(2*i+1), int64(i+3))
	}
	if floating {
		if c.DPrimal.Factorization == nil {
			t.Fatal("missing floating factorization")
		}
		rhs := make([]float64, rows)
		for i := range rhs {
			for j, col := range basis {
				av, _ := a[i][col].Float64()
				xv, _ := xs[j].Float64()
				rhs[i] += float64(av * xv)
			}
		}
		h.must(c.DPrimal.Factorization.SolveBy(rhs))
		for i, v := range rhs {
			want, _ := xs[i].Float64()
			if math.Abs(v-want) > 1e-7*(1+math.Abs(want)) {
				t.Fatal("independent floating solve", basis, rhs, xs)
			}
		}
	} else {
		if c.RPrimal.Factorization == nil || c.RPrimal.Factorization.GetStatus() != LUOK {
			t.Fatal("missing precise factorization")
		}
		rhs := NewIndexedPairVector(h.m, uint32(rows))
		for i := 0; i < rows; i++ {
			x, y := new(big.Rat), new(big.Rat)
			for j, col := range basis {
				x.Add(x, new(big.Rat).Mul(a[i][col], xs[j]))
				y.Add(y, new(big.Rat).Mul(a[i][col], ys[j]))
			}
			h.must(rhs.Data[i].Copy(h.m, h.pair(x.RatString(), y.RatString())))
		}
		h.must(c.RPrimal.Factorization.SolveBy(rhs.Data))
		for i := 0; i < rows; i++ {
			h.check(&rhs.Data[i], xs[i], ys[i])
		}
		h.must(rhs.Close())
	}
}

func TestLARCatchUpIndependentTraceModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	rng := rand.New(rand.NewPCG(81627, 9251))
	for world := 0; world < 160; world++ {
		for _, floating := range []bool{false, true} {
			h := newLPPairHarness(t)
			rows, cols := 1+world%3, 5+world%3
			c, a := lpLARCatchUpFixture(t, h, rows, cols, world)
			// Independently generate the other solver's chronological exchanges.
			basis := make([]uint32, rows)
			non := make([]uint32, cols-rows)
			for i := range basis {
				basis[i] = uint32(i)
			}
			for i := range non {
				non[i] = uint32(rows + i)
			}
			var trace []uint32
			for step := 0; step < 8+world%29; step++ {
				r, n := rng.IntN(rows), rng.IntN(len(non))
				trace = append(trace, non[n], basis[r])
				basis[r], non[n] = non[n], basis[r]
			}
			target := make([]int32, cols)
			for i := range target {
				target[i] = -1
			}
			for i, j := range basis {
				target[j] = int32(i)
			}
			changes := uint32(0)
			for _, j := range basis {
				if j >= uint32(rows) {
					changes++
				}
			}
			counter := uint32(world % 100)
			if world%3 == 0 {
				counter = 199
			}
			var err error
			if floating {
				old := c.DPrimal.Factorization
				old.RefactorCounter = counter
				err = c.CatchUpInFloatLU(trace, target)
				h.must(err)
				lpCheckLARCatchUpBasis(t, c.DBasis, c.DNBasis, c.DHeading, basis)
				if counter+uint32(len(trace)/2) >= 200 {
					if c.DPrimal.Factorization != nil || !old.closed {
						t.Fatal("floating threshold", world)
					}
					h.must(InitFloatFactorization(&c.DPrimal.Factorization, c.DA, c.DBasis, c.settings))
				} else if c.DPrimal.Factorization != old || old.RefactorCounter != counter+changes {
					t.Fatal("compacted float updates", world, old.RefactorCounter, counter, changes)
				}
			} else {
				old := c.RPrimal.Factorization
				old.RefactorCounter = counter
				err = c.CatchUpInPairLU(trace, target)
				h.must(err)
				lpCheckLARCatchUpBasis(t, c.RBasis, c.RNBasis, c.RHeading, basis)
				if counter+uint32(len(trace)/2) >= 200 {
					if c.RPrimal.Factorization == old || !old.closed || c.RPrimal.Factorization.RefactorCounter != 0 {
						t.Fatal("exact threshold", world)
					}
				} else if c.RPrimal.Factorization != old || old.RefactorCounter != counter+changes {
					t.Fatal("compacted exact updates", world, old.RefactorCounter, counter, changes)
				}
			}
			lpCheckLARCatchUpSolve(t, h, c, a, floating)
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARCatchUpReverseIndependentModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	rng := rand.New(rand.NewPCG(91712, 1513))
	for world := 0; world < 100; world++ {
		h := newLPPairHarness(t)
		rows, cols := 1+world%3, 5+world%3
		c, a := lpLARCatchUpFixture(t, h, rows, cols, world)
		initial := append([]uint32(nil), c.RBasis...)
		var trace []uint32
		for step := 0; step < 2+world%19; step++ {
			enter, leave := c.RNBasis[rng.IntN(len(c.RNBasis))], c.RBasis[rng.IntN(rows)]
			trace = append(trace, enter, leave)
			h.must(c.RPrimal.ChangeBasisUnconditionally(enter, leave))
		}
		old := c.RPrimal.Factorization
		h.must(c.CatchUpInPairLUInReverse(trace))
		if !reflect.DeepEqual(c.RBasis, initial) || c.RPrimal.Factorization == old || !old.closed {
			t.Fatal("reverse original basis", world, c.RBasis, initial)
		}
		lpCheckLARCatchUpBasis(t, c.RBasis, c.RNBasis, c.RHeading, initial)
		lpCheckLARCatchUpSolve(t, h, c, a, false)
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARCatchUpDegenerationContinuesBasis(t *testing.T) {
	for _, floating := range []bool{false, true} {
		h := newLPPairHarness(t)
		c, a := lpLARCatchUpFixture(t, h, 2, 4, 1)
		// Entering col2 duplicates col1: the first basis is singular. The
		// subsequent col3 exchange restores independence; basis changes continue
		// after LU degeneration even though incremental replacement is skipped.
		for i := 0; i < 2; i++ {
			a[i][2] = new(big.Rat).Set(a[i][1])
			// Update the existing source cells, preserving matrix topology.
			for k := range c.RA.Rows[i] {
				if c.RA.Rows[i][k].Column == 2 {
					h.must(h.m.Copy(&c.RA.Rows[i][k].Value, &h.pair(a[i][2].RatString(), "0").X))
				}
			}
			for k := range c.DA.Rows[i] {
				if c.DA.Rows[i][k].Column == 2 {
					v, _ := a[i][2].Float64()
					c.DA.Rows[i][k].Value = v
				}
			}
		}
		trace, target := []uint32{2, 0, 3, 1}, []int32{-1, -2, 0, 1}
		if floating {
			h.must(c.CatchUpInFloatLU(trace, target))
			if c.DPrimal.Factorization != nil {
				t.Fatal("source leaves float LU nil")
			}
			lpCheckLARCatchUpBasis(t, c.DBasis, c.DNBasis, c.DHeading, []uint32{2, 3})
		} else {
			h.must(c.CatchUpInPairLU(trace, target))
			lpCheckLARCatchUpBasis(t, c.RBasis, c.RNBasis, c.RHeading, []uint32{2, 3})
			lpCheckLARCatchUpSolve(t, h, c, a, false)
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARCatchUpNilThresholdAndUnsignedCounter(t *testing.T) {
	for _, floating := range []bool{false, true} {
		for _, phase := range []string{"nil", "nil-empty", "boundary-198", "boundary-199", "counter-wrap"} {
			t.Run(fmt.Sprintf("%t/%s", floating, phase), func(t *testing.T) {
				h := newLPPairHarness(t)
				c, a := lpLARCatchUpFixture(t, h, 1, 3, 2)
				trace := []uint32{1, 0}
				target := []int32{-1, 0, -2}
				counter := uint32(198)
				if phase == "boundary-199" {
					counter = 199
				}
				if phase == "counter-wrap" {
					counter = ^uint32(0)
				}
				missing := phase == "nil" || phase == "nil-empty"
				if phase == "nil" || phase == "boundary-199" {
					target = nil
				}
				if phase == "nil-empty" {
					trace = nil
				}
				if floating {
					old := c.DPrimal.Factorization
					if missing {
						old.Close()
						c.DPrimal.Factorization = nil
					} else {
						old.RefactorCounter = counter
					}
					h.must(c.CatchUpInFloatLU(trace, target))
					discarded := missing || counter+uint32(len(trace)/2) >= 200
					if (c.DPrimal.Factorization == nil) != discarded {
						t.Fatal("float refactor boundary", phase)
					}
					if !discarded {
						if c.DPrimal.Factorization != old || old.RefactorCounter != counter+1 {
							t.Fatal("unsigned float increment", phase, old.RefactorCounter)
						}
						lpCheckLARCatchUpSolve(t, h, c, a, true)
					}
				} else {
					old := c.RPrimal.Factorization
					if missing {
						h.must(old.Close())
						c.RPrimal.Factorization = nil
					} else {
						old.RefactorCounter = counter
					}
					h.must(c.CatchUpInPairLU(trace, target))
					if c.RPrimal.Factorization == nil {
						t.Fatal("precise rebuild missing")
					}
					if missing || counter+uint32(len(trace)/2) >= 200 {
						if c.RPrimal.Factorization == old || !old.closed {
							t.Fatal("precise replacement", phase)
						}
					} else if c.RPrimal.Factorization != old || old.RefactorCounter != counter+1 {
						t.Fatal("unsigned precise increment", phase, old.RefactorCounter)
					}
					lpCheckLARCatchUpSolve(t, h, c, a, false)
				}
				basis := c.RBasis
				if floating {
					basis = c.DBasis
				}
				want := uint32(1)
				if phase == "nil-empty" {
					want = 0
				}
				if basis[0] != want {
					t.Fatal("chronological basis", phase, basis)
				}
				h.must(c.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
	}
}

func TestLARCatchUpReachedErrorPrefixes(t *testing.T) {
	for _, floating := range []bool{false, true} {
		for _, phase := range []string{"stopped", "nil-solver", "bad-lu", "stopped-lu", "target-short", "odd-incremental", "odd-refactor", "bad-enter-refactor", "bad-leave-refactor", "detached-nbasis-incremental", "bad-row-incremental", "short-basis-incremental"} {
			t.Run(fmt.Sprintf("%t/%s", floating, phase), func(t *testing.T) {
				h := newLPPairHarness(t)
				c, _ := lpLARCatchUpFixture(t, h, 1, 3, 1)
				savedPair, savedFloat := c.RPrimal, c.DPrimal
				trace, target := []uint32{1, 0}, []int32{-1, 0, -2}
				want := ErrBounds
				prefix := uint32(0)
				switch phase {
				case "stopped":
					c.stopped = true
					want = ErrExactVectorPending
				case "nil-solver":
					if floating {
						c.DPrimal = nil
					} else {
						c.RPrimal = nil
					}
				case "bad-lu":
					c.RPrimal.Factorization.status = LUDegenerated
					c.DPrimal.Factorization.status = LUDegenerated
					if sourceDebugEnabled {
						want = ErrAssertion
					} else {
						want = nil
						prefix = 1
					}
				case "stopped-lu":
					c.RPrimal.Factorization.stopped = true
					c.DPrimal.Factorization.stopped = true
					want = ErrLUStopped
				case "target-short":
					trace = []uint32{1, 0, 2, 1}
					target = target[:2]
					prefix = 1
				case "odd-incremental":
					trace = []uint32{1, 0, 2}
					prefix = 1
				case "odd-refactor":
					trace = []uint32{1, 0, 2}
					prefix = 1
					c.RPrimal.Factorization.RefactorCounter = 199
					c.DPrimal.Factorization.RefactorCounter = 199
				case "bad-enter-refactor":
					trace = []uint32{7, 0}
					c.RPrimal.Factorization.RefactorCounter = 199
					c.DPrimal.Factorization.RefactorCounter = 199
				case "bad-leave-refactor":
					trace = []uint32{1, 7}
					c.RPrimal.Factorization.RefactorCounter = 199
					c.DPrimal.Factorization.RefactorCounter = 199
				case "detached-nbasis-incremental":
					want = nil
					prefix = 1
					c.RNBasis = nil
					c.DNBasis = nil
				case "bad-row-incremental":
					c.RHeading[0] = 8
					c.DHeading[0] = 8
				case "short-basis-incremental":
					c.RBasis = nil
					c.DBasis = nil
				}
				var err error
				if floating {
					err = c.CatchUpInFloatLU(trace, target)
				} else {
					err = c.CatchUpInPairLU(trace, target)
				}
				if err != want {
					t.Fatal("reached failure", err, want)
				}
				basis := c.RBasis
				if floating {
					basis = c.DBasis
				}
				if phase != "short-basis-incremental" && phase != "bad-row-incremental" && basis[0] != prefix {
					t.Fatal("completed basis prefix", phase, basis, prefix)
				}
				if phase == "bad-row-incremental" {
					if floating {
						if c.DPrimal.Factorization.RefactorCounter != 1 {
							t.Fatal("replace counter")
						}
					} else if c.RPrimal.Factorization.RefactorCounter != 1 {
						t.Fatal("replace counter")
					}
				}
				if phase == "short-basis-incremental" {
					heading, counter := c.RHeading, c.RPrimal.Factorization.RefactorCounter
					if floating {
						heading, counter = c.DHeading, c.DPrimal.Factorization.RefactorCounter
					}
					if heading[1] != 0 || heading[0] != 0 || counter != 1 {
						t.Fatal("LU replacement precedes partial heading publication", heading, counter)
					}
				}
				if err != nil {
					if _, e := c.FindSolutionSignatureWithDoubles(nil); e != ErrExactVectorPending {
						t.Fatal("stopped parent", e)
					}
				}
				c.RPrimal, c.DPrimal = savedPair, savedFloat
				h.must(c.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
	}
}

func TestLARCatchUpReverseReachedPrefixes(t *testing.T) {
	for _, phase := range []string{"stopped", "nil-solver", "stopped-child", "odd", "bad-leave", "empty"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			c, _ := lpLARCatchUpFixture(t, h, 1, 3, 1)
			h.must(c.RPrimal.ChangeBasisUnconditionally(1, 0))
			saved := c.RPrimal
			old := saved.Factorization
			trace := []uint32{1, 0}
			want := ErrBounds
			switch phase {
			case "stopped":
				c.stopped = true
				want = ErrExactVectorPending
			case "nil-solver":
				c.RPrimal = nil
			case "stopped-child":
				c.RPrimal.stopped = true
				want = ErrExactVectorPending
			case "odd":
				trace = []uint32{2, 1, 0}
			case "bad-leave":
				trace = []uint32{7, 0}
			case "empty":
				trace = nil
				want = nil
			}
			err := c.CatchUpInPairLUInReverse(trace)
			if err != want {
				t.Fatal(err, want)
			}
			if phase == "odd" && c.RBasis[0] != 0 {
				t.Fatal("reverse completed prefix", c.RBasis)
			}
			if phase == "empty" && (c.RPrimal.Factorization == old || !old.closed) {
				t.Fatal("empty reverse must rebuild")
			}
			c.RPrimal = saved
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func lpSetLARCatchUpExactCell(t *testing.T, h *lpPairHarness, c *LARCoreSolver, row, col uint32, value string) {
	t.Helper()
	for k := range c.RA.Rows[row] {
		if c.RA.Rows[row][k].Column == col {
			h.must(h.m.Copy(&c.RA.Rows[row][k].Value, &h.pair(value, "0").X))
			return
		}
	}
	t.Fatal("missing coefficient")
}

func TestLARCatchUpPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		for _, path := range []string{"incremental", "rebuild", "reverse"} {
			success, failures := false, 0
			for ordinal := int64(0); ordinal < 1500; ordinal++ {
				done := false
				t.Run(fmt.Sprintf("%t/%s/%d", direct, path, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					c, _ := lpLARCatchUpFixture(t, h, 2, 4, 1)
					huge := strings.Repeat("1234567890", 20)
					lpSetLARCatchUpExactCell(t, h, c, 0, 0, huge+"7/13")
					lpSetLARCatchUpExactCell(t, h, c, 1, 2, huge+"3/17")
					h.must(InitPairFactorization(&c.RPrimal.Factorization, c.RA, c.RBasis, c.settings))
					trace, target := []uint32{2, 0}, []int32{-1, 1, 0, -2}
					if path == "rebuild" {
						c.RPrimal.Factorization.RefactorCounter = 199
					}
					if path == "reverse" {
						h.must(c.RPrimal.ChangeBasisUnconditionally(2, 0))
					}
					h.budget.FailAfter(ordinal)
					var err error
					if path == "reverse" {
						err = c.CatchUpInPairLUInReverse(trace)
					} else {
						err = c.CatchUpInPairLU(trace, target)
					}
					h.budget.FailAfter(-1)
					if err == nil {
						success, done = true, true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						want := uint32(0)
						if path == "rebuild" {
							want = 2
						}
						if c.RBasis[0] != want {
							t.Fatal("numerical failure basis prefix", path, c.RBasis, want)
						}
						if c.CatchUpInPairLU(nil, nil) != ErrExactVectorPending {
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
			if !success || direct && failures == 0 {
				t.Fatal("physical campaign", direct, path, success, failures)
			}
		}
	}
}

func TestLARCatchUpQueueReachedExtents(t *testing.T) {
	for _, phase := range []string{"enter-target", "enter-heading", "leave-target", "leave-heading", "leave-queue", "enter-queue", "short-circuit"} {
		t.Run(phase, func(t *testing.T) {
			var queues larCatchUpQueues
			enter, leave := uint32(0), uint32(1)
			var target, heading []int32
			switch phase {
			case "enter-heading":
				target = []int32{0, 0}
			case "leave-target":
				target = []int32{-1}
			case "leave-heading":
				target = []int32{-1, -1}
			case "leave-queue":
				target = []int32{0, 0}
				heading = []int32{-1, 0}
				queues.leave = []uint32{2}
			case "enter-queue":
				target = []int32{-1, -1}
				heading = []int32{0, 0}
				queues.enter = []uint32{2}
			case "short-circuit":
				target = []int32{-1, 0}
			}
			_, _, use, err := queues.selectColumns(enter, leave, target, heading)
			if phase == "short-circuit" {
				if err != nil || use {
					t.Fatal("unreached heading access", err, use)
				}
			} else if err != ErrBounds {
				t.Fatal("reached extent", err)
			}
		})
	}
}

func TestLARCatchUpPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			h := newLPPairHarness(t)
			c, _ := lpLARCatchUpFixture(t, h, 2, 4, 1)
			huge := strings.Repeat("1234567890", 20) + "7/13"
			lpSetLARCatchUpExactCell(t, h, c, 0, 2, huge)
			v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
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
				err = c.CatchUpInPairLU([]uint32{2, 0}, []int32{-1, 1, 0, -2})
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

func TestLARCatchUpPairPendingPrimal(t *testing.T) {
	h := newLPPairHarness(t)
	c, _ := lpLARCatchUpFixture(t, h, 1, 3, 1)
	c.RPrimal.stopped = true
	if err := c.CatchUpInPairLU([]uint32{1, 0}, []int32{-1, 0, -2}); err != ErrExactVectorPending || c.catchUpWork != nil || c.RBasis[0] != 0 {
		t.Fatal("pending child reached before workspace", err)
	}
	h.must(c.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARCatchUpUnmatchedQueuesRetainBasis(t *testing.T) {
	for _, floating := range []bool{false, true} {
		for _, trace := range [][]uint32{{1, 2}, {2, 0}} {
			h := newLPPairHarness(t)
			c, a := lpLARCatchUpFixture(t, h, 1, 3, 1)
			target := []int32{-1, 0, -2}
			// These traces belong to a solver whose old basis differs from ours.
			// A single useful endpoint is delayed and then discarded at scope exit.
			if floating {
				old := c.DPrimal.Factorization
				h.must(c.CatchUpInFloatLU(trace, target))
				if c.DBasis[0] != 0 || c.DPrimal.Factorization != old || old.RefactorCounter != 0 {
					t.Fatal("unmatched float endpoint", trace, c.DBasis)
				}
			} else {
				old := c.RPrimal.Factorization
				h.must(c.CatchUpInPairLU(trace, target))
				if c.RBasis[0] != 0 || c.RPrimal.Factorization != old || old.RefactorCounter != 0 || c.catchUpWork != nil {
					t.Fatal("unmatched pair endpoint", trace, c.RBasis)
				}
			}
			lpCheckLARCatchUpSolve(t, h, c, a, floating)
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARCatchUpFloatingSearchExactIntegration(t *testing.T) {
	for world := 0; world < 80; world++ {
		h := newLPPairHarness(t)
		lower := 1 + world%17
		strict := world%2 == 0
		c := lpLARFloatingSignatureFixture(t, h, lower, lower+4, strict)
		signature := NewLARSolutionSignature()
		trace, err := c.FindSolutionSignatureWithDoubles(signature)
		h.must(err)
		if !reflect.DeepEqual(trace, []uint32{1, 0}) {
			t.Fatal("actual presolve trace", world, trace)
		}
		h.must(c.CatchUpInPairLU(trace, c.DHeading))
		h.must(c.PreparePairSolverXWithSignature(signature, c.RPrimal))
		epsilon := new(big.Rat)
		if strict {
			epsilon.SetInt64(1)
		}
		h.check(&c.RX.Data[0], big.NewRat(int64(lower), 1), epsilon)
		h.check(&c.RX.Data[1], big.NewRat(int64(-lower), 2), new(big.Rat).Quo(new(big.Rat).Neg(epsilon), big.NewRat(2, 1)))
		h.check(&c.RX.Data[2], new(big.Rat), new(big.Rat))
		h.must(c.RPrimal.InfSet.Resize(3))
		c.RPrimal.StartTracingBasisChanges()
		h.must(c.RPrimal.FindFeasibleSolution())
		c.RPrimal.StopTracingBasisChanges()
		if c.RPrimal.Status != StatusFeasible || c.RPrimal.TotalIterations != 0 || len(c.RPrimal.Trace) != 0 {
			t.Fatal("exact signature witness", world, c.RPrimal.Status, c.RPrimal.Trace)
		}
		old := c.DPrimal.Factorization
		h.must(c.CatchUpInFloatLU(c.RPrimal.Trace, c.RHeading))
		if c.DPrimal.Factorization != old || c.RBasis[0] != 1 || c.DBasis[0] != 1 {
			t.Fatal("coupled exact/floating basis")
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

package lp

// Source: src/util/lp/lar_core_solver.h:276-352;
// src/util/lp/lp_core_solver_base.h:685-688.

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

func lpLARSignatureTableauFixture(t *testing.T, h *lpPairHarness) *LARCoreSolver {
	t.Helper()
	c := lpNewCoreScopeFixture(t, h)
	c.settings.SimplexStrategy = StrategyTableauRows
	c.RPrimal.Names = lpFeasibleNames{}
	h.must(c.PrefixR())
	h.must(c.RPrimal.InfSet.Resize(3))
	h.must(c.RPrimal.Factorization.Close())
	c.RPrimal.Factorization = nil
	return c
}

func TestLARTableauSignatureUpdateIndependentModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	type profile struct {
		position      NonBasicColumnValuePosition
		kind          ColumnType
		side, initial int
		changed, add  bool
	}
	profiles := []profile{
		{PositionAtLowBound, ColumnBoxed, 1, 0, true, false},
		{PositionAtLowBound, ColumnLowBound, 1, 1, false, false},
		{PositionAtUpperBound, ColumnBoxed, 2, 0, true, false},
		{PositionAtUpperBound, ColumnUpperBound, 2, 2, false, false},
		{PositionAtFixed, ColumnFixed, 2, 0, true, false},
		{PositionAtFixed, ColumnFixed, 2, 2, false, false},
		{PositionFreeOfBounds, ColumnFree, 0, 0, false, false},
		{PositionNotAtBound, ColumnFree, 0, 0, false, false},
		{PositionNotAtBound, ColumnLowBound, 1, 0, true, false},
		{PositionNotAtBound, ColumnLowBound, 1, 1, true, false},
		{PositionNotAtBound, ColumnUpperBound, 2, 0, true, false},
		{PositionNotAtBound, ColumnUpperBound, 2, 2, true, false},
		{PositionNotAtBound, ColumnFixed, 1, 0, true, false},
		{PositionNotAtBound, ColumnBoxed, 1, 0, true, false},
		{PositionNotAtBound, ColumnBoxed, 2, 3, true, true},
	}
	for world := 0; world < 450; world++ {
		p := profiles[world%len(profiles)]
		h := newLPPairHarness(t)
		c := lpLARSignatureTableauFixture(t, h)
		c.Bounds.Types.values[1] = p.kind
		lo, ly := big.NewRat(int64(world%7-4), 7), big.NewRat(1, 17)
		up, uy := big.NewRat(int64(world%11+4), 5), big.NewRat(-1, 19)
		if p.kind == ColumnFixed {
			up.Set(lo)
			uy.Set(ly)
		}
		x, y := new(big.Rat), new(big.Rat)
		switch p.initial {
		case 1:
			x.Set(lo)
			y.Set(ly)
		case 2:
			x.Set(up)
			y.Set(uy)
		case 3:
			x.Add(up, big.NewRat(1, 1))
			y.Set(uy)
		}
		h.must(c.Bounds.Low.Set(1, h.pair(lo.RatString(), ly.RatString())))
		h.must(c.Bounds.Upper.Set(1, h.pair(up.RatString(), uy.RatString())))
		h.must(c.RX.Data[1].Copy(h.m, h.pair(x.RatString(), y.RatString())))
		h.must(c.RPrimal.InfSet.Insert(1))
		delta := h.pair("17", "23")
		changed, err := c.UpdateXJAndGetDelta(1, p.position, delta)
		if p.add && sourceDebugEnabled {
			if err != ErrAssertion || !changed {
				t.Fatal("boxed source assertion", err, changed)
			}
		} else {
			h.must(err)
			if changed != p.changed {
				t.Fatal(world, changed, p.changed)
			}
		}
		wantX, wantY := new(big.Rat).Set(x), new(big.Rat).Set(y)
		dx, dy := big.NewRat(17, 1), big.NewRat(23, 1)
		if p.changed {
			bound, boundY := lo, ly
			if p.side == 2 {
				bound, boundY = up, uy
			}
			dx.Sub(bound, x)
			dy.Sub(boundY, y)
			wantX.Set(bound)
			wantY.Set(boundY)
			if p.add {
				wantX.Add(x, bound)
				wantY.Add(y, boundY)
			}
		}
		h.check(&c.RX.Data[1], wantX, wantY)
		h.check(delta, dx, dy)
		if c.RPrimal.InfSet.Contains(1) == p.changed {
			t.Fatal("erase only after actual update", world)
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARTableauSignaturePreparationIndependentSystems(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 240; world++ {
		h := newLPPairHarness(t)
		c := lpLARSignatureTableauFixture(t, h)
		// Independent homogeneous system x0+2*x1+3*x2=0. Both rational and
		// epsilon coordinates must still satisfy it after both nonbasic updates.
		x1, y1 := big.NewRat(int64(world%13-6), 7), big.NewRat(int64(world%5-2), 11)
		x2, y2 := big.NewRat(int64(world%17-8), 13), big.NewRat(int64(world%7-3), 17)
		x0 := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), x1), new(big.Rat).Mul(big.NewRat(3, 1), x2)))
		y0 := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), y1), new(big.Rat).Mul(big.NewRat(3, 1), y2)))
		for j, v := range [][2]*big.Rat{{x0, y0}, {x1, y1}, {x2, y2}} {
			h.must(c.RX.Data[j].Copy(h.m, h.pair(v[0].RatString(), v[1].RatString())))
		}
		lo, ly := big.NewRat(int64(world%9-4), 5), big.NewRat(1, 23)
		up, uy := big.NewRat(int64(world%11+1), 3), big.NewRat(-1, 29)
		c.Bounds.Types.values[1], c.Bounds.Types.values[2] = ColumnLowBound, ColumnUpperBound
		h.must(c.Bounds.Low.Set(1, h.pair(lo.RatString(), ly.RatString())))
		h.must(c.Bounds.Upper.Set(2, h.pair(up.RatString(), uy.RatString())))
		// A bounded basic variable also exercises real InfSet insert/erase.
		if world%2 == 0 {
			c.Bounds.Types.values[0] = ColumnLowBound
			h.must(c.Bounds.Low.Set(0, h.pair("0", "0")))
		}
		for j := uint32(0); j < 3; j++ {
			h.must(c.RPrimal.UpdateColumnInInfSet(j))
		}
		signature := lpSignature(map[uint32]NonBasicColumnValuePosition{0: NonBasicColumnValuePosition(99), 1: PositionAtLowBound, 2: PositionAtUpperBound})
		h.must(c.PrepareSolverXWithSignatureTableau(signature))
		wantX := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), lo), new(big.Rat).Mul(big.NewRat(3, 1), up)))
		wantY := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), ly), new(big.Rat).Mul(big.NewRat(3, 1), uy)))
		h.check(&c.RX.Data[0], wantX, wantY)
		h.check(&c.RX.Data[1], lo, ly)
		h.check(&c.RX.Data[2], up, uy)
		basicInfeasible := world%2 == 0 && (wantX.Sign() < 0 || wantX.Sign() == 0 && wantY.Sign() < 0)
		if c.RPrimal.InfSet.Contains(0) != basicInfeasible || c.RPrimal.InfSet.Contains(1) || c.RPrimal.InfSet.Contains(2) {
			t.Fatal("independent feasibility membership", world)
		}
		if c.signatureDelta != nil || c.signatureExpression != nil || c.signatureScalars != nil {
			t.Fatal("source lexical lifetime")
		}
		if p0, _ := signature.Get2(0); signature.Len() != 3 || p0 != 99 {
			t.Fatal("signature mutated")
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARTableauSignatureBoxedAdditiveSourcePrefix(t *testing.T) {
	h := newLPPairHarness(t)
	c := lpLARSignatureTableauFixture(t, h)
	c.Bounds.Types.values[1] = ColumnBoxed
	h.must(c.Bounds.Low.Set(1, h.pair("0", "0")))
	h.must(c.Bounds.Upper.Set(1, h.pair("3", "0")))
	h.must(c.RX.Data[0].Copy(h.m, h.pair("-10", "0")))
	h.must(c.RX.Data[1].Copy(h.m, h.pair("5", "0")))
	h.must(c.RPrimal.InfSet.Insert(1))
	err := c.PrepareSolverXWithSignatureTableau(lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionNotAtBound}))
	wantBasic := int64(-6)
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
		wantBasic = -10
	} else {
		h.must(err)
	}
	h.check(&c.RX.Data[1], big.NewRat(8, 1), new(big.Rat))
	h.check(&c.RX.Data[0], big.NewRat(wantBasic, 1), new(big.Rat))
	if c.RPrimal.InfSet.Contains(1) {
		t.Fatal("erasure precedes feasibility assertion")
	}
	h.must(c.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARTableauSignaturePhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		for _, path := range []string{"copy", "boxed-add", "basic-product"} {
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
					c := lpLARSignatureTableauFixture(t, h)
					huge := strings.Repeat("1234567890", 20) + "7/13"
					signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtLowBound})
					c.Bounds.Types.values[1] = ColumnLowBound
					h.must(c.Bounds.Low.Set(1, h.pair(huge, "1/17")))
					if path == "boxed-add" {
						c.Bounds.Types.values[1] = ColumnBoxed
						h.must(c.Bounds.Low.Set(1, h.pair("0", "0")))
						h.must(c.Bounds.Upper.Set(1, h.pair("-"+huge, "0")))
						// Starting at zero makes the source additive result equal the upper.
						// Use a lower below upper so real debug feasibility can finish.
						h.must(c.Bounds.Low.Set(1, h.pair(new(big.Rat).Sub(mustLPRat(t, "-"+huge), big.NewRat(2, 1)).RatString(), "0")))
						h.must(c.RX.Data[1].Copy(h.m, h.pair("0", "0")))
						signature.Set(1, PositionNotAtBound)
					}
					if path == "basic-product" {
						lpSetLARCatchUpExactCell(t, h, c, 0, 1, huge)
						h.must(c.Bounds.Low.Set(1, h.pair("3/7", "1/17")))
					}
					for j := uint32(0); j < 3; j++ {
						h.must(c.RPrimal.UpdateColumnInInfSet(j))
					}
					h.budget.FailAfter(ordinal)
					err := c.PrepareSolverXWithSignatureTableau(signature)
					h.budget.FailAfter(-1)
					if err == nil {
						success, done = true, true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if c.PrepareSolverXWithSignatureTableau(nil) != ErrExactVectorPending {
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

func TestLARTableauSignatureUpdateReachedPrefixes(t *testing.T) {
	for _, phase := range []string{"stopped", "nil-primal", "primal-stopped", "nil-delta", "short-x", "short-low", "short-upper", "short-boxed-upper", "short-types", "invalid-kind", "invalid-position"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpLARSignatureTableauFixture(t, h)
			primal, x, low, upper, types := c.RPrimal, c.RX.Data, c.Bounds.Low.values, c.Bounds.Upper.values, c.Bounds.Types.values
			delta := h.pair("17", "23")
			arg := delta
			j, pos := uint32(1), PositionAtLowBound
			want := error(ErrBounds)
			h.must(c.RPrimal.InfSet.Insert(1))
			switch phase {
			case "stopped":
				c.stopped = true
				want = ErrExactVectorPending
			case "nil-primal":
				c.RPrimal = nil
			case "primal-stopped":
				c.RPrimal.stopped = true
				want = ErrExactVectorPending
			case "nil-delta":
				arg = nil
			case "short-x":
				c.RX.Data = c.RX.Data[:1]
			case "short-low":
				c.Bounds.Low.values = c.Bounds.Low.values[:1]
			case "short-upper":
				c.Bounds.Upper.values = c.Bounds.Upper.values[:1]
				pos = PositionAtUpperBound
			case "short-boxed-upper":
				c.Bounds.Types.values[1] = ColumnBoxed
				c.Bounds.Upper.values = c.Bounds.Upper.values[:1]
				pos = PositionNotAtBound
			case "short-types":
				c.Bounds.Types.values = c.Bounds.Types.values[:1]
				pos = PositionNotAtBound
			case "invalid-kind":
				c.Bounds.Types.values[1] = ColumnType(99)
				pos = PositionNotAtBound
			case "invalid-position":
				pos = NonBasicColumnValuePosition(99)
			}
			invalid := phase == "invalid-kind" || phase == "invalid-position"
			if invalid {
				if sourceDebugEnabled {
					want = ErrAssertion
				} else {
					want = nil
				}
			}
			changed, err := c.UpdateXJAndGetDelta(j, pos, arg)
			if err != want || changed != (invalid && !sourceDebugEnabled) {
				t.Fatal("source prefix", changed, err, want)
			}
			if primal.InfSet.Contains(1) == (invalid && !sourceDebugEnabled) {
				t.Fatal("erase prefix")
			}
			h.check(delta, big.NewRat(17, 1), big.NewRat(23, 1))
			c.RPrimal = primal
			c.RX.Data = x
			c.Bounds.Low.values = low
			c.Bounds.Upper.values = upper
			c.Bounds.Types.values = types
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLARTableauSignaturePreparationReachedPrefixes(t *testing.T) {
	for _, phase := range []string{"stopped", "nil-primal", "primal-stopped", "short-heading", "late-heading", "short-columns", "short-basis", "bad-basic-value", "bad-link", "residual", "initial-membership"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpLARSignatureTableauFixture(t, h)
			primal, heading, basis, columns := c.RPrimal, c.RHeading, c.RBasis, c.RA.Columns
			link := c.RA.Columns[1][0]
			h.must(c.Bounds.Low.Set(1, h.pair("2", "0")))
			signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtLowBound})
			want := error(ErrBounds)
			published := false
			switch phase {
			case "stopped":
				c.stopped = true
				want = ErrExactVectorPending
			case "nil-primal":
				c.RPrimal = nil
			case "primal-stopped":
				c.RPrimal.stopped = true
				want = ErrExactVectorPending
			case "short-heading":
				c.RHeading = c.RHeading[:1]
			case "late-heading":
				// libstdc++ order visits 99 before 1 (AUDIT.md
				// util_lp_03-F1), so the bad heading index stops the walk
				// before column 1 is published.
				signature.Set(99, PositionAtLowBound)
				if got := fmt.Sprint(signature.Keys()); got != "[99 1]" {
					t.Fatal("signature order", got)
				}
			case "short-columns":
				c.RA.Columns = c.RA.Columns[:1]
				published = true
			case "short-basis":
				c.RBasis = nil
				published = true
			case "bad-basic-value":
				c.RBasis[0] = 3
				published = true
			case "bad-link":
				c.RA.Columns[1][0].Offset = 999
				published = true
			case "residual":
				h.must(c.RightSides.Data[0].Copy(h.m, h.pair("1", "0")))
				published = true
				if sourceDebugEnabled {
					want = ErrAssertion
				} else {
					want = nil
				}
			case "initial-membership":
				h.must(primal.InfSet.Insert(1))
				if sourceDebugEnabled {
					want = ErrAssertion
				} else {
					want = nil
					published = true
				}
			}
			var err error
			output := lpCaptureFeasibleStdout(t, func() { err = c.PrepareSolverXWithSignatureTableau(signature) })
			if sourceDebugEnabled && phase == "initial-membership" && !strings.HasSuffix(output, "belongs_to_set = 1\nfeas\n") {
				t.Fatal("real membership diagnostic", output)
			}
			if sourceDebugEnabled && phase == "residual" && !strings.Contains(output, "left side = (0, 0)") {
				t.Fatal("real residual diagnostic", output)
			}
			if err != want {
				t.Fatal("source stage", err, want)
			}
			wantX := int64(0)
			if published {
				wantX = 2
			}
			h.check(&c.RX.Data[1], big.NewRat(wantX, 1), new(big.Rat))
			c.RPrimal = primal
			c.RHeading = heading
			c.RBasis = basis
			c.RBasis[0] = 0
			c.RA.Columns = columns
			c.RA.Columns[1][0] = link
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLARTableauSignatureRemovePendingCore(t *testing.T) {
	h := newLPPairHarness(t)
	c := lpLARSignatureTableauFixture(t, h)
	h.must(c.RPrimal.InfSet.Insert(1))
	c.RPrimal.PairCoreSolverBase.stopped = true
	if err := c.RPrimal.RemoveColumnFromInfSet(1); err != ErrExactVectorPending || !c.RPrimal.InfSet.Contains(1) {
		t.Fatal("pending core", err)
	}
	h.must(c.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARTableauSignaturePreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpLARSignatureTableauFixture(t, h)
			c.Bounds.Types.values[1] = ColumnBoxed
			h.must(c.Bounds.Low.Set(1, h.pair("0", "0")))
			h.must(c.Bounds.Upper.Set(1, h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")))
			h.must(c.RX.Data[1].Copy(h.m, h.pair("1", "0")))
			h.must(c.RX.Data[0].Copy(h.m, h.pair("-2", "0")))
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
				err = c.PrepareSolverXWithSignatureTableau(lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionNotAtBound}))
				if err == nil || c.calls.comparison != 0 {
					t.Fatal("adopted older comparison", err)
				}
			}
			h.must(c.Close())
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

func TestLARTableauSignaturePreparationMultipleRows(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 120; world++ {
		h := newLPPairHarness(t)
		rows, cols := 1+world%3, 6
		c, _ := lpLARTableauCatchUpFixture(t, h, rows, cols, world)
		h.must(c.RPrimal.InfSet.Resize(uint32(cols)))
		// The input is an actual canonical tableau. Independently evaluate each
		// row for the complete old and target nonbasic vectors in both coordinates.
		oldX, oldY, wantX, wantY := make([]*big.Rat, cols), make([]*big.Rat, cols), make([]*big.Rat, cols), make([]*big.Rat, cols)
		signature := lpSignature(map[uint32]NonBasicColumnValuePosition{0: NonBasicColumnValuePosition(99)})
		for j := 0; j < cols; j++ {
			oldX[j] = new(big.Rat)
			oldY[j] = new(big.Rat)
			wantX[j] = new(big.Rat)
			wantY[j] = new(big.Rat)
		}
		for j := rows; j < cols; j++ {
			oldX[j].SetFrac64(int64(world+j-9), 7)
			oldY[j].SetFrac64(int64(j-3), 11)
			wantX[j].SetFrac64(int64(world%13+j-6), 5)
			wantY[j].SetFrac64(int64(2*j-7), 17)
			side := world + j
			if side%3 == 0 {
				wantX[j].Set(oldX[j])
				wantY[j].Set(oldY[j])
			}
			if side%2 == 0 {
				c.Bounds.Types.values[j] = ColumnLowBound
				h.must(c.Bounds.Low.Set(uint32(j), h.pair(wantX[j].RatString(), wantY[j].RatString())))
				signature.Set(uint32(j), PositionAtLowBound)
			} else {
				c.Bounds.Types.values[j] = ColumnUpperBound
				h.must(c.Bounds.Upper.Set(uint32(j), h.pair(wantX[j].RatString(), wantY[j].RatString())))
				signature.Set(uint32(j), PositionAtUpperBound)
			}
			if side%4 < 2 {
				signature.Set(uint32(j), PositionNotAtBound)
			}
		}
		for row := 0; row < rows; row++ {
			for k := range c.RA.Rows[row] {
				cell := &c.RA.Rows[row][k]
				j := int(cell.Column)
				if j < rows {
					continue
				}
				text, err := h.m.String(&cell.Value)
				h.must(err)
				a := mustLPRat(t, text)
				oldX[row].Sub(oldX[row], new(big.Rat).Mul(a, oldX[j]))
				oldY[row].Sub(oldY[row], new(big.Rat).Mul(a, oldY[j]))
				wantX[row].Sub(wantX[row], new(big.Rat).Mul(a, wantX[j]))
				wantY[row].Sub(wantY[row], new(big.Rat).Mul(a, wantY[j]))
			}
		}
		for j := 0; j < cols; j++ {
			h.must(c.RX.Data[j].Copy(h.m, h.pair(oldX[j].RatString(), oldY[j].RatString())))
			h.must(c.RPrimal.UpdateColumnInInfSet(uint32(j)))
		}
		h.must(c.PrepareSolverXWithSignatureTableau(signature))
		for j := 0; j < cols; j++ {
			h.check(&c.RX.Data[j], wantX[j], wantY[j])
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

package lp

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestLARSolveOnSignatureIndependentModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 100; world++ {
		for _, profile := range []string{"exact-pivot", "presolved", "missing-r-lu", "missing-d-lu", "initial-feasible", "infeasible"} {
			h := newLPPairHarness(t)
			lower, extent := 1+world%17, 20+world%7
			if profile == "initial-feasible" {
				lower = -lower
			}
			if profile == "infeasible" {
				lower = 2*extent + 1
			}
			strict := world%2 == 0
			c := lpLARFloatingSignatureFixture(t, h, lower, extent, strict)
			h.must(c.RPrimal.InfSet.Resize(3))
			signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed})
			var changes []uint32
			if profile == "presolved" || profile == "missing-r-lu" {
				var err error
				changes, err = c.FindSolutionSignatureWithDoubles(signature)
				h.must(err)
			}
			if profile == "missing-r-lu" {
				h.must(c.RPrimal.Factorization.Close())
				c.RPrimal.Factorization = nil
				if !c.NoRLU() {
					t.Fatal("nil exact factorization predicate")
				}
			}
			if profile == "missing-d-lu" {
				c.DPrimal.Factorization.Close()
				c.DPrimal.Factorization = nil
			}
			h.must(c.SolveOnSignature(signature, changes))
			if c.RPrimal.Tracing || !c.RPrimal.LookForFeasibleSolutionOnly || c.NoRLU() {
				t.Fatal("exact solve state", world, profile)
			}
			epsilon := new(big.Rat)
			if strict {
				epsilon.SetInt64(1)
			}
			wantX, wantY := big.NewRat(int64(lower), 1), epsilon
			if profile == "initial-feasible" {
				wantX, wantY = new(big.Rat), new(big.Rat)
			}
			if profile == "infeasible" {
				wantX, wantY = big.NewRat(int64(2*extent), 1), new(big.Rat)
			}
			h.check(&c.RX.Data[0], wantX, wantY)
			h.check(&c.RX.Data[1], new(big.Rat).Quo(new(big.Rat).Neg(wantX), big.NewRat(2, 1)), new(big.Rat).Quo(new(big.Rat).Neg(wantY), big.NewRat(2, 1)))
			h.check(&c.RX.Data[2], new(big.Rat), new(big.Rat))
			if profile == "infeasible" {
				if c.RPrimal.Status != StatusInfeasible {
					t.Fatal("independently impossible bounds", c.RPrimal.Status)
				}
			} else if !c.RPrimal.CurrentXIsFeasible() {
				t.Fatal("independent feasible solution", profile)
			}
			if !reflect.DeepEqual(c.RBasis, c.DBasis) {
				t.Fatal("basis synchronization", world, profile, c.RBasis, c.DBasis)
			}
			// The two source solvers keep independent nonbasic orderings.
			lpCheckLARCatchUpBasis(t, c.DBasis, c.DNBasis, c.DHeading, c.RBasis)
			if profile == "missing-d-lu" && c.DPrimal.Factorization != nil {
				t.Fatal("source retains nil float LU")
			}
			if profile == "exact-pivot" || profile == "missing-d-lu" {
				if !reflect.DeepEqual(c.RPrimal.Trace, []uint32{1, 0}) || c.RPrimal.TotalIterations != 1 {
					t.Fatal("real exact pivot", world, profile, c.RPrimal.Trace, c.RPrimal.TotalIterations)
				}
			}
			if profile == "presolved" || profile == "missing-r-lu" || profile == "initial-feasible" {
				if len(c.RPrimal.Trace) != 0 || c.RPrimal.TotalIterations != 0 {
					t.Fatal("initial exact witness", world, profile)
				}
			}
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

// Exact column1 is zero, while the floating suggestion still has a valid
// column1. The suggested exact basis is singular; the old column0 basis is not.
func lpLARSignatureRollbackFixture(t *testing.T, h *lpPairHarness, lower int) *LARCoreSolver {
	t.Helper()
	c := lpLARFloatingSignatureFixture(t, h, lower, 7, false)
	h.must(c.RPrimal.InfSet.Resize(3))
	lpSetLARCatchUpExactCell(t, h, c, 0, 1, "0")
	h.must(c.DPrimal.ChangeBasisUnconditionally(1, 0))
	h.must(InitFloatFactorization(&c.DPrimal.Factorization, c.DA, c.DBasis, c.settings))
	return c
}

func TestLARSolveOnSignatureDegenerateRollback(t *testing.T) {
	for _, missing := range []bool{false, true} {
		for _, tracing := range []bool{false, true} {
			h := newLPPairHarness(t)
			c := lpLARSignatureRollbackFixture(t, h, -1)
			if missing {
				h.must(c.RPrimal.Factorization.Close())
				c.RPrimal.Factorization = nil
			}
			c.RPrimal.Tracing = tracing
			c.RPrimal.Trace = []uint32{77, 66}
			c.DX = []float64{17, 18, 19}
			old := c.DPrimal.Factorization
			limit := &lpTestResourceLimit{cancel: true, err: lpErrExactSolveCancel}
			c.settings.SetResourceLimit(limit)
			// The source fallback never prepares from this deliberately invalid key.
			signature := lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionAtUpperBound})
			h.must(c.SolveOnSignature(signature, []uint32{1, 0}))
			if limit.calls != 0 || c.RPrimal.Tracing != tracing || !reflect.DeepEqual(c.RPrimal.Trace, []uint32{77, 66}) {
				t.Fatal("fallback cancellation/tracing prefix", missing, tracing, limit.calls, c.RPrimal.Trace)
			}
			if !reflect.DeepEqual(c.RBasis, []uint32{0}) || !reflect.DeepEqual(c.DBasis, c.RBasis) || !reflect.DeepEqual(c.DHeading, c.RHeading) || !reflect.DeepEqual(c.DNBasis, c.RNBasis) {
				t.Fatal("rollback basis", c.RBasis, c.DBasis, c.DNBasis, c.RNBasis)
			}
			if c.DPrimal.Factorization != nil || !old.closed || c.NoRLU() || c.RPrimal.Status != StatusFeasible {
				t.Fatal("rollback LU lifecycle")
			}
			if !reflect.DeepEqual(c.DX, []float64{17, 18, 19}) || !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{99: PositionAtUpperBound}) {
				t.Fatal("fallback value/signature publication")
			}
			c.DBasis[0] = 2
			c.DHeading[0] = 44
			c.DNBasis[0] = 55
			if c.RBasis[0] != 0 || c.RHeading[0] != 0 || c.RNBasis[0] == 55 {
				t.Fatal("copied metadata aliases exact arrays")
			}
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARSolveOnSignatureFallbackErrorBeforeSynchronization(t *testing.T) {
	h := newLPPairHarness(t)
	c := lpLARSignatureRollbackFixture(t, h, 3)
	old := c.DPrimal.Factorization
	limit := &lpTestResourceLimit{err: lpErrExactSolveCancel}
	c.settings.SetResourceLimit(limit)
	err := c.SolveOnSignature(lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionAtUpperBound}), []uint32{1, 0})
	if err != lpErrExactSolveCancel || limit.calls != 1 || c.RBasis[0] != 0 || c.DBasis[0] != 1 || c.DPrimal.Factorization != old || old.closed || !c.RPrimal.LookForFeasibleSolutionOnly {
		t.Fatal("fallback failure prefix", err, limit.calls, c.RBasis, c.DBasis)
	}
	if c.SolveOnSignature(nil, nil) != ErrExactVectorPending {
		t.Fatal("stopped parent")
	}
	h.must(c.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func lpLARRoundedSignatureFixture(t *testing.T, h *lpPairHarness) *LARCoreSolver {
	t.Helper()
	c, _ := lpLARCatchUpFixture(t, h, 2, 4, 1)
	h.must(c.PrefixR())
	h.must(c.RPrimal.InfSet.Resize(4))
	scale, _ := new(big.Int).SetString("1000000000000000000", 10)
	for i, denom := range []int64{3, 7} {
		for j, mult := range []int64{1, 5} {
			n := new(big.Int).Mul(scale, big.NewInt(mult))
			r := new(big.Rat).SetFrac(n, big.NewInt(denom))
			col := uint32(2 + j)
			lpSetLARCatchUpExactCell(t, h, c, uint32(i), col, r.RatString())
			// Source get_double converts numerator and denominator separately.
			nf, _ := r.Num().Float64()
			df, _ := r.Denom().Float64()
			for k := range c.DA.Rows[i] {
				if c.DA.Rows[i][k].Column == col {
					c.DA.Rows[i][k].Value = nf / df
				}
			}
		}
	}
	h.must(c.DPrimal.ChangeBasisUnconditionally(2, 0))
	h.must(c.DPrimal.ChangeBasisUnconditionally(3, 1))
	h.must(InitFloatFactorization(&c.DPrimal.Factorization, c.DA, c.DBasis, c.settings))
	old := c.DPrimal.Factorization
	if old.GetStatus() != LUOK {
		t.Fatal("rounded floating basis must be nonsingular", old.GetStatus())
	}
	return c
}

func TestLARSolveOnSignatureRoundedSingularBasisRollback(t *testing.T) {
	h := newLPPairHarness(t)
	c := lpLARRoundedSignatureFixture(t, h)
	old := c.DPrimal.Factorization
	// Exact col3=5*col2 independently proves the suggested determinant zero.
	h.must(c.SolveOnSignature(lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionAtLowBound}), []uint32{2, 0, 3, 1}))
	if c.NoRLU() || c.RPrimal.Status != StatusFeasible || !reflect.DeepEqual(c.RBasis, []uint32{0, 1}) || !reflect.DeepEqual(c.DBasis, c.RBasis) || c.DPrimal.Factorization != nil || !old.closed {
		t.Fatal("rounded singular rollback", c.RBasis, c.DBasis, c.RPrimal.Status)
	}
	if off, err := c.RPrimal.AMultXIsOff(); err != nil || off {
		t.Fatal("rollback equations", off, err)
	}
	h.must(c.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARSolveOnSignatureCancellationPrefixes(t *testing.T) {
	for _, phase := range []string{"head-cancel", "post-cancel", "head-error", "post-error", "feasible-cancel"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			lower := 3
			if phase == "feasible-cancel" {
				lower = -1
			}
			c := lpLARFloatingSignatureFixture(t, h, lower, 7, false)
			h.must(c.RPrimal.InfSet.Resize(3))
			limit := &lpExactSolveCancelSequence{}
			pivot := false
			wantCalls := 2
			switch phase {
			case "head-cancel":
				limit.values = []bool{true, true}
			case "post-cancel":
				limit.values = []bool{false, true}
				pivot = true
			case "head-error":
				limit.errorAt = 1
				wantCalls = 1
			case "post-error":
				limit.errorAt = 2
				pivot = true
			case "feasible-cancel":
				limit.values = []bool{true}
				wantCalls = 1
			}
			c.settings.SetResourceLimit(limit)
			signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed})
			err := c.SolveOnSignature(signature, nil)
			failed := phase == "head-error" || phase == "post-error"
			if failed && err != lpErrExactSolveCancel || !failed && err != nil {
				t.Fatal("cancellation error", err)
			}
			if limit.calls != wantCalls || !c.RPrimal.Tracing || c.DBasis[0] != 0 || c.DPrimal.Factorization.RefactorCounter != 0 {
				t.Fatal("cancel before tracing stop/float catch-up", limit.calls, c.RBasis, c.DBasis)
			}
			if pivot {
				if !reflect.DeepEqual(c.RPrimal.Trace, []uint32{1, 0}) || c.RBasis[0] != 1 {
					t.Fatal("retained completed pivot", c.RPrimal.Trace, c.RBasis)
				}
				h.check(&c.RX.Data[0], big.NewRat(3, 1), new(big.Rat))
			} else if len(c.RPrimal.Trace) != 0 || c.RBasis[0] != 0 {
				t.Fatal("cancel before pivot")
			}
			if c.stopped != failed {
				t.Fatal("source cancel is not a Go error", c.stopped, failed)
			}
			if !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}) {
				t.Fatal("input signature changed")
			}
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

type lpLARSignaturePostSolveLimit struct {
	c     *LARCoreSolver
	mode  string
	calls int
}

func (l *lpLARSignaturePostSolveLimit) CancelFlag() (bool, error) {
	l.calls++
	if l.calls == 2 {
		switch l.mode {
		case "counter":
			l.c.RPrimal.TotalIterations = 0
		case "float-row":
			l.c.DHeading[0] = 8
		}
	}
	return false, nil
}

func TestLARSolveOnSignaturePostSolvePublication(t *testing.T) {
	for _, mode := range []string{"counter", "float-row"} {
		h := newLPPairHarness(t)
		c := lpLARFloatingSignatureFixture(t, h, 3, 7, false)
		h.must(c.RPrimal.InfSet.Resize(3))
		limit := &lpLARSignaturePostSolveLimit{c: c, mode: mode}
		c.settings.SetResourceLimit(limit)
		err := c.SolveOnSignature(lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}), nil)
		want := ErrBounds
		if mode == "counter" {
			want = nil
			if sourceDebugEnabled {
				want = ErrAssertion
			}
		}
		if err != want || limit.calls != 2 || c.RPrimal.Tracing || !reflect.DeepEqual(c.RPrimal.Trace, []uint32{1, 0}) || c.RBasis[0] != 1 {
			t.Fatal("post-solve prefix", mode, err, want, limit.calls, c.RPrimal.Trace)
		}
		h.check(&c.RX.Data[0], big.NewRat(3, 1), new(big.Rat))
		h.check(&c.RX.Data[1], big.NewRat(-3, 2), new(big.Rat))
		if mode == "counter" && !sourceDebugEnabled {
			if c.DBasis[0] != 1 {
				t.Fatal("native erased assertion must reach catch-up")
			}
		} else if c.DBasis[0] != 0 {
			t.Fatal("failed catch-up must retain floating basis")
		}
		if mode == "float-row" && c.DPrimal.Factorization.RefactorCounter != 1 {
			t.Fatal("reached floating replacement counter")
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARSolveOnSignatureReachedPrefixes(t *testing.T) {
	for _, phase := range []string{"stopped", "nil-r", "pending-r", "nil-lu-pending-r", "tableau", "nil-lu-odd", "nil-lu-bad-change", "target-short", "prepare-low-short", "solve-domain", "fallback-nil-d"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpLARFloatingSignatureFixture(t, h, 3, 7, false)
			h.must(c.RPrimal.InfSet.Resize(3))
			signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed})
			var changes []uint32
			oldR, oldD, oldLow, oldHeading := c.RPrimal, c.DPrimal, c.Bounds.Low.values, c.DHeading
			c.RPrimal.Trace = []uint32{77, 66}
			want := ErrBounds
			switch phase {
			case "stopped":
				c.stopped = true
				want = ErrExactVectorPending
			case "nil-r":
				c.RPrimal = nil
			case "pending-r":
				c.RPrimal.stopped = true
				want = ErrExactVectorPending
			case "nil-lu-pending-r":
				h.must(c.RPrimal.Factorization.Close())
				c.RPrimal.Factorization = nil
				c.RPrimal.stopped = true
				want = ErrExactVectorPending
			case "tableau":
				c.settings.SimplexStrategy = StrategyTableauRows
				h.must(c.Bounds.Low.Set(0, h.pair("-1", "0")))
				if sourceDebugEnabled {
					want = ErrAssertion
				} else {
					want = nil
				}
			case "nil-lu-odd":
				h.must(c.RPrimal.Factorization.Close())
				c.RPrimal.Factorization = nil
				changes = []uint32{1, 0, 2}
			case "nil-lu-bad-change":
				h.must(c.RPrimal.Factorization.Close())
				c.RPrimal.Factorization = nil
				changes = []uint32{1, 7}
			case "target-short":
				c.DHeading = nil
				changes = []uint32{1, 0}
			case "prepare-low-short":
				signature.Set(1, PositionAtLowBound)
				c.Bounds.Low.values = c.Bounds.Low.values[:1]
			case "solve-domain":
				h.must(c.RPrimal.InfSet.Resize(0))
			case "fallback-nil-d":
				h.must(c.Close())
				c = lpLARSignatureRollbackFixture(t, h, -1)
				oldR, oldD = c.RPrimal, c.DPrimal
				oldLow, oldHeading = c.Bounds.Low.values, c.DHeading
				c.DPrimal = nil
				changes = []uint32{1, 0}
				signature = lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionAtLowBound})
			}
			err := c.SolveOnSignature(signature, changes)
			if err != want {
				t.Fatal("reached failure", err, want)
			}
			if phase == "nil-lu-odd" {
				if c.RBasis[0] != 1 || c.RPrimal.Factorization != nil {
					t.Fatal("nil LU trace prefix", c.RBasis)
				}
			}
			if phase == "prepare-low-short" && (!reflect.DeepEqual(c.RPrimal.Trace, []uint32{77, 66}) || c.RPrimal.Tracing) {
				t.Fatal("prepare must precede trace start")
			}
			if phase == "solve-domain" && (!c.RPrimal.Tracing || !c.RPrimal.LookForFeasibleSolutionOnly || len(c.RPrimal.Trace) != 0) {
				t.Fatal("solve failure after trace start")
			}
			if phase == "fallback-nil-d" && (c.RBasis[0] != 0 || c.DBasis[0] != 0 || oldD.Factorization.closed) {
				t.Fatal("fallback nil d reached after metadata copy")
			}
			if err != nil && c.SolveOnSignature(nil, nil) != ErrExactVectorPending {
				t.Fatal("stopped reuse")
			}
			c.RPrimal, c.DPrimal = oldR, oldD
			c.Bounds.Low.values, c.DHeading = oldLow, oldHeading
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLARSolveOnSignaturePhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		for _, path := range []string{"init-r", "catch-r", "prepare-r", "solve-r", "rollback"} {
			success, failures := false, 0
			for ordinal := int64(0); ordinal < 2000; ordinal++ {
				done := false
				t.Run(fmt.Sprintf("%t/%s/%d", direct, path, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					c := lpLARFloatingSignatureFixture(t, h, 3, 7, false)
					h.must(c.RPrimal.InfSet.Resize(3))
					huge := strings.Repeat("1234567890", 20) + "7/13"
					signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed})
					var changes []uint32
					switch path {
					case "init-r":
						lpSetLARSignatureBothMatrices(t, h, c, 0, 0, huge)
						h.must(InitFloatFactorization(&c.DPrimal.Factorization, c.DA, c.DBasis, c.settings))
						h.must(c.Bounds.Low.Set(0, h.pair("-1", "0")))
						h.must(c.RPrimal.Factorization.Close())
						c.RPrimal.Factorization = nil
					case "catch-r":
						lpSetLARSignatureBothMatrices(t, h, c, 0, 1, huge)
						h.must(c.DPrimal.ChangeBasisUnconditionally(1, 0))
						h.must(InitFloatFactorization(&c.DPrimal.Factorization, c.DA, c.DBasis, c.settings))
						h.must(c.Bounds.Low.Set(1, h.pair("-"+huge, "0")))
						changes = []uint32{1, 0}
						signature = lpSignature(map[uint32]NonBasicColumnValuePosition{0: PositionAtLowBound, 2: PositionAtFixed})
					case "prepare-r":
						h.must(c.Bounds.Low.Set(1, h.pair("-"+huge, "0")))
						signature.Set(1, PositionAtLowBound)
					case "solve-r":
						h.must(c.Bounds.Low.Set(0, h.pair(huge, "0")))
						h.must(c.Bounds.Low.Set(1, h.pair("-"+huge, "0")))
					case "rollback":
						h.must(c.Close())
						c = lpLARRoundedSignatureFixture(t, h)
						lpSetLARSignatureBothMatrices(t, h, c, 0, 0, huge)
						h.must(InitPairFactorization(&c.RPrimal.Factorization, c.RA, c.RBasis, c.settings))
						changes = []uint32{2, 0, 3, 1}
						signature = lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionAtUpperBound})
					}
					h.budget.FailAfter(ordinal)
					err := c.SolveOnSignature(signature, changes)
					h.budget.FailAfter(-1)
					if err == nil {
						success, done = true, true
						if c.NoRLU() || !c.RPrimal.CurrentXIsFeasible() {
							t.Fatal("successful source feasibility", path, c.RPrimal.Status)
						}
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if c.SolveOnSignature(signature, changes) != ErrExactVectorPending {
							t.Fatal("stopped retry")
						}
						if path == "init-r" && c.RBasis[0] != 0 {
							t.Fatal("initialization cannot exchange basis")
						}
						if path == "prepare-r" && !c.RPrimal.LookForFeasibleSolutionOnly && c.RPrimal.Tracing {
							t.Fatal("prepare refusal precedes trace start")
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

func lpSetLARSignatureBothMatrices(t *testing.T, h *lpPairHarness, c *LARCoreSolver, row, col uint32, value string) {
	t.Helper()
	lpSetLARCatchUpExactCell(t, h, c, row, col, value)
	r := mustLPRat(t, value)
	n, _ := r.Num().Float64()
	d, _ := r.Denom().Float64()
	for k := range c.DA.Rows[row] {
		if c.DA.Rows[row][k].Column == col {
			c.DA.Rows[row][k].Value = n / d
			return
		}
	}
	t.Fatal("missing float coefficient")
}

func TestLARSolveOnSignaturePreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpLARFloatingSignatureFixture(t, h, 3, 7, false)
			h.must(c.RPrimal.InfSet.Resize(3))
			huge := strings.Repeat("1234567890", 20) + "7/13"
			h.must(c.Bounds.Low.Set(1, h.pair("-"+huge, "0")))
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
				err = c.SolveOnSignature(lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtLowBound, 2: PositionAtFixed}), nil)
				if err == nil || c.calls.comparison != 0 {
					t.Fatal("adopted caller comparison", err)
				}
			}
			h.must(c.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("consumed older caller comparison")
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

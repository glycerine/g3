package lp

// Source: src/util/lp/lar_core_solver.h:487-515.

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func lpPrepareLARSignatureTableau(t *testing.T, h *lpPairHarness, c *LARCoreSolver, strategy SimplexStrategy) {
	t.Helper()
	c.settings.SimplexStrategy = strategy
	c.RPrimal.Names = lpFeasibleNames{}
	h.must(c.PrefixR())
	if c.RPrimal.Factorization != nil {
		h.must(c.RPrimal.Factorization.Close())
		c.RPrimal.Factorization = nil
	}
	for row, j := range c.RBasis {
		ok, err := c.RPrimal.PivotColumnTableau(j, uint32(row))
		h.must(err)
		if !ok {
			t.Fatal("initial canonical basis")
		}
	}
	h.must(c.RPrimal.InfSet.Resize(c.RA.ColumnCount()))
	h.must(c.RPrimal.InitInfSet())
	// Supply the real initial tableau costs, rather than scope-only zeros.
	c.RPrimal.UsingInfeasCosts = false
	if strategy == StrategyTableauCosts {
		h.must(c.RPrimal.InitReducedCostsTableau())
	}
}

func TestLARSolveOnSignatureTableauIndependentModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts} {
		for world := 0; world < 60; world++ {
			for _, profile := range []string{"exact-pivot", "presolved", "missing-d-lu", "initial-feasible", "infeasible", "signature-change"} {
				t.Run(fmt.Sprintf("%d/%d/%s", strategy, world, profile), func(t *testing.T) {
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
					signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed})
					var changes []uint32
					if profile == "presolved" {
						var err error
						changes, err = c.FindSolutionSignatureWithDoubles(signature)
						h.must(err)
					}
					lpPrepareLARSignatureTableau(t, h, c, strategy)
					if profile == "missing-d-lu" {
						c.DPrimal.Factorization.Close()
						c.DPrimal.Factorization = nil
					}
					if profile == "signature-change" {
						signature.Set(1, PositionAtLowBound)
					}
					original := signature.Positions()
					h.must(c.SolveOnSignatureTableau(signature, changes))
					if c.RPrimal.Tracing || !c.RPrimal.LookForFeasibleSolutionOnly || c.RPrimal.Factorization != nil {
						t.Fatal("tableau lifecycle")
					}
					wantX, wantY := big.NewRat(int64(lower), 1), new(big.Rat)
					if strict {
						wantY.SetInt64(1)
					}
					if profile == "initial-feasible" {
						wantX, wantY = new(big.Rat), new(big.Rat)
					}
					if profile == "infeasible" && strategy == StrategyTableauCosts || profile == "signature-change" {
						wantX, wantY = big.NewRat(int64(2*extent), 1), new(big.Rat)
					}
					h.check(&c.RX.Data[0], wantX, wantY)
					h.check(&c.RX.Data[1], new(big.Rat).Quo(new(big.Rat).Neg(wantX), big.NewRat(2, 1)), new(big.Rat).Quo(new(big.Rat).Neg(wantY), big.NewRat(2, 1)))
					h.check(&c.RX.Data[2], new(big.Rat), new(big.Rat))
					if profile == "infeasible" {
						if c.RPrimal.Status != StatusInfeasible {
							t.Fatal("independent impossible interval", c.RPrimal.Status)
						}
					} else if !c.RPrimal.CurrentXIsFeasible() {
						t.Fatal("independent feasible interval", c.RPrimal.Status)
					}
					lpCheckLARCatchUpBasis(t, c.DBasis, c.DNBasis, c.DHeading, c.RBasis)
					if !lpSignatureEqual(signature, original) {
						t.Fatal("signature mutated")
					}
					if profile == "missing-d-lu" && c.DPrimal.Factorization != nil {
						t.Fatal("discarded float LU rebuilt")
					}
					if profile == "exact-pivot" || profile == "missing-d-lu" {
						if !reflect.DeepEqual(c.RPrimal.Trace, []uint32{1, 0}) || c.RPrimal.TotalIterations != 1 {
							t.Fatal("actual exact pivot", c.RPrimal.Trace, c.RPrimal.TotalIterations)
						}
					}
					h.must(c.Close())
					lpExactVectorNamedCensus(t, h, nil)
				})
			}
		}
	}
}

func TestLARSolveOnSignatureTableauSingularFallback(t *testing.T) {
	for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts} {
		for _, tracing := range []bool{false, true} {
			h := newLPPairHarness(t)
			c := lpLARSignatureRollbackFixture(t, h, -1)
			lpPrepareLARSignatureTableau(t, h, c, strategy)
			c.RPrimal.Tracing = tracing
			c.RPrimal.Trace = []uint32{77, 66}
			c.DX = []float64{17, 18, 19}
			old := c.DPrimal.Factorization
			// Unlike LU fallback, tableau fallback does apply the signature. Column1
			// is zero in the exact matrix, so this bound move preserves the equation.
			h.must(c.Bounds.Low.Set(1, h.pair("-3", "1/7")))
			limit := &lpTestResourceLimit{cancel: true, err: lpErrExactSolveCancel}
			c.settings.SetResourceLimit(limit)
			signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtLowBound, 2: PositionAtFixed})
			h.must(c.SolveOnSignatureTableau(signature, []uint32{1, 0}))
			h.check(&c.RX.Data[1], big.NewRat(-3, 1), big.NewRat(1, 7))
			if limit.calls != 0 || c.RPrimal.Tracing != tracing || !reflect.DeepEqual(c.RPrimal.Trace, []uint32{77, 66}) {
				t.Fatal("fallback source cancellation/trace", limit.calls, c.RPrimal.Trace)
			}
			if c.DBasis[0] != 0 || !reflect.DeepEqual(c.RHeading, c.DHeading) || !reflect.DeepEqual(c.RNBasis, c.DNBasis) || c.DPrimal.Factorization != nil || !old.closed {
				t.Fatal("fallback metadata/LU lifecycle")
			}
			if !reflect.DeepEqual(c.DX, []float64{17, 18, 19}) {
				t.Fatal("floating values changed")
			}
			c.DBasis[0] = 2
			c.DHeading[0] = 44
			c.DNBasis[0] = 55
			if c.RBasis[0] != 0 || c.RHeading[0] != 0 || c.RNBasis[0] == 55 {
				t.Fatal("metadata copy aliases exact")
			}
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARSolveOnSignatureTableauRoundedSingularFallback(t *testing.T) {
	for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts} {
		for _, tracing := range []bool{false, true} {
			h := newLPPairHarness(t)
			c := lpLARRoundedSignatureFixture(t, h)
			old := c.DPrimal.Factorization
			lpPrepareLARSignatureTableau(t, h, c, strategy)
			c.RPrimal.Tracing = tracing
			c.RPrimal.Trace = []uint32{77, 66}
			limit := &lpTestResourceLimit{cancel: true, err: lpErrExactSolveCancel}
			c.settings.SetResourceLimit(limit)
			h.must(c.SolveOnSignatureTableau(lpSignature(map[uint32]NonBasicColumnValuePosition{0: PositionFreeOfBounds, 1: NonBasicColumnValuePosition(99), 3: PositionFreeOfBounds}), []uint32{2, 0, 3, 1}))
			if !reflect.DeepEqual(c.RBasis, []uint32{2, 1}) || !reflect.DeepEqual(c.DBasis, c.RBasis) || !reflect.DeepEqual(c.DHeading, c.RHeading) || !reflect.DeepEqual(c.DNBasis, c.RNBasis) {
				t.Fatal("keep earlier successful exchange", c.RBasis, c.DBasis)
			}
			wantTrace := []uint32{77, 66}
			if tracing {
				wantTrace = append(wantTrace, 2, 0)
			}
			if c.RPrimal.Tracing != tracing || !reflect.DeepEqual(c.RPrimal.Trace, wantTrace) || limit.calls != 0 || c.DPrimal.Factorization != nil || !old.closed || c.RPrimal.Factorization != nil {
				t.Fatal("rounded fallback lifecycle", c.RPrimal.Trace, limit.calls)
			}
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARSolveOnSignatureTableauCancellationPrefixes(t *testing.T) {
	for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts} {
		for _, phase := range []string{"head-cancel", "head-error", "exit-cancel", "exit-error", "post-cancel", "post-error", "transient-exit-cancel", "feasible-cancel"} {
			t.Run(fmt.Sprintf("%d/%s", strategy, phase), func(t *testing.T) {
				h := newLPPairHarness(t)
				lower := 3
				if phase == "feasible-cancel" {
					lower = -1
				}
				c := lpLARFloatingSignatureFixture(t, h, lower, 7, false)
				lpPrepareLARSignatureTableau(t, h, c, strategy)
				limit := &lpExactSolveCancelSequence{}
				calls, pivot, stoppedTrace := 3, true, false
				wantErr := error(nil)
				switch phase {
				case "head-cancel":
					limit.values = []bool{true, true}
					calls = 2
					pivot = false
				case "head-error":
					limit.errorAt = 1
					calls = 1
					pivot = false
					wantErr = lpErrExactSolveCancel
				case "exit-cancel":
					limit.values = []bool{false, true, true}
				case "exit-error":
					limit.errorAt = 2
					calls = 2
					wantErr = lpErrExactSolveCancel
				case "post-cancel":
					limit.values = []bool{false, false, true}
				case "post-error":
					limit.errorAt = 3
					wantErr = lpErrExactSolveCancel
				case "transient-exit-cancel":
					limit.values = []bool{false, true, false}
					stoppedTrace = true
				case "feasible-cancel":
					limit.values = []bool{true}
					calls = 1
					pivot = false
				}
				c.settings.SetResourceLimit(limit)
				err := c.SolveOnSignatureTableau(lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}), nil)
				if err != wantErr || limit.calls != calls || c.RPrimal.Tracing == stoppedTrace || c.stopped != (wantErr != nil) {
					t.Fatal("source cancellation prefix", err, wantErr, limit.calls, c.RPrimal.Tracing, c.stopped)
				}
				wantR, wantD := uint32(0), uint32(0)
				if pivot {
					wantR = 1
					if !reflect.DeepEqual(c.RPrimal.Trace, []uint32{1, 0}) {
						t.Fatal("actual completed pivot", c.RPrimal.Trace)
					}
				} else if len(c.RPrimal.Trace) != 0 {
					t.Fatal("trace before pivot")
				}
				if stoppedTrace {
					wantD = 1
				}
				if c.RBasis[0] != wantR || c.DBasis[0] != wantD {
					t.Fatal("basis prefix", c.RBasis, c.DBasis)
				}
				h.must(c.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
	}
}

func TestLARSolveOnSignatureTableauFallbackCancellation(t *testing.T) {
	for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts} {
		for _, failed := range []bool{false, true} {
			h := newLPPairHarness(t)
			c := lpLARSignatureRollbackFixture(t, h, 3)
			lpPrepareLARSignatureTableau(t, h, c, strategy)
			old := c.DPrimal.Factorization
			limit := &lpExactSolveCancelSequence{values: []bool{false, true}}
			if failed {
				limit.errorAt = 1
			}
			c.settings.SetResourceLimit(limit)
			err := c.SolveOnSignatureTableau(lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}), []uint32{1, 0})
			if failed {
				if err != lpErrExactSolveCancel || limit.calls != 1 || c.DBasis[0] != 1 || old.closed || c.DPrimal.Factorization != old {
					t.Fatal("fallback failure before copy", err, limit.calls, c.DBasis)
				}
			} else {
				h.must(err)
				if limit.calls != 2 || c.RPrimal.Status != StatusCancelled || c.DBasis[0] != 0 || c.DPrimal.Factorization != nil || !old.closed {
					t.Fatal("fallback cancellation still synchronizes", limit.calls, c.RPrimal.Status, c.DBasis)
				}
			}
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARSolveOnSignatureTableauReachedPrefixes(t *testing.T) {
	for _, phase := range []string{"stopped", "nil-r", "pending-r", "lu-profile", "bad-basis", "odd-change", "target-short", "prepare-low-short", "prepare-key", "nil-d", "fallback-prepare", "fallback-nil-d"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpLARFloatingSignatureFixture(t, h, -1, 7, false)
			if phase == "fallback-prepare" || phase == "fallback-nil-d" {
				h.must(c.Close())
				c = lpLARSignatureRollbackFixture(t, h, -1)
			}
			lpPrepareLARSignatureTableau(t, h, c, StrategyTableauRows)
			oldR, oldD, oldLow, oldHeading := c.RPrimal, c.DPrimal, c.Bounds.Low.values, c.DHeading
			signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed})
			var changes []uint32
			c.RPrimal.Trace = []uint32{77, 66}
			want := error(ErrBounds)
			switch phase {
			case "stopped":
				c.stopped = true
				want = ErrExactVectorPending
			case "nil-r":
				c.RPrimal = nil
			case "pending-r":
				c.RPrimal.stopped = true
				want = ErrExactVectorPending
			case "lu-profile":
				c.settings.SimplexStrategy = StrategyLU
				if sourceDebugEnabled {
					want = ErrAssertion
				} else {
					want = nil
				}
			case "bad-basis":
				lpSetLARCatchUpExactCell(t, h, c, 0, 0, "2")
				if sourceDebugEnabled {
					want = ErrAssertion
				} else {
					want = nil
				}
			case "odd-change":
				changes = []uint32{1}
			case "target-short":
				c.DHeading = nil
				changes = []uint32{1, 0}
			case "prepare-low-short":
				signature.Set(1, PositionAtLowBound)
				c.Bounds.Low.values = c.Bounds.Low.values[:1]
			case "prepare-key":
				signature.Set(99, PositionAtLowBound)
			case "nil-d":
				c.DPrimal = nil
			case "fallback-prepare":
				changes = []uint32{1, 0}
				signature.Set(99, PositionAtLowBound)
			case "fallback-nil-d":
				changes = []uint32{1, 0}
				c.DPrimal = nil
			}
			err := c.SolveOnSignatureTableau(signature, changes)
			if err != want {
				t.Fatal("source stage", err, want)
			}
			if phase == "prepare-low-short" || phase == "prepare-key" || phase == "fallback-prepare" {
				if c.RPrimal.Tracing || !reflect.DeepEqual(c.RPrimal.Trace, []uint32{77, 66}) {
					t.Fatal("preparation precedes tracing", c.RPrimal.Trace)
				}
			}
			if phase == "fallback-nil-d" && (c.DBasis[0] != 0 || oldD.Factorization.closed) {
				t.Fatal("copy precedes nil floating owner guard")
			}
			if err != nil && c.SolveOnSignatureTableau(nil, nil) != ErrExactVectorPending {
				t.Fatal("stopped retry")
			}
			c.RPrimal, c.DPrimal = oldR, oldD
			c.Bounds.Low.values = oldLow
			c.DHeading = oldHeading
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

type lpLARTableauPostSolveLimit struct {
	c     *LARCoreSolver
	mode  string
	calls int
}

func (l *lpLARTableauPostSolveLimit) CancelFlag() (bool, error) {
	l.calls++
	if l.calls == 3 {
		if l.mode == "counter" {
			l.c.RPrimal.TotalIterations = 0
		} else {
			l.c.DHeading[0] = 8
		}
	}
	return false, nil
}
func TestLARSolveOnSignatureTableauPostSolvePublication(t *testing.T) {
	for _, mode := range []string{"counter", "float-row"} {
		h := newLPPairHarness(t)
		c := lpLARFloatingSignatureFixture(t, h, 3, 7, false)
		lpPrepareLARSignatureTableau(t, h, c, StrategyTableauRows)
		limit := &lpLARTableauPostSolveLimit{c: c, mode: mode}
		c.settings.SetResourceLimit(limit)
		err := c.SolveOnSignatureTableau(lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}), nil)
		want := error(ErrBounds)
		if mode == "counter" {
			want = nil
			if sourceDebugEnabled {
				want = ErrAssertion
			}
		}
		if err != want || limit.calls != 3 || c.RPrimal.Tracing || !reflect.DeepEqual(c.RPrimal.Trace, []uint32{1, 0}) || c.RBasis[0] != 1 {
			t.Fatal("post-solve prefix", err, want, limit.calls, c.RPrimal.Trace)
		}
		h.check(&c.RX.Data[0], big.NewRat(3, 1), new(big.Rat))
		h.check(&c.RX.Data[1], big.NewRat(-3, 2), new(big.Rat))
		wantD := uint32(0)
		if mode == "counter" && !sourceDebugEnabled {
			wantD = 1
		}
		if c.DBasis[0] != wantD {
			t.Fatal("float basis prefix", c.DBasis, wantD)
		}
		if mode == "float-row" && c.DPrimal.Factorization.RefactorCounter != 1 {
			t.Fatal("reached replacement counter")
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARSolveOnSignatureTableauPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts} {
		for _, direct := range []bool{false, true} {
			for _, path := range []string{"catch", "prepare", "solve", "fallback", "rounded"} {
				success, failures := false, 0
				for ordinal := int64(0); ordinal < 2000; ordinal++ {
					done := false
					t.Run(fmt.Sprintf("%d/%t/%s/%d", strategy, direct, path, ordinal), func(t *testing.T) {
						var h *lpPairHarness
						if direct {
							h = lpNewDirectSparseHarness(t)
						} else {
							h = newLPPairHarness(t)
						}
						lower := -1
						if path == "solve" {
							lower = 3
						}
						var c *LARCoreSolver
						if path == "fallback" {
							c = lpLARSignatureRollbackFixture(t, h, -1)
						} else if path == "rounded" {
							c = lpLARRoundedSignatureFixture(t, h)
						} else {
							c = lpLARFloatingSignatureFixture(t, h, lower, 7, false)
						}
						lpPrepareLARSignatureTableau(t, h, c, strategy)
						huge := strings.Repeat("1234567890", 20) + "7/13"
						signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed})
						var changes []uint32
						switch path {
						case "catch":
							lpSetLARSignatureBothMatrices(t, h, c, 0, 1, huge)
							h.must(c.DPrimal.ChangeBasisUnconditionally(1, 0))
							h.must(InitFloatFactorization(&c.DPrimal.Factorization, c.DA, c.DBasis, c.settings))
							changes = []uint32{1, 0}
							signature = nil
						case "prepare":
							h.must(c.Bounds.Low.Set(1, h.pair("-"+huge, "1/17")))
							signature.Set(1, PositionAtLowBound)
						case "solve":
							h.must(c.Bounds.Low.Set(0, h.pair(huge, "1/17")))
						case "fallback":
							h.must(c.Bounds.Low.Set(1, h.pair("-"+huge, "1/17")))
							signature.Set(1, PositionAtLowBound)
							changes = []uint32{1, 0}
						case "rounded":
							signature = lpSignature(map[uint32]NonBasicColumnValuePosition{0: PositionFreeOfBounds, 3: PositionFreeOfBounds})
							changes = []uint32{2, 0, 3, 1}
						}
						oldD := c.DPrimal.Factorization
						h.budget.FailAfter(ordinal)
						err := c.SolveOnSignatureTableau(signature, changes)
						h.budget.FailAfter(-1)
						if err == nil {
							success, done = true, true
							if c.RPrimal.Factorization != nil {
								t.Fatal("tableau created exact LU")
							}
							lpCheckLARCatchUpBasis(t, c.DBasis, c.DNBasis, c.DHeading, c.RBasis)
							if (path == "fallback" || path == "rounded") && (c.DPrimal.Factorization != nil || !oldD.closed) {
								t.Fatal("successful fallback must discard float LU")
							}
						} else {
							failures++
							if !lpExactPriorityPhysicalRefusal(err) {
								t.Fatal(err)
							}
							if c.SolveOnSignatureTableau(nil, nil) != ErrExactVectorPending {
								t.Fatal("stopped retry")
							}
							if (path == "fallback" || path == "rounded") && (oldD.closed || c.DPrimal.Factorization != oldD) {
								t.Fatal("numerical error precedes float disposal")
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
					t.Fatal("physical campaign", strategy, direct, path, success, failures)
				}
			}
		}
	}
}

func TestLARSolveOnSignatureTableauPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpLARFloatingSignatureFixture(t, h, -1, 7, false)
			lpPrepareLARSignatureTableau(t, h, c, StrategyTableauRows)
			c.Bounds.Types.values[1] = ColumnBoxed
			h.must(c.Bounds.Low.Set(1, h.pair("0", "0")))
			h.must(c.Bounds.Upper.Set(1, h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")))
			h.must(c.RX.Data[1].Copy(h.m, h.pair("1", "0")))
			h.must(c.RX.Data[0].Copy(h.m, h.pair("-2", "0")))
			h.must(c.RPrimal.UpdateColumnInInfSet(0))
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
				err = c.SolveOnSignatureTableau(lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionNotAtBound}), nil)
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

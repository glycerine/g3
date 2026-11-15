package lp

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

// One homogeneous equation x0+2*x1+3*x2=0, x2=0. Independent
// elimination gives x0=-2*x1 and x0 in [0,2*extent].
func lpLARFloatingSignatureFixture(t *testing.T, h *lpPairHarness, lower, extent int, strict bool) *LARCoreSolver {
	t.Helper()
	c := lpNewCoreScopeFixture(t, h)
	h.must(c.PrefixR())
	h.must(c.PrefixD())
	// The scope fixture deliberately retains diagnostic count sentinels.
	// This coupled solver fixture also runs exact pricing, whose estimates
	// must describe the live matrix rather than those scope-only values.
	clear(c.RPrimal.ColumnsNZ)
	clear(c.RPrimal.RowsNZ)
	h.must(c.RPrimal.InitColumnRowNonZeroes())
	c.Bounds.Types.values[0] = ColumnLowBound
	c.Bounds.Types.values[1] = ColumnBoxed
	c.Bounds.Types.values[2] = ColumnFixed
	epsilon := "0"
	if strict {
		epsilon = "1"
	}
	h.must(c.Bounds.Low.Set(0, h.pair(fmt.Sprint(lower), epsilon)))
	h.must(c.Bounds.Low.Set(1, h.pair(fmt.Sprint(-extent), "0")))
	c.DPrimal.Trace = []uint32{99, 98}
	return c
}

func TestLARFloatingSignatureReachedPrefixes(t *testing.T) {
	for _, phase := range []string{"stopped", "nil-solver", "nil-lu", "bad-lu", "bounds", "pair-extract", "nil-signature", "prepare", "solve"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			c := lpLARFloatingSignatureFixture(t, h, 3, 7, false)
			signature := lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionFreeOfBounds})
			old := map[uint32]NonBasicColumnValuePosition{99: PositionFreeOfBounds}
			initial := map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}
			savedD, savedLU := c.DPrimal, c.DPrimal.Factorization
			savedTypes, savedX := c.Bounds.Types.values, c.DX
			c.DLow[0] = 91
			argument := signature
			early := false
			want := ErrBounds
			switch phase {
			case "stopped":
				c.stopped = true
				want = ErrExactVectorPending
			case "nil-solver":
				c.DPrimal = nil
			case "nil-lu":
				c.DPrimal.Factorization = nil
				early = true
			case "bad-lu":
				savedLU.status = LUDegenerated
				early = true
			case "bounds":
				c.Bounds.Types.values = c.Bounds.Types.values[:2]
			case "pair-extract":
				c.RPrimal.stopped = true
				want = ErrExactVectorPending
			case "nil-signature":
				argument = nil
			case "prepare":
				c.DX = c.DX[:1]
			case "solve":
				h.must(c.DPrimal.InfSet.Resize(0))
			}
			trace, err := c.FindSolutionSignatureWithDoubles(argument)
			if early {
				if err != nil {
					t.Fatal(err)
				}
			} else if err != want {
				t.Fatal("source stage", err, want)
			}
			if len(trace) != 0 {
				t.Fatal("failure returned trace", trace)
			}
			extracted := phase == "prepare" || phase == "solve"
			if extracted && !lpSignatureEqual(signature, initial) || !extracted && !lpSignatureEqual(signature, old) {
				t.Fatal("signature publication", signature)
			}
			if phase == "solve" {
				if !savedD.Tracing || len(savedD.Trace) != 0 || !savedD.LookForFeasibleSolutionOnly {
					t.Fatal("solve failure prefix")
				}
			} else if savedD.Tracing || !reflect.DeepEqual(savedD.Trace, []uint32{99, 98}) {
				t.Fatal("premature trace start")
			}
			if early && c.DLow[0] != 91 {
				t.Fatal("early factorization guard touched bounds")
			}
			if !early && phase != "stopped" {
				if _, e := c.FindSolutionSignatureWithDoubles(signature); e != ErrExactVectorPending {
					t.Fatal("stopped retry", e)
				}
			}
			c.DPrimal = savedD
			c.DPrimal.Factorization = savedLU
			c.Bounds.Types.values, c.DX = savedTypes, savedX
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

// Mutation occurs at the real post-Solve cancellation read, after the source
// has completed its feasibility checks and before final signature extraction.
type lpLARSignatureExtractionLimit struct {
	c     *LARCoreSolver
	calls int
}

func (l *lpLARSignatureExtractionLimit) CancelFlag() (bool, error) {
	l.calls++
	l.c.DHeading = append(l.c.DHeading, -3)
	return false, nil
}

func TestLARFloatingSignatureFinalExtractionPrefix(t *testing.T) {
	h := newLPPairHarness(t)
	c := lpLARFloatingSignatureFixture(t, h, -1, 7, false)
	limit := &lpLARSignatureExtractionLimit{c: c}
	c.settings.SetResourceLimit(limit)
	signature := lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionFreeOfBounds})
	trace, err := c.FindSolutionSignatureWithDoubles(signature)
	if err != ErrBounds || len(trace) != 0 || c.DPrimal.Tracing || limit.calls != 1 || !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}) {
		t.Fatal("final extraction prefix", trace, err, signature, limit.calls)
	}
	h.must(c.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARFloatingSignaturePhysicalOwnership(t *testing.T) {
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
				c := lpLARFloatingSignatureFixture(t, h, -1, 7, false)
				c.Bounds.Types.values[0] = ColumnFree
				huge := strings.Repeat("1234567890", 20) + "7/13"
				h.must(c.Bounds.Low.Set(1, h.pair("-"+huge, "0")))
				h.must(c.Bounds.Upper.Set(1, h.pair(huge, "0")))
				h.must(c.RX.Data[1].Copy(h.m, h.pair(huge, "0")))
				signature := lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionFreeOfBounds})
				h.budget.FailAfter(ordinal)
				_, err := c.FindSolutionSignatureWithDoubles(signature)
				h.budget.FailAfter(-1)
				if err == nil {
					success, done = true, true
				} else {
					failures++
					if !lpExactPriorityPhysicalRefusal(err) {
						t.Fatal(err)
					}
					if _, e := c.FindSolutionSignatureWithDoubles(signature); e != ErrExactVectorPending {
						t.Fatal("stopped retry", e)
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
			t.Fatal("physical campaign", direct, success, failures)
		}
	}
}

func TestLARFloatingSignatureIndependentFeasibilityModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 180; world++ {
		h := newLPPairHarness(t)
		extent := 2 + world%17
		lower := 1 + world%extent
		if world%3 == 1 {
			lower = -lower
		}
		if world%3 == 2 {
			lower = 2*extent + 1
		}
		strict := world%2 == 0
		c := lpLARFloatingSignatureFixture(t, h, lower, extent, strict)
		signature := lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionFreeOfBounds})
		trace, err := c.FindSolutionSignatureWithDoubles(signature)
		h.must(err)
		target := float64(lower)
		if strict {
			target += 1e-6
		}
		feasible := target <= float64(2*extent)
		if c.DPrimal.Tracing || !c.DPrimal.LookForFeasibleSolutionOnly {
			t.Fatal("tracing/feasibility mode", world)
		}
		if feasible {
			if c.DX[0] < target-1e-9 || c.DX[1] < float64(-extent)-1e-9 || c.DX[1] > 1e-9 || c.DX[2] != 0 || math.Abs(c.DX[0]+2*c.DX[1]) > 1e-9 {
				t.Fatal("independent feasibility", world, c.DX, target)
			}
			if target > 0 {
				if !reflect.DeepEqual(trace, []uint32{1, 0}) || !reflect.DeepEqual(c.DBasis, []uint32{1}) || math.Abs(c.DX[0]-target) > 1e-9 || math.Abs(c.DX[1]+target/2) > 1e-9 {
					t.Fatal("independent exchanged solution", world, trace, c.DX, c.DBasis)
				}
				if !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{0: PositionAtLowBound, 2: PositionAtFixed}) {
					t.Fatal("exchanged signature", signature)
				}
			} else if len(trace) != 0 || !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}) {
				t.Fatal("initial feasible signature", trace, signature)
			}
		} else {
			if c.DPrimal.Status != StatusInfeasible || c.DX[0] != float64(2*extent) || c.DX[1] != float64(-extent) || len(trace) != 0 {
				t.Fatal("independent impossible interval", world, c.DPrimal.Status, c.DX, trace)
			}
			if !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{1: PositionAtLowBound, 2: PositionAtFixed}) {
				t.Fatal("infeasible bound signature", signature)
			}
		}
		if len(trace) > 0 {
			trace[0] = 71
			if c.DPrimal.Trace[0] != 1 {
				t.Fatal("returned trace aliases solver")
			}
		}
		if len(c.RBasis) != 1 || c.RBasis[0] != 0 {
			t.Fatal("floating solve changed exact basis")
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARFloatingSignatureCancellationPrefixes(t *testing.T) {
	for _, pivot := range []bool{false, true} {
		for _, failed := range []bool{false, true} {
			h := newLPPairHarness(t)
			lower := -1
			if pivot {
				lower = 3
			}
			c := lpLARFloatingSignatureFixture(t, h, lower, 7, false)
			limit := &lpTestResourceLimit{cancel: true}
			sentinel := errors.New("signature cancellation read")
			if failed {
				limit.err = sentinel
			}
			// Already-feasible Solve performs no cancellation reads. A pivoting
			// search reads at its loop head, so cancellation leaves the initial X.
			c.settings.SetResourceLimit(limit)
			signature := lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionFreeOfBounds})
			trace, err := c.FindSolutionSignatureWithDoubles(signature)
			if failed && !errors.Is(err, sentinel) || !failed && err != nil {
				t.Fatal("cancel error", pivot, failed, err)
			}
			if len(trace) != 0 || !c.DPrimal.Tracing || !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}) {
				t.Fatal("source cancellation prefix", pivot, failed, trace, signature, c.DPrimal.Tracing)
			}
			if limit.calls != 1 && (!pivot || failed) || limit.calls != 2 && pivot && !failed {
				t.Fatal("source cancellation reads", pivot, failed, limit.calls)
			}
			if failed {
				if _, e := c.FindSolutionSignatureWithDoubles(signature); e != ErrExactVectorPending {
					t.Fatal("failed parent reuse", e)
				}
			}
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARFloatingSignatureCancellationAfterPivot(t *testing.T) {
	for _, failed := range []bool{false, true} {
		h := newLPPairHarness(t)
		c := lpLARFloatingSignatureFixture(t, h, 3, 7, false)
		limit := &lpExactSolveCancelSequence{values: []bool{false, true}}
		if failed {
			limit.errorAt = 2
		}
		c.settings.SetResourceLimit(limit)
		signature := lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionFreeOfBounds})
		trace, err := c.FindSolutionSignatureWithDoubles(signature)
		if failed && err != lpErrExactSolveCancel || !failed && err != nil {
			t.Fatal(err)
		}
		if limit.calls != 2 || len(trace) != 0 || !c.DPrimal.Tracing || !reflect.DeepEqual(c.DPrimal.Trace, []uint32{1, 0}) || !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{1: PositionAtUpperBound, 2: PositionAtFixed}) {
			t.Fatal("post-pivot cancellation", limit.calls, trace, c.DPrimal.Trace, signature)
		}
		if c.DX[0] != 3 || c.DX[1] != -1.5 || c.DBasis[0] != 1 {
			t.Fatal("completed pivot", c.DX, c.DBasis)
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

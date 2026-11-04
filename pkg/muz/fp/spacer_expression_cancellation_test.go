package fp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestSpacerExpressionCancellationPreservesOwnersAndRetry(t *testing.T) {
	preparationFailures, solveFailures, success := 0, 0, false
	for steps := uint32(0); steps <= 32768 && !success; steps++ {
		t.Run(fmt.Sprint(steps), func(t *testing.T) {
			f := newCoveredExpressionFixture(t, "spacer")
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			// Separate source ingestion from query preparation and solving:
			// cancellation can make HNF return no clause while source flush
			// still completes that input. The companion test covers it.
			must(e.flushPendingRules())
			owner := e.spacer
			proofsBefore, err := m.ProofsEnabled()
			must(err)
			limit := m.ResourceLimit()
			message := messages.MaxResource
			if steps == 0 {
				limit.IncCancel()
				message = messages.Canceled
			} else {
				must(limit.Push(steps))
			}
			active := true
			defer func() {
				if active {
					if steps == 0 {
						limit.DecCancel()
					} else {
						limit.Pop()
					}
				}
			}()
			status, queryErr := e.QueryExpression(f.query)
			if steps == 0 {
				if limit.CancelCount() != 1 {
					t.Fatal("query consumed the caller's cancellation", limit.CancelCount())
				}
				limit.DecCancel()
			} else {
				if limit.CancelCount() != 0 {
					t.Fatal("step exhaustion introduced an independent cancellation", limit.CancelCount())
				}
				limit.Pop()
			}
			active = false
			proofsAfter, err := m.ProofsEnabled()
			must(err)
			if proofsAfter != proofsBefore {
				t.Fatal("cancellation changed caller proof mode")
			}
			if queryErr == nil && status == lbool.True {
				if steps == 0 {
					t.Fatal("explicitly canceled query reported reachable")
				}
				success = true
				return
			}
			if queryErr != nil {
				// context::checkpoint deliberately throws "spacer canceled"
				// for either resource exhaustion or explicit cancellation.
				if !strings.Contains(queryErr.Error(), message) && !errors.Is(queryErr, spacer.ErrCanceled) {
					t.Fatal("unexpected cancellation diagnostic", steps, queryErr)
				}
				if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("canceled query published a result", err)
				}
				if _, err := e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("canceled query published an answer", err)
				}
			} else {
				if status != lbool.Undef {
					t.Fatal("step exhaustion reported an incorrect decided result", status)
				}
				last, err := e.LastResult()
				must(err)
				if last != lbool.Undef {
					t.Fatal("unknown query published a decided result", last)
				}
				if _, err := e.ExpressionAnswer(); err == nil {
					t.Fatal("unknown query published an answer")
				}
			}
			if e.queryClone != nil {
				solveFailures++
				if e.queryClone.spacer != owner {
					t.Fatal("canceled solve lost its context")
				}
				initialized, err := owner.Initialized()
				must(err)
				if !initialized || e.queryClone.spacerRules == nil {
					t.Fatal("canceled-solve fixture did not install rules")
				}
			} else {
				preparationFailures++
				if e.spacer != owner {
					t.Fatal("canceled preparation lost its pre-query owner")
				}
				initialized, err := owner.Initialized()
				must(err)
				if initialized || e.spacerRules != nil {
					t.Fatal("canceled preparation published a program")
				}
			}
			cover, err := e.CoverDelta(2, f.declaration)
			must(err)
			defer func() { must(m.DelRef(&cover)) }()
			handle, err := m.RefGet(cover)
			must(err)
			if handle != f.property {
				t.Fatal("cancellation lost the supplied cover", steps, handle, f.property)
			}
			status, err = e.QueryExpression(f.query)
			must(err)
			if status != lbool.True {
				t.Fatal("query did not recover from cancellation", steps, status)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !success || preparationFailures == 0 || solveFailures == 0 {
		t.Fatal("missing cancellation/success frontiers", preparationFailures, solveFailures, success)
	}
}

// Source flush advances its pending head after mk_rule returns, even when
// resource cancellation makes HNF return normally without producing the fact.
func TestSpacerExpressionCancellationDuringPendingFlush(t *testing.T) {
	observed := false
	for steps := uint32(0); steps < 128 && !observed; steps++ {
		t.Run(fmt.Sprint(steps), func(t *testing.T) {
			f := newCoveredExpressionFixture(t, "spacer")
			e, limit := f.engine, f.manager.ResourceLimit()
			if steps == 0 {
				limit.IncCancel()
			} else {
				fragmentMust(t, limit.Push(steps))
			}
			_, _ = e.QueryExpression(f.query)
			exhausted := !limit.IncOffset(0)
			if steps == 0 {
				limit.DecCancel()
			} else {
				limit.Pop()
			}
			count, err := e.set.Len()
			fragmentMust(t, err)
			if !exhausted || count != 2 || len(e.rawRules) != 3 {
				return
			}
			for _, raw := range e.rawRules {
				if raw.pending {
					return
				}
			}
			// The final input is the only fact for this query's predicate.
			for i := 0; i < count; i++ {
				rule, err := e.set.Rule(i)
				fragmentMust(t, err)
				name, err := rule.NameSymbol()
				fragmentMust(t, err)
				fragmentMust(t, rule.Close())
				if name == e.rawRules[2].name {
					t.Fatal("the allegedly empty input published its fact")
				}
			}
			observed = true
			status, err := e.QueryExpression(f.query)
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("retry recreated a source input completed without a clause", status)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !observed {
		t.Fatal("campaign did not observe pending input completion without a clause")
	}
}

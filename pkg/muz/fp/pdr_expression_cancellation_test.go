package fp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestPDRExpressionCancellationPreservesOwnersAndRetry(t *testing.T) {
	preparationFailures, solveFailures, success := 0, 0, false
	for steps := uint32(0); steps <= 32768 && !success; steps++ {
		t.Run(fmt.Sprint(steps), func(t *testing.T) {
			f := newCoveredPDRExpressionFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			owner := e.pdr
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
				if !strings.Contains(queryErr.Error(), message) {
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
				// pdr::context::get_answer returns true for l_undef. Preserve
				// that source certificate while the unknown result snapshot is
				// live; only an error-producing cancellation suppresses it.
				answer, err := e.ExpressionAnswer()
				must(err)
				answerHandle, err := m.RefGet(answer)
				must(err)
				if !m.IsTrue(answerHandle) {
					t.Fatal("unknown query answer is not source true", answerHandle)
				}
				must(m.DelRef(&answer))
			}
			if e.queryClone != nil {
				solveFailures++
				if e.queryClone.pdr != owner {
					t.Fatal("canceled solve lost its context")
				}
				initialized, err := owner.Initialized()
				must(err)
				if !initialized {
					t.Fatal("canceled-solve fixture did not install rules")
				}
			} else {
				preparationFailures++
				if e.pdr != owner {
					t.Fatal("canceled preparation lost its pre-query owner")
				}
				initialized, err := owner.Initialized()
				must(err)
				if initialized {
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

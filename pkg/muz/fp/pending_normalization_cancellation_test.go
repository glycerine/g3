package fp

// HNF intentionally returns a partial result when its source worklist stops.
// Pending-rule publication must distinguish that result from completion.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPendingNormalizationCancellationPreservesRulesAndRetry(t *testing.T) {
	for _, flush := range []bool{false, true} {
		t.Run(fmt.Sprintf("flush=%t", flush), func(t *testing.T) {
			failures, completed, observedPrefix, observedEmptyAdvance := 0, false, false, false
			for steps := uint32(0); steps < 256 && !completed; steps++ {
				t.Run(fmt.Sprint(steps), func(t *testing.T) {
					f := newExpressionFixture(t, "pdr", false)
					e, m := f.engine, f.manager
					must := func(err error) { t.Helper(); fragmentMust(t, err) }
					if !flush {
						must(e.addRuleEager(f.query, "pending-open-fact"))
					}
					original, revision := e.set, e.revision
					raw := append([]rawRuleFormula(nil), e.rawRules...)
					_, err := original.Len()
					must(err)
					limit := m.ResourceLimit()
					if steps == 0 {
						limit.IncCancel()
					} else {
						must(limit.Push(steps))
					}
					err = e.normalizePendingRules(flush)
					exhausted, message := !limit.IncOffset(0), limit.CancelMessage()
					if steps == 0 {
						limit.DecCancel()
					} else {
						limit.Pop()
					}
					if err != nil {
						failures++
						if !strings.Contains(err.Error(), message) {
							t.Fatal("unexpected normalization failure", err)
						}
						after, countErr := e.set.Len()
						must(countErr)
						if e.set != original || (flush && len(e.rawRules) != len(raw)) {
							t.Fatal("canceled normalization changed the persistent rule owner")
						}
						prefix, pendingSeen := 0, false
						if !flush {
							// Display may promote and swap-remove completed open inputs
							// before a later source conversion fails.
							seen := make(map[ast.ASTRef]bool)
							for _, got := range e.rawRules {
								found := false
								for _, originalRaw := range raw {
									if got.formula == originalRaw.formula && got.name == originalRaw.name {
										found = true
										break
									}
								}
								if !found || seen[got.formula] {
									t.Fatal("display cancellation corrupted the pending raw archive")
								}
								seen[got.formula] = true
							}
						}
						for i := range raw {
							if !flush {
								break
							}
							got := e.rawRules[i]
							if got.formula != raw[i].formula || got.name != raw[i].name || got.bound != raw[i].bound {
								t.Fatal("canceled normalization changed raw input identity", i)
							}
							if got.pending {
								pendingSeen = true
								// A flush removes eager placeholders before source conversion;
								// cancellation before preparation preserves their original ranges.
								if got.first != got.end && (got.first != raw[i].first || got.end != raw[i].end) {
									t.Fatal("canceled flush corrupted an unfinished input range", i)
								}
							} else {
								if pendingSeen {
									t.Fatal("canceled flush published a non-prefix input", i)
								}
								prefix++
							}
						}
						if e.revision < revision+uint64(prefix) {
							t.Fatal("canceled normalization published an unexpected number of inputs", e.revision, revision, prefix)
						}
						var committed []*base.Rule
						published := after
						{
							for _, item := range e.rawRules {
								if item.pending && item.first != item.end {
									published = 0 // preparation did not complete
									break
								}
							}
						}
						for i := 0; i < published; i++ {
							rule, getErr := e.set.Rule(i)
							must(getErr)
							committed = append(committed, rule)
						}
						observedPrefix = observedPrefix || prefix > 0
						observedEmptyAdvance = observedEmptyAdvance || flush && prefix > after
						must(e.normalizePendingRules(flush))
						for i, rule := range committed {
							retried, getErr := e.set.Rule(i)
							must(getErr)
							same, sameErr := rule.Same(retried)
							must(sameErr)
							must(retried.Close())
							must(rule.Close())
							if !same {
								t.Fatal("retry reconstructed a completed prefix rule", i)
							}
						}
					} else {
						completed = true
					}
					// Source advances its head after mk_rule returns, even if HNF
					// returned early with no clauses after the limit was canceled.
					if exhausted {
						return
					}
					status, err := e.QueryExpression(f.query)
					must(err)
					if status != lbool.True {
						t.Fatal("normalization retry lost the reachable fact", status)
					}
				})
				if t.Failed() {
					break
				}
			}
			if failures == 0 || !completed || flush && (!observedPrefix || !observedEmptyAdvance) {
				t.Fatal("campaign missed cancellation, completed prefix, empty completion, or completion", failures, observedPrefix, observedEmptyAdvance, completed)
			}
		})
	}
}

func TestPendingNormalizationHonorsSuspendedLimit(t *testing.T) {
	f := newExpressionFixture(t, "pdr", false)
	e, m := f.engine, f.manager
	limit := m.ResourceLimit()
	limit.IncCancel()
	suspended := control.NewSuspendLimit(limit)
	err := e.flushPendingRules()
	suspended.Close()
	limit.DecCancel()
	fragmentMust(t, err)
	count, err := e.set.Len()
	fragmentMust(t, err)
	if count != 3 || len(e.rawRules) != 3 {
		t.Fatal("suspended normalization changed the original clauses", count, len(e.rawRules))
	}
	for _, raw := range e.rawRules {
		if raw.pending {
			t.Fatal("successful suspended normalization did not advance its input")
		}
	}
	status, err := e.QueryExpression(f.query)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("suspended normalization lost a reachable fact", status)
	}
}

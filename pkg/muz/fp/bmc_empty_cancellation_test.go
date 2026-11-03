package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp bmc::query. The source checkpoints
// cancellation after installing the transformed query and before either
// empty-program return.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestBMCEmptyQueryChecksCancellationBeforeFalseResult(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc"} {
		for _, expression := range []bool{false, true} {
			entry := "predicate"
			if expression {
				entry = "expression"
			}
			t.Run(backend+"/"+entry, func(t *testing.T) {
				f := newDirectFactFixture(t)
				e, m := f.engine, f.manager
				must := func(err error) {
					t.Helper()
					fragmentMust(t, err)
				}
				must(e.SelectEngine(backend))
				queryRef, err := m.NewRef(f.applications["p"])
				must(err)
				defer func() { must(m.DelRef(&queryRef)) }()
				queryExpression, err := m.RefGet(queryRef)
				must(err)
				query := func() (lbool.LBool, error) {
					if expression {
						return e.QueryExpression(queryExpression)
					}
					return e.QueryPredicate(f.declarations["p"])
				}

				limit := m.ResourceLimit()
				limit.IncCancel()
				status, queryErr := query()
				cancelCount := limit.CancelCount()
				limit.DecCancel()
				if cancelCount != 1 {
					t.Fatal("empty BMC query consumed caller cancellation", cancelCount)
				}
				if status != lbool.Undef || queryErr == nil || !strings.Contains(queryErr.Error(), messages.Canceled) {
					t.Fatal("empty BMC query ignored cancellation", status, queryErr)
				}
				if e.queryClone != nil || e.queryClonePredicate != (ast.ASTHandle{}) || len(e.bmcTrace) != 0 || e.bmcProof != (ast.ASTRef{}) {
					t.Fatal("canceled empty BMC query published owners", e.queryClone, e.queryClonePredicate, len(e.bmcTrace), e.bmcProof)
				}
				if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("canceled empty BMC query published a result", err)
				}
				reason, err := e.ReasonUnknown()
				must(err)
				if reason != "canceled" {
					t.Fatal("empty BMC cancellation reason", reason)
				}

				status, queryErr = query()
				must(queryErr)
				if status != lbool.False || e.queryClone == nil || e.queryClone.closed || e.queryClonePredicate.IsNull() {
					t.Fatal("empty BMC query did not recover", status, e.queryClone, e.queryClonePredicate)
				}
				last, err := e.LastResult()
				must(err)
				if last != lbool.False {
					t.Fatal("empty BMC retry result", last)
				}
				reason, err = e.ReasonUnknown()
				must(err)
				if reason != "ok" {
					t.Fatal("empty BMC retry kept cancellation", reason)
				}

				prior := e.queryClone
				status, queryErr = query()
				must(queryErr)
				if status != lbool.False || e.queryClone == nil || e.queryClone == prior || !prior.closed {
					t.Fatal("repeated empty BMC query did not replace its clone owner", status, prior.closed)
				}
			})
		}
	}
}

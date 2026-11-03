package fp

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestBMCExpressionStatisticsIncludeWorkBeforeFailure(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc"} {
		t.Run(backend, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			fragmentMust(t, e.SelectEngine(backend))
			params, err := e.params.NewValue()
			fragmentMust(t, err)
			for _, key := range []string{"xform.inline_eager", "xform.inline_linear", "xform.subsumption_checker", "xform.slice"} {
				name := key
				fragmentMust(t, e.params.SetBoolName(&params, &name, false))
			}
			fragmentMust(t, e.SetQueryParameters(&params))
			fragmentMust(t, e.params.Del(&params))
			fragmentMust(t, e.AddRule(f.applications["p"], "statistics-start"))
			for _, edge := range [][2]string{{"p", "q"}, {"q", "u"}} {
				rule, err := m.MkImplies(f.applications[edge[0]], f.applications[edge[1]])
				fragmentMust(t, err)
				fragmentMust(t, e.AddRule(rule, "statistics-"+edge[1]))
			}
			query := f.applications["u"]
			limit := m.ResourceLimit()
			before := limit.Count()
			status, err := e.QueryExpression(query)
			fragmentMust(t, err)
			steps := limit.Count() - before
			if status != lbool.True || bmcExpressionCheckCount(t, e) == 0 || steps < 2 {
				t.Fatal("calibration did not solve the rule chain", status, steps)
			}
			// Calibrate from actual work rather than hard-coding a particular
			// solver checkpoint. Keep this tiny chain's failure search bounded.
			if steps > 65536 {
				t.Fatal("resource calibration exceeded focused test bound", steps)
			}
			found := false
			for offset := uint64(1); offset < steps && offset <= 128; offset++ {
				allowance := uint32(steps - offset)
				fragmentMust(t, e.ResetStatistics())
				fragmentMust(t, limit.Push(allowance))
				status, err = e.QueryExpression(query)
				limit.Pop()
				checks := bmcExpressionCheckCount(t, e)
				if err == nil {
					continue
				}
				if status != lbool.Undef || !strings.Contains(err.Error(), messages.MaxResource) {
					t.Fatal("unexpected bounded query failure", allowance, status, err)
				}
				if checks == 0 {
					continue
				}
				found = true
				if got := bmcExpressionCheckCount(t, e); got != checks {
					t.Fatal("collecting failed-query statistics changed the count", checks, got)
				}
				if _, err = e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("failed query published a result", err)
				}
				if _, err = e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("failed query published an answer", err)
				}
				break
			}
			if !found {
				t.Fatal("resource campaign did not expose solver work before a query error", steps)
			}
			status, err = e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.True || bmcExpressionCheckCount(t, e) == 0 {
				t.Fatal("unlimited retry did not solve the chain", status)
			}
			answer, err := e.ExpressionAnswer()
			fragmentMust(t, err)
			fragmentMust(t, m.DelRef(&answer))
		})
	}
}

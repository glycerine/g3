package fp

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestFixedpointCanceledQueryPublishesSourceReasonAndRetries(t *testing.T) {
	fixture := newDirectFactFixture(t)
	engine, manager := fixture.engine, fixture.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(engine.AddRule(fixture.applications["p"], "canceled-status-fact"))
	must(engine.SelectEngine("pdr"))

	limit := manager.ResourceLimit()
	limit.IncCancel()
	status, queryErr := engine.QueryPredicate(fixture.declarations["p"])
	limit.DecCancel()
	if status != lbool.Undef || queryErr == nil || !strings.Contains(queryErr.Error(), messages.Canceled) {
		t.Fatal("canceled query result", status, queryErr)
	}
	reason, err := engine.ReasonUnknown()
	must(err)
	if reason != "canceled" {
		t.Fatal("canceled query source status", reason)
	}
	if _, err = engine.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("canceled query published a result", err)
	}

	status, err = engine.QueryPredicate(fixture.declarations["p"])
	must(err)
	if status != lbool.True {
		t.Fatal("fixedpoint query did not retry after cancellation", status)
	}
	reason, err = engine.ReasonUnknown()
	must(err)
	if reason != "ok" {
		t.Fatal("successful retry kept canceled status", reason)
	}
}

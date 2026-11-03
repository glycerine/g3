package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRTransferredContextCloseRemainsIdempotentAfterRetirement(t *testing.T) {
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("context retirement leaked storage", budget.Used())
		}
	})
	f := newPDRLifecycleFixture(t)
	e, manager := f.engine, f.manager
	budget = manager.Budget()
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	predicate, query := f.declarations["p"], f.applications["p"]
	_, err := e.NumLevels(predicate)
	must(err)
	first := e.pdr
	if first == nil {
		t.Fatal("prequery access did not create a context")
	}
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.True || e.queryClone == nil || e.queryClone.pdr != first || e.pdr != nil {
		t.Fatal("expression query did not transfer its prequery context", status)
	}
	status, err = e.QueryExpression(query)
	must(err)
	if status != lbool.True || e.queryClone == nil || e.queryClone.pdr == first {
		t.Fatal("replacement query did not retire the previous context", status)
	}
	second := e.queryClone.pdr
	if _, err = first.Initialized(); !errors.Is(err, pdr.ErrContextOwner) {
		t.Fatal("superseded context still accepts live operations", err)
	}
	before := manager.Stats()
	must(first.Close())
	must(first.Close())
	if after := manager.Stats(); after != before {
		t.Fatal("repeated close changed current query ownership", before, after)
	}
	current, err := e.LastResult()
	must(err)
	if current != lbool.True {
		t.Fatal("closing a retired context invalidated the current result", current)
	}
	must(e.Close())
	if _, err = second.Initialized(); !errors.Is(err, pdr.ErrContextOwner) {
		t.Fatal("engine close left its query context open", err)
	}
	before = manager.Stats()
	must(second.Close())
	must(second.Close())
	if after := manager.Stats(); after != before {
		t.Fatal("repeated context close changed finished engine ownership", before, after)
	}
	var absent *pdr.Context
	if err = absent.Close(); !errors.Is(err, pdr.ErrContextOwner) {
		t.Fatal("nil context close lost its owner guard", err)
	}
	if err = new(pdr.Context).Close(); !errors.Is(err, pdr.ErrContextOwner) {
		t.Fatal("uninitialized context close lost its owner guard", err)
	}
}

package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointObjectLoadSMT2DoesNotExecuteQuery(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.LoadSMT2String(`(declare-rel loaded-query-p ()) (rule (! loaded-query-p :named query-fact)) (query loaded-query-p)`))
	reason, err := f.fp.ReasonUnknown()
	astTestMust(t, err)
	if reason != "ok" {
		t.Fatal("loader changed query status", reason)
	}
	predicates, err := f.fp.command.FixedpointPredicates()
	astTestMust(t, err)
	if len(predicates) != 4 {
		t.Fatal("loaded predicate count", len(predicates))
	}
	status, err := f.fp.Query(predicates[3])
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("explicit query", status)
	}
}

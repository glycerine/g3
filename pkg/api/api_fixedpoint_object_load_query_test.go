package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointObjectLoadSMT2DoesNotExecuteQuery(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.LoadSMT2String(`(rule (! api-object-fp-p :named query-fact)) (query api-object-fp-p)`))
	reason, err := f.fp.ReasonUnknown()
	astTestMust(t, err)
	if reason != "ok" {
		t.Fatal("loader changed query status", reason)
	}
	status, err := f.fp.Query(f.p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("explicit query", status)
	}
}

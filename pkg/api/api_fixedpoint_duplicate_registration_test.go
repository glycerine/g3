package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: src/muz/base/dl_context.cpp context::register_predicate only mutates
// the context when the declaration is not already registered.
func TestAPIFixedpointDuplicateRelationRegistrationIsIdempotent(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.AddFact(f.p, nil))
	status, err := f.fp.Query(f.p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("initial fixedpoint query", status)
	}

	before, err := f.fp.command.FixedpointPredicates()
	astTestMust(t, err)
	answer, err := f.fp.LastAnswer()
	astTestMust(t, err)
	astTestMust(t, f.m.DelRef(&answer))

	astTestMust(t, f.fp.RegisterRelation(f.p))
	after, err := f.fp.command.FixedpointPredicates()
	astTestMust(t, err)
	if len(after) != len(before) {
		t.Fatalf("duplicate registration changed predicate count from %d to %d", len(before), len(after))
	}
	for i := range before {
		if after[i] != before[i] {
			t.Fatalf("duplicate registration changed predicate order at %d", i)
		}
	}
	answer, err = f.fp.LastAnswer()
	astTestMust(t, err)
	answerHandle, err := f.m.RefGet(answer)
	astTestMust(t, err)
	if !f.m.IsTrue(answerHandle) {
		t.Fatal("duplicate registration changed the published answer")
	}
	astTestMust(t, f.m.DelRef(&answer))

	status, err = f.fp.Query(f.p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("query after duplicate registration", status)
	}
}

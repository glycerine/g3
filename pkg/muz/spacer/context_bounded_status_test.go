package spacer

// Source: src/muz/spacer/spacer_context.cpp context::solve_core records
// datalog::BOUNDED only when the configured level budget is exhausted.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestContextBoundedStatusTracksLevelExhaustion(t *testing.T) {
	bounded := newContextAnswerFixture(t, false)
	if status, err := bounded.context.Solve(0, 0); err != nil || status != lbool.Undef {
		t.Fatal("zero-level solve", status, err)
	}
	if status, err := bounded.context.Bounded(); err != nil || !status {
		t.Fatal("bounded status", status, err)
	}
	if err := bounded.context.Reset(); err != nil {
		t.Fatal(err)
	}
	if status, err := bounded.context.Bounded(); err != nil || status {
		t.Fatal("reset bounded status", status, err)
	}

	decided := newContextAnswerFixture(t, false)
	if status, err := decided.context.Solve(0, 1); err != nil || status != lbool.True {
		t.Fatal("one-level solve", status, err)
	}
	if status, err := decided.context.Bounded(); err != nil || status {
		t.Fatal("decided bounded status", status, err)
	}
}

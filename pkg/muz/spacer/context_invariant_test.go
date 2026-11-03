package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSpacerCheckInvariantFramesAndSourceGate(t *testing.T) {
	for _, recursive := range []bool{false, true} {
		f := newContextAnswerFixture(t, recursive)
		c, m := f.context, f.context.manager
		p := c.transformers[f.query]
		if p.context != c {
			t.Fatal("live transformer has no assertion context")
		}
		for _, level := range []uint32{0, 1, InfinityLevel} {
			valid, err := c.CheckPredicateInvariant(level, f.query)
			if err != nil || !valid {
				t.Fatal("empty next frame", valid, err)
			}
		}
		falsity, err := m.MkFalse()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = p.AddLemma(falsity, 1); err != nil {
			t.Fatal(err)
		}
		valid, err := c.CheckPredicateInvariant(0, f.query)
		if err != nil || valid != recursive {
			t.Fatal("initial premises versus next frame", recursive, valid, err)
		}

		truth, err := m.MkTrue()
		if err != nil {
			t.Fatal(err)
		}
		for _, level := range []uint32{0, 1} {
			obligation, err := p.NewProofObligation(nil, level, 0, truth, nil)
			if err != nil {
				t.Fatal(err)
			}
			holds, checkErr := p.expansionInvariantsHold(obligation)
			if err = obligation.Close(); err != nil {
				t.Fatal(err)
			}
			if checkErr != nil || holds != (level == 0 || recursive) {
				t.Fatal("expansion assertion level boundary", level, holds, checkErr)
			}
		}
		valid, err = c.CheckInvariants(0)
		if err != nil || valid != recursive {
			t.Fatal("all-predicate validation", recursive, valid, err)
		}
		// Level two has no property: only next-level formulas, never the entire
		// frame history, are the obligation being checked.
		valid, err = c.CheckPredicateInvariant(1, f.query)
		if err != nil || !valid {
			t.Fatal("wrong frame selected", valid, err)
		}
		if _, err = c.CheckPredicateInvariant(0, f.other); err == nil {
			t.Fatal("unknown predicate accepted")
		}
		enabled, err := diagnostic.BorrowProcessDebugContext().ConditionalAssertionEnabled("spacer")
		if err != nil {
			t.Fatal(err)
		}
		if !enabled {
			if err := p.assertExpansionInvariants(nil); err != nil {
				t.Fatal("disabled source assertion evaluated its operand", err)
			}
		}
	}
}

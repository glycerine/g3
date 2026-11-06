package smtcore

import "testing"

// Current C++ theory_bv.cpp add_fixed_eq increments an unsigned byte before
// testing its old value. The table is heuristic state and is not backtracked.
func TestSMTBVFixedEqualityActivityThrottle(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	_, left := bvFixedSymbolic(t, m, s, bv, "activity-left", 2)
	_, right := bvFixedSymbolic(t, m, s, bv, "activity-right", 2)
	astTestMust(t, s.PushScope())
	for hit := 1; hit <= 512; hit++ {
		clauses := len(s.clauses)
		if hit%2 == 0 {
			astTestMust(t, bv.addFixedEquality(right, left))
		} else {
			astTestMust(t, bv.addFixedEquality(left, right))
		}
		if got, want := bv.statistics.eqDynamic, uint64(hit/256); got != want {
			t.Fatalf("hit %d: emitted %d, want %d", hit, got, want)
		}
		if hit%256 != 0 && len(s.clauses) != clauses {
			t.Fatalf("hit %d emitted premature axioms", hit)
		}
		if hit == 128 {
			astTestMust(t, s.PopScopes(1))
		}
	}
	if bv.eqActivity[smtBVFixedEqualityBucket(left, right)] != 0 {
		t.Fatal("activity byte did not wrap")
	}
	eq, err := bv.MkEqualityLiteral(bv.variables[left].owner, bv.variables[right].owner, true)
	astTestMust(t, err)
	if s.relevancy != nil && !s.relevancy.IsRelevant(s.boolExpr[eq.Var()]) {
		t.Fatal("emitted word equality was not marked relevant")
	}
}

func TestSMTBVFixedEqualityThrottleStillPropagatesEquality(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	l, lv := bvFixedSymbolic(t, m, s, bv, "propagate-left", 2)
	r, rv := bvFixedSymbolic(t, m, s, bv, "propagate-right", 2)
	bvFixedAssign(t, s, bv, lv, 1)
	bvFixedAssign(t, s, bv, rv, 1)
	astTestMust(t, bv.fixedVariable(lv))
	astTestMust(t, bv.fixedVariable(rv))
	if bv.statistics.eqDynamic != 0 || bv.statistics.theoryToCoreEq != 1 {
		t.Fatal("throttle suppressed logical equality or emitted early axioms", bv.statistics)
	}
	astTestMust(t, s.propagateCoreEqualities())
	if l.root != r.root {
		t.Fatal("fixed equality did not propagate while axiom emission was throttled")
	}
}

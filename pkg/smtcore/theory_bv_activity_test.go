package smtcore

import "testing"

// The source in src/smt/theory_bv.cpp emits fixed equality axioms on every
// match. A newer-source activity throttle caused incremental BV queries to
// lose models; the accompanying relevance change also changed string models.
func TestSMTBVFixedEqualityAxiomsAreEagerAcrossScopes(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	_, left := bvFixedSymbolic(t, m, s, bv, "activity-left", 2)
	_, right := bvFixedSymbolic(t, m, s, bv, "activity-right", 2)
	astTestMust(t, s.PushScope())
	for hit := 1; hit <= 16; hit++ {
		clauses := len(s.clauses)
		if hit%2 == 0 {
			astTestMust(t, bv.addFixedEquality(right, left))
		} else {
			astTestMust(t, bv.addFixedEquality(left, right))
		}
		if got, want := bv.statistics.eqDynamic, uint64(hit); got != want {
			t.Fatalf("hit %d: emitted %d, want %d", hit, got, want)
		}
		if len(s.clauses) <= clauses {
			t.Fatalf("hit %d skipped fixed equality axioms", hit)
		}
		if hit == 8 {
			astTestMust(t, s.PopScopes(1))
		}
	}
	eq, err := bv.MkEqualityLiteral(bv.variables[left].owner, bv.variables[right].owner, true)
	astTestMust(t, err)
	if s.relevancy != nil && s.relevancy.IsRelevant(s.boolExpr[eq.Var()]) {
		t.Fatal("fixed equality axioms changed word-equality relevance beyond the source behavior")
	}
}

func TestSMTBVFixedEqualityAxiomsAndCorePropagation(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	l, lv := bvFixedSymbolic(t, m, s, bv, "propagate-left", 2)
	r, rv := bvFixedSymbolic(t, m, s, bv, "propagate-right", 2)
	bvFixedAssign(t, s, bv, lv, 1)
	bvFixedAssign(t, s, bv, rv, 1)
	astTestMust(t, bv.fixedVariable(lv))
	astTestMust(t, bv.fixedVariable(rv))
	if bv.statistics.eqDynamic != 1 || bv.statistics.theoryToCoreEq != 1 {
		t.Fatal("missing fixed equality axioms or logical equality", bv.statistics)
	}
	astTestMust(t, s.propagateCoreEqualities())
	if l.root != r.root {
		t.Fatal("fixed equality did not propagate")
	}
}

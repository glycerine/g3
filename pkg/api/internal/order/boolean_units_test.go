package order

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestBooleanUnitsPropagateFromAssertedRootInOneRound(t *testing.T) {
	const root = 130
	limit, err := control.NewReslimit(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer limit.Close()

	for _, contradictory := range []bool{false, true} {
		// The root implies gate 2 through a long chain. Gate 2 requires
		// variable 1, and in the second problem it also requires its negation.
		k := BooleanProblem{Vars: root, Clauses: []BooleanClause{{-2, 1}}}
		if contradictory {
			k.Clauses = append(k.Clauses, BooleanClause{-2, -1})
		}
		for gate := int32(3); gate <= root; gate++ {
			k.Clauses = append(k.Clauses, BooleanClause{-gate, gate - 1})
		}
		k.Clauses = append(k.Clauses, BooleanClause{root})
		conflict, complete := k.Reduce(1, limit)
		if !complete || conflict != contradictory {
			t.Fatalf("contradictory=%t gave conflict=%t complete=%t", contradictory, conflict, complete)
		}
		if !conflict && (k.Vars != 0 || len(k.Clauses) != 0) {
			t.Fatalf("satisfied chain retained %d variables and %d clauses", k.Vars, len(k.Clauses))
		}
	}
}

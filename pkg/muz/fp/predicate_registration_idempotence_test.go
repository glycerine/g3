package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDuplicatePredicateRegistrationPreservesSolvedState(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	p := f.declarations["p"]
	fragmentMust(t, e.AddFact(p, nil))
	status, err := e.QueryPredicate(p)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("initial predicate query", status)
	}

	revision := e.revision
	state := e.predicates[p]
	order := append([]ast.ASTHandle(nil), e.predicateOrder...)
	fragmentMust(t, e.RegisterPredicate(p))
	if e.revision != revision {
		t.Fatalf("duplicate registration advanced revision from %d to %d", revision, e.revision)
	}
	if e.predicates[p] != state {
		t.Fatal("duplicate registration replaced predicate state")
	}
	if len(e.predicateOrder) != len(order) {
		t.Fatalf("duplicate registration changed predicate count from %d to %d", len(order), len(e.predicateOrder))
	}
	for i := range order {
		if e.predicateOrder[i] != order[i] {
			t.Fatalf("duplicate registration changed predicate order at %d", i)
		}
	}
	if result, err := e.LastResult(); err != nil || result != lbool.True {
		t.Fatal("duplicate registration invalidated result", result, err)
	}
}

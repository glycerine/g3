package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestFixedpointQueryExpressionDoesNotMutateProgram(t *testing.T) {
	f := newQueryExpressionFixture(t)
	pDecl, p := f.predicate("query-isolation-P")
	queryMust(t, f.engine.AddFact(pDecl, nil))
	beforePredicates, err := f.engine.Predicates()
	queryMust(t, err)
	beforeRules, err := f.engine.Rules()
	queryMust(t, err)
	queryMust(t, closeQueryTestRefs(f.manager, beforeRules))
	result, err := f.engine.QueryExpression(p)
	queryMust(t, err)
	if result != lbool.True {
		t.Fatal("isolated query", result)
	}
	afterPredicates, err := f.engine.Predicates()
	queryMust(t, err)
	afterRules, err := f.engine.Rules()
	queryMust(t, err)
	defer func() { queryMust(t, closeQueryTestRefs(f.manager, afterRules)) }()
	if len(afterPredicates) != len(beforePredicates) || len(afterRules) != len(beforeRules) {
		t.Fatal("query mutated program", len(beforePredicates), len(afterPredicates), len(beforeRules), len(afterRules))
	}
}

package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newStandaloneGroundUDocProductFailureFixture(t *testing.T) *publicUDocProductFixture {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("standalone product leaked native budget", budget.Used())
		}
	})
	f := newPublicUDocStandaloneFixture(t, "hashtable", true, true)
	budget = f.m.Budget()
	f.pair(false)
	f.ground(f.finite, "ground-first", f.values[1], f.yes)
	f.ground(f.other, "ground-second", f.values[1], f.yes)
	factory, err := rel.NewUDocRelationFactory(f.m, arena.VectorConfig{
		PageSize: 4, Descriptors: arena.Config{PageSize: 4, Budget: budget},
	})
	fragmentMust(t, err)
	f.e.udocDatalog = &datalogUDocContext{manager: f.m, factory: factory, seeds: make(map[ast.ASTHandle]*rel.TypedUDocRelation)}
	return f.publicUDocProductFixture
}

func TestDatalogUDocStandaloneGroundAllocationFailureRetryAndClose(t *testing.T) {
	checkPublicUDocProductAllocationFailureRetryAndClose(t, newStandaloneGroundUDocProductFailureFixture)
}

func TestDatalogUDocStandaloneGroundCancellationAndHeldAnswer(t *testing.T) {
	f := newStandaloneGroundUDocProductFailureFixture(t)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	c := f.e.udocDatalog
	if len(c.seeds) != 0 || len(c.products.program.groundProducts) != 2 || c.products.program.rules[f.head].pair == nil {
		t.Fatal("standalone failure fixture did not use only ground products and their join")
	}
	held, err := f.e.Answer(f.head)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&held)) }()
	f.m.Limit.Cancel()
	_, cause := f.e.QueryPredicate(f.head)
	f.m.Limit.ResetCancel()
	if cause == nil {
		t.Fatal("standalone product query ignored cancellation")
	}
	stale, err := f.e.Answer(f.head)
	if err == nil || stale != (ast.ASTRef{}) {
		if stale != (ast.ASTRef{}) {
			_ = f.m.DelRef(&stale)
		}
		t.Fatal("cancelled standalone query published a stale answer", err)
	}
	f.checkFormula(held, []ast.ASTHandle{f.values[1], f.yes}, true)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	if c.products.result[f.head] == nil || len(c.products.pending) != 0 {
		t.Fatal("standalone query retry lost actual product storage or pending cleanup")
	}
}

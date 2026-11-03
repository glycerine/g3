package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newMixedGroundUDocProductFailureFixture(t *testing.T) *publicUDocProductFixture {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("mixed ground product leaked native budget", budget.Used())
		}
	})
	f := newPublicUDocProductFixture(t, "hashtable", false, false)
	budget = f.m.Budget()
	prepareUDocProductGroundRules(f)
	f.addRule(false)
	f.addGroundRule(f.doc, "ground-doc", f.values[1])
	f.addMixedGroundRule("ground-mixed", f.values[1], f.yes)
	factory, err := rel.NewUDocRelationFactory(f.m, arena.VectorConfig{
		PageSize: 4, Descriptors: arena.Config{PageSize: 4, Budget: budget},
	})
	fragmentMust(t, err)
	f.e.udocDatalog = &datalogUDocContext{manager: f.m, factory: factory, seeds: make(map[ast.ASTHandle]*rel.TypedUDocRelation)}
	return f
}

func TestDatalogUDocProductsMixedGroundAllocationFailureRetryAndClose(t *testing.T) {
	checkPublicUDocProductAllocationFailureRetryAndClose(t, newMixedGroundUDocProductFailureFixture)
}

func TestDatalogUDocProductsMixedGroundCancellationKeepsPublishedAnswerOwned(t *testing.T) {
	f := newMixedGroundUDocProductFailureFixture(t)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	answer, err := f.e.Answer(f.head)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&answer)) }()
	f.m.Limit.Cancel()
	_, cause := f.e.QueryPredicate(f.head)
	f.m.Limit.ResetCancel()
	if cause == nil {
		t.Fatal("mixed ground query ignored cancellation")
	}
	stale, err := f.e.Answer(f.head)
	if err == nil || stale != (ast.ASTRef{}) {
		if stale != (ast.ASTRef{}) {
			_ = f.m.DelRef(&stale)
		}
		t.Fatal("cancelled mixed ground query published an answer", err)
	}
	f.checkFormula(answer, []ast.ASTHandle{f.values[1], f.yes}, true)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	if f.e.udocDatalog.products.result[f.finite] == nil || len(f.e.udocDatalog.products.pending) != 0 {
		t.Fatal("mixed ground retry lost actual input product or pending cleanup")
	}
}

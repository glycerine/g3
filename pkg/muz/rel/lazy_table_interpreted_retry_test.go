package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazyTableInterpretedForceRetriesTransferredTableAfterFailure(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{0}, Fact{1})
	closeLazyTables(t, table)
	condition := lazyBooleanCondition(t, f.ast, f.boolean)
	if err := table.FilterInterpreted(
		f.relations,
		[]ast.ASTHandle{f.boolean},
		rewriter,
		[]ast.ASTHandle{condition},
		map[uint32]int{0: 0},
		map[uint32]ast.ASTHandle{0: f.boolean},
	); err != nil {
		t.Fatal(err)
	}
	if err := rewriter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Facts(); !errors.Is(err, ErrLazyTable) {
		t.Fatal("closed rewriter did not fail deferred force", err)
	}
	if table.root.evaluated == nil || !table.root.retryForce || table.root.left.evaluated != nil {
		t.Fatal("failed force lost transferred owner or published incomplete result")
	}
	table.root.interpreted.rewriter = tableRelationTheoryRewriter(t, f.ast)
	facts, err := table.Facts()
	if err != nil {
		t.Fatal("retry", err)
	}
	instructionWantFacts(t, facts, Fact{0})
	if table.root.retryForce || !table.Forced() {
		t.Fatal("successful retry did not publish completed force")
	}
	stats := engine.Statistics()
	if stats.InterpretedFilters != 1 {
		t.Fatal("failed attempt counted as completed interpreted filter", stats)
	}
}

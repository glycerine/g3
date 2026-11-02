package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazyTableInterpretedForceReleasesTransferredSourceOwner(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{0}, Fact{1})
	condition := lazyBooleanCondition(t, f.ast, f.boolean)
	baseline := f.ast.Stats().LiveReferences
	filter := func() {
		t.Helper()
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
	}
	filter()
	filter()
	if got := f.ast.Stats().LiveReferences; got != baseline+2 {
		t.Fatalf("nested condition owners = %d, want %d", got, baseline+2)
	}
	if table.root.left == nil || table.root.left.interpreted == nil {
		t.Fatal("fixture did not create nested interpreted source")
	}
	facts, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{0})
	if table.root.left != nil {
		t.Fatal("forced interpreted node kept transferred source edge")
	}
	if got := f.ast.Stats().LiveReferences; got != baseline+1 {
		t.Fatalf("transferred source condition owner remains live: got %d want %d", got, baseline+1)
	}
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
	if got := f.ast.Stats().LiveReferences; got != baseline {
		t.Fatalf("outer condition owner remains live: got %d want %d", got, baseline)
	}
}

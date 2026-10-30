package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func lazyBooleanCondition(t *testing.T, manager *ast.ASTManager, sort ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	variable, err := manager.MkVar(0, sort)
	if err != nil {
		t.Fatal(err)
	}
	condition, err := manager.MkNot(variable)
	if err != nil {
		t.Fatal(err)
	}
	return condition
}

func TestLazyTableInterpretedFilterRetainsConditionAcrossSharedNodes(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	table, err := engine.New(lazySignature(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = table.Add(Fact{0}); err != nil {
		t.Fatal(err)
	}
	if _, err = table.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	condition := lazyBooleanCondition(t, f.ast, f.boolean)
	callerRef, err := f.ast.NewRef(condition)
	if err != nil {
		t.Fatal(err)
	}
	before := f.ast.Stats().LiveReferences
	if err = table.FilterInterpreted(f.relations, []ast.ASTHandle{f.boolean}, rewriter, []ast.ASTHandle{condition}, map[uint32]int{0: 0}, map[uint32]ast.ASTHandle{0: f.boolean}); err != nil {
		t.Fatal(err)
	}
	if got := f.ast.Stats().LiveReferences; got != before+1 {
		t.Fatalf("lazy condition references = %d, want %d", got, before+1)
	}
	if err = f.ast.DelRef(&callerRef); err != nil {
		t.Fatal(err)
	}
	derived, err := table.Project(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
	if got := f.ast.Stats().LiveReferences; got != before {
		t.Fatalf("closing one shared handle released condition too early: live refs %d, want %d", got, before)
	}
	facts, err := derived.Facts()
	if err != nil || len(facts) != 1 || facts[0][0] != 0 {
		t.Fatalf("lazy interpreted filter facts = %v, %v", facts, err)
	}
	if err = derived.Close(); err != nil {
		t.Fatal(err)
	}
	if got := f.ast.Stats().LiveReferences; got != before-1 {
		t.Fatalf("last shared handle retained condition: live refs %d, want %d", got, before-1)
	}
}

func TestLazyTableInterpretedFilterProjectFusion(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	table, err := engine.New(lazySignature(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = table.Add(Fact{0}); err != nil {
		t.Fatal(err)
	}
	if _, err = table.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	condition := lazyBooleanCondition(t, f.ast, f.boolean)
	if err = table.FilterInterpreted(f.relations, []ast.ASTHandle{f.boolean}, rewriter, []ast.ASTHandle{condition}, map[uint32]int{0: 0}, map[uint32]ast.ASTHandle{0: f.boolean}); err != nil {
		t.Fatal(err)
	}
	projected, err := table.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, table, projected)
	facts, err := projected.Facts()
	if err != nil || len(facts) != 1 || len(facts[0]) != 0 {
		t.Fatalf("interpreted filter/project result = %v, %v", facts, err)
	}
	stats := engine.Statistics()
	if stats.InterpretedProjects != 1 || stats.InterpretedFilters != 1 || stats.Projects != 0 {
		t.Fatalf("interpreted filter/project was not fused: %+v", stats)
	}
}

func TestLazyTableEngineCloseReleasesConditionOwners(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	engine := NewLazyTableEngine()
	table, err := engine.New(lazySignature(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	condition := lazyBooleanCondition(t, f.ast, f.boolean)
	callerRef, err := f.ast.NewRef(condition)
	if err != nil {
		t.Fatal(err)
	}
	baseline := f.ast.Stats().LiveReferences
	if err = table.FilterInterpreted(f.relations, []ast.ASTHandle{f.boolean}, rewriter, []ast.ASTHandle{condition}, map[uint32]int{0: 0}, map[uint32]ast.ASTHandle{0: f.boolean}); err != nil {
		t.Fatal(err)
	}
	if err = f.ast.DelRef(&callerRef); err != nil {
		t.Fatal(err)
	}
	if err = engine.Close(); err != nil {
		t.Fatal(err)
	}
	if !table.closed || f.ast.Stats().LiveReferences != baseline-1 {
		t.Fatalf("engine close left lazy owner or AST reference live: closed=%v refs=%d want=%d", table.closed, f.ast.Stats().LiveReferences, baseline-1)
	}
}

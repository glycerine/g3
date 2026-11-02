package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazySparseFiniteInstructionDispatchKeepsTypedDeferredResults(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	left, err := engine.New(lazySignature(t, 3, 3), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{1, 2}, {2, 1}} {
		if _, err = left.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	right, err := engine.New(lazySignature(t, 3), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = right.Add(Fact{2})

	projectedValue, err := finiteTableProject(left, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	projected, ok := projectedValue.(*LazySparseTable)
	if !ok || projected.Forced() {
		t.Fatalf("projection type/deferral = %T/%v", projectedValue, ok && projected.Forced())
	}
	projectedFacts, err := projected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, projectedFacts, Fact{1}, Fact{2})

	joinedValue, err := finiteTableJoinProject(left, right, []int{1}, []int{0}, []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	joined, ok := joinedValue.(*LazySparseTable)
	if !ok || joined.Forced() {
		t.Fatalf("join-project type/deferral = %T/%v", joinedValue, ok && joined.Forced())
	}
	joinedFacts, err := joined.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, joinedFacts, Fact{1})

	selectedValue, err := finiteTableSelectProject(left, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	selected, ok := selectedValue.(*LazySparseTable)
	if !ok || selected.Forced() {
		t.Fatalf("select-project type/deferral = %T/%v", selectedValue, ok && selected.Forced())
	}
	selectedFacts, err := selected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, selectedFacts, Fact{2})
}

func TestCompilerPreflightAcceptsOwnedLazySparseFiniteTable(t *testing.T) {
	f := newTableRelationFixture(t)
	engine := NewLazySparseEngine()
	defer engine.Close()
	table, err := engine.New(lazySignature(t, 2), 0)
	if err != nil {
		t.Fatal(err)
	}
	name, err := f.ast.Symbols.Intern("lazy-sparse-compiled")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.relations.StoreFiniteRelation(predicate, table); err != nil {
		t.Fatal(err)
	}
	if err = f.relations.SupportsCompiledFiniteTables(); err != nil {
		t.Fatal(err)
	}
}

func TestLazySparseUnarySelectProjectProducesInternalNullary(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	source, err := engine.New(lazySignature(t, 2), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = source.Add(Fact{1})
	value, err := finiteTableSelectProject(source, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	selected, ok := value.(*LazySparseTable)
	if !ok {
		t.Fatalf("select-project result %T", value)
	}
	facts, err := selected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{})
	sourceFacts, err := source.Facts()
	if err != nil {
		t.Fatal("select-project consumed shared source", err)
	}
	instructionWantFacts(t, sourceFacts, Fact{1})
}

package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func requireTableFact(t *testing.T, relation *TableRelation, fact Fact, want bool) {
	t.Helper()
	if relation == nil || relation.Table() == nil {
		t.Fatal("closed relation")
	}
	got, err := relation.Table().Contains(fact)
	if err != nil || got != want {
		t.Fatalf("fact %v: got %v want %v: %v", fact, got, want, err)
	}
}

func TestTableRelationJoinProjectRenameAndSelection(t *testing.T) {
	f := newTableRelationFixture(t)
	left, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	right, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.bits, f.boolean})
	closeTableRelations(t, left, right)
	_, _ = left.AddTableFact(Fact{0, 1})
	_, _ = left.AddTableFact(Fact{1, 2})
	_, _ = right.AddTableFact(Fact{1, 1})
	_, _ = right.AddTableFact(Fact{3, 0})
	joined, err := left.Join(right, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, joined)
	if joined.NumRows() != 1 {
		t.Fatal("join rows", joined.NumRows())
	}
	requireTableFact(t, joined, Fact{0, 1, 1, 1}, true)

	joinedProjected, err := left.JoinProject(right, []int{1}, []int{0}, []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, joinedProjected)
	requireTableFact(t, joinedProjected, Fact{0, 1}, true)

	projected, err := left.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, projected)
	requireTableFact(t, projected, Fact{1}, true)
	requireTableFact(t, projected, Fact{2}, true)

	renamed, err := left.RenameCycle([]int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, renamed)
	requireTableFact(t, renamed, Fact{1, 0}, true)
	requireTableFact(t, renamed, Fact{2, 1}, true)

	selected, err := left.SelectEqualAndProject(f.bv(2), 1)
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, selected)
	requireTableFact(t, selected, Fact{1}, true)
	if selected.NumRows() != 1 {
		t.Fatal("selected rows", selected.NumRows())
	}
}

func TestTableRelationUnionDeltaAndFilters(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.boolean}
	target, _ := NewTableRelation(f.relations, sorts)
	source, _ := NewTableRelation(f.relations, sorts)
	delta, _ := NewTableRelation(f.relations, sorts)
	closeTableRelations(t, target, source, delta)
	_, _ = target.AddTableFact(Fact{0, 0})
	_, _ = source.AddTableFact(Fact{0, 0})
	_, _ = source.AddTableFact(Fact{1, 0})
	_, _ = source.AddTableFact(Fact{1, 1})
	if err := target.UnionFrom(source, delta); err != nil {
		t.Fatal(err)
	}
	if target.NumRows() != 3 || delta.NumRows() != 2 {
		t.Fatal("union/delta", target.NumRows(), delta.NumRows())
	}
	if err := target.FilterIdentical([]int{0, 1}); err != nil || target.NumRows() != 2 {
		t.Fatal("identical filter", target.NumRows(), err)
	}
	if err := target.FilterEqual(f.truth, 0); err != nil || target.NumRows() != 1 {
		t.Fatal("equal filter", target.NumRows(), err)
	}
	requireTableFact(t, target, Fact{1, 1}, true)

	wrong, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.bits, f.boolean})
	closeTableRelations(t, wrong)
	if err := target.UnionFrom(wrong, nil); !errors.Is(err, ErrTableRelation) {
		t.Fatal("union sort mismatch", err)
	}
}

func TestTableRelationNegationFilter(t *testing.T) {
	f := newTableRelationFixture(t)
	target, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	negated, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.bits})
	closeTableRelations(t, target, negated)
	for _, fact := range []Fact{{0, 0}, {0, 1}, {1, 1}, {1, 2}} {
		_, _ = target.AddTableFact(fact)
	}
	_, _ = negated.AddTableFact(Fact{1})
	if err := target.FilterByNegation(negated, []int{1}, []int{0}); err != nil {
		t.Fatal(err)
	}
	if target.NumRows() != 2 {
		t.Fatal("negation rows", target.NumRows())
	}
	requireTableFact(t, target, Fact{0, 0}, true)
	requireTableFact(t, target, Fact{1, 2}, true)
	requireTableFact(t, target, Fact{0, 1}, false)
}

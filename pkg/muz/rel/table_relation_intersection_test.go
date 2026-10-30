package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestTableRelationPositiveIntersectionForwarding(t *testing.T) {
	f := newTableRelationFixture(t)
	target, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewTableRelation(f.relations, []ast.ASTHandle{f.bits})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, target, source)
	for _, fact := range []Fact{{0, 1}, {1, 2}, {0, 3}} {
		if _, err = target.AddTableFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = source.AddTableFact(Fact{2}); err != nil {
		t.Fatal(err)
	}
	if err = target.FilterByIntersection(source, []int{1}, []int{0}); err != nil {
		t.Fatal(err)
	}
	requireTableFact(t, target, Fact{1, 2}, true)
	if target.NumRows() != 1 {
		t.Fatal("rows", target.NumRows())
	}
}

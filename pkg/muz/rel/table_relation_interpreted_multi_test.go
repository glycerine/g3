package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestTableRelationInterpretedMultipleConditions(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	relation, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	closeTableRelations(t, relation)
	for _, fact := range []Fact{{0, 0}, {0, 1}, {1, 0}, {1, 1}} {
		_, _ = relation.AddTableFact(fact)
	}
	left, _ := f.ast.MkVar(1, f.boolean)
	right, _ := f.ast.MkVar(0, f.boolean)
	if err := relation.FilterInterpreted(rewriter, []ast.ASTHandle{left, right}, map[uint32]int{1: 0, 0: 1}, map[uint32]ast.ASTHandle{1: f.boolean, 0: f.boolean}); err != nil {
		t.Fatal(err)
	}
	requireTableFact(t, relation, Fact{1, 1}, true)
	if relation.NumRows() != 1 {
		t.Fatal("rows", relation.NumRows())
	}
}

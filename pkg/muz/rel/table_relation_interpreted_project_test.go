package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestTableRelationInterpretedProjectLeavesInput(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	relation, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	closeTableRelations(t, relation)
	_, _ = relation.AddTableFact(Fact{0, 1})
	_, _ = relation.AddTableFact(Fact{1, 2})
	variable, _ := f.ast.MkVar(0, f.boolean)
	projected, err := relation.FilterInterpretedAndProject(rewriter, []ast.ASTHandle{variable}, map[uint32]int{0: 0}, map[uint32]ast.ASTHandle{0: f.boolean}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, projected)
	requireTableFact(t, projected, Fact{2}, true)
	if projected.NumRows() != 1 || relation.NumRows() != 2 {
		t.Fatal("rows", projected.NumRows(), relation.NumRows())
	}
}

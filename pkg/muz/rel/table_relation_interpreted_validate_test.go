package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestTableRelationInterpretedValidation(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	relation, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	closeTableRelations(t, relation)
	variable, _ := f.ast.MkVar(0, f.boolean)
	if err := relation.FilterInterpreted(nil, []ast.ASTHandle{variable}, map[uint32]int{0: 0}, map[uint32]ast.ASTHandle{0: f.boolean}); !errors.Is(err, ErrTableRelation) {
		t.Fatal("nil rewriter", err)
	}
	if err := relation.FilterInterpreted(rewriter, nil, nil, nil); !errors.Is(err, ErrTableRelation) {
		t.Fatal("empty conditions", err)
	}
	if err := relation.FilterInterpreted(rewriter, []ast.ASTHandle{variable}, map[uint32]int{0: 1}, map[uint32]ast.ASTHandle{0: f.boolean}); !errors.Is(err, ErrTableRelation) {
		t.Fatal("bad column", err)
	}
	bitVariable, _ := f.ast.MkVar(0, f.bits)
	if err := relation.FilterInterpreted(rewriter, []ast.ASTHandle{bitVariable}, map[uint32]int{0: 0}, map[uint32]ast.ASTHandle{0: f.boolean}); !errors.Is(err, ErrTableRelation) {
		t.Fatal("non-Boolean", err)
	}
}

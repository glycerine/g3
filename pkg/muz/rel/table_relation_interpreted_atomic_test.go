package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestTableRelationInterpretedFailureIsAtomic(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	relation, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	closeTableRelations(t, relation)
	_, _ = relation.AddTableFact(Fact{0})
	_, _ = relation.AddTableFact(Fact{1})
	name, _ := f.ast.Symbols.Intern("unreduced_bool")
	declaration, err := f.ast.MkFuncDecl(name, nil, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	condition, err := f.ast.MkApp(declaration, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = relation.FilterInterpreted(rewriter, []ast.ASTHandle{condition}, nil, nil); !errors.Is(err, ErrTableRelation) {
		t.Fatal("unreduced condition", err)
	}
	if relation.NumRows() != 2 {
		t.Fatal("partial mutation", relation.NumRows())
	}
}

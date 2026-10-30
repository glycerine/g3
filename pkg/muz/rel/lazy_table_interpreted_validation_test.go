package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazyTableInterpretedFilterValidatesBeforeRetainingAST(t *testing.T) {
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
	condition := lazyBooleanCondition(t, f.ast, f.boolean)
	variable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	before := f.ast.Stats().LiveReferences
	if err = table.FilterInterpreted(f.relations, []ast.ASTHandle{f.bits}, rewriter, []ast.ASTHandle{condition}, nil, nil); !errors.Is(err, ErrLazyTable) {
		t.Fatalf("mismatched finite signature error = %v", err)
	}
	if err = table.FilterInterpreted(f.relations, []ast.ASTHandle{f.boolean}, rewriter, []ast.ASTHandle{variable}, nil, nil); !errors.Is(err, ErrLazyTable) {
		t.Fatalf("non-application condition error = %v", err)
	}
	if err = table.FilterInterpreted(f.relations, []ast.ASTHandle{f.boolean}, rewriter, []ast.ASTHandle{condition}, map[uint32]int{0: 2}, map[uint32]ast.ASTHandle{0: f.boolean}); !errors.Is(err, ErrTableRelation) {
		t.Fatalf("invalid variable binding error = %v", err)
	}
	if got := f.ast.Stats().LiveReferences; got != before {
		t.Fatalf("rejected filters retained AST references: before=%d after=%d", before, got)
	}
}

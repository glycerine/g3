package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestTableRelationFiniteSignatureCapability(t *testing.T) {
	f := newTableRelationFixture(t)
	if !f.relations.CanHandleTableSorts([]ast.ASTHandle{f.boolean, f.bits}) {
		t.Fatal("finite signature rejected")
	}
	if f.relations.CanHandleTableSorts([]ast.ASTHandle{{}}) {
		t.Fatal("invalid sort accepted")
	}
	name, err := f.ast.Symbols.Intern("table_capability")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean, f.bits}, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if !f.relations.CanHandleTablePredicate(predicate) {
		t.Fatal("finite predicate rejected")
	}
	if f.relations.CanHandleTablePredicate(f.truth) {
		t.Fatal("non-declaration accepted")
	}
}

package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRelationManagerHasFactsIgnoresUnmaterializedPredicate(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("has-facts-unmaterialized")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	hasFacts, err := f.relations.HasFacts(predicate)
	if err != nil || hasFacts {
		t.Fatal("unmaterialized predicate facts", hasFacts, err)
	}
	if _, found, err := f.relations.TryGetFiniteRelation(predicate); err != nil || found {
		t.Fatal("fact query materialized predicate", found, err)
	}
}

package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalPredicateSignatureAcceptsArithmeticDomains(t *testing.T) {
	f := newTableRelationFixture(t)
	arithmetic, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := f.ast.MkFamilySort(arithmetic, 0, nil)
	astTestMust(t, err)
	integerSort, err := f.ast.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	name, err := f.ast.Symbols.Intern("external-arithmetic-signature")
	astTestMust(t, err)
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{integerSort, realSort}, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.ast, predicate)

	source, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{})
	astTestMust(t, err)
	relation, err := source.PredicateRelation(predicate)
	astTestMust(t, err)
	sorts, err := relation.Sorts()
	astTestMust(t, err)
	if len(sorts) != 2 || sorts[0] != integerSort || sorts[1] != realSort {
		t.Fatal("external predicate lost arithmetic domain sorts", sorts)
	}
	target, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{})
	astTestMust(t, err)
	astTestMust(t, target.ReplacePredicateFrom(predicate, relation))
	replaced, found, err := target.TryGetPredicateRelation(predicate)
	astTestMust(t, err)
	if !found || replaced == nil {
		t.Fatal("arithmetic predicate replacement was not published")
	}
	replacedSorts, err := replaced.Sorts()
	astTestMust(t, err)
	if len(replacedSorts) != 2 || replacedSorts[0] != integerSort || replacedSorts[1] != realSort {
		t.Fatal("arithmetic replacement changed exact domains", replacedSorts)
	}
	astTestMust(t, target.Close())
	astTestMust(t, source.Close())
}

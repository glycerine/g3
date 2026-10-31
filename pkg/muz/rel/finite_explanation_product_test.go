package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteExplanationProductFactsAndOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	symbol, err := f.ast.Symbols.Intern("finite-explanation")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(symbol, ruleSort)
	astTestMust(t, err)
	ref, err := f.ast.NewRef(explanation)
	astTestMust(t, err)
	product, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth}, explanation))
	astTestMust(t, f.ast.DelRef(&ref))
	got, found, err := product.Explanation([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	if !found || got != explanation {
		t.Fatal("finite explanation", got, found)
	}
	_, found, err = product.Explanation([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	if found {
		t.Fatal("missing finite tuple found")
	}
	if err = product.AddFact([]ast.ASTHandle{f.truth}, explanation); err == nil {
		t.Fatal("duplicate inner explanation accepted")
	}
	if err = product.AddFact(nil, explanation); err == nil {
		t.Fatal("wrong tuple arity accepted")
	}
	count, err := product.Len()
	astTestMust(t, err)
	if count != 1 {
		t.Fatal("product row count", count)
	}
	astTestMust(t, product.Close())
	if _, err = f.ast.GetKind(explanation); err == nil {
		t.Fatal("closed product kept explanation alive")
	}
}

func TestFiniteExplanationProductZeroFiniteColumns(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	symbol, err := f.ast.Symbols.Intern("zero-key-explanation")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(symbol, ruleSort)
	astTestMust(t, err)
	product, err := NewFiniteExplanationProduct(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact(nil, explanation))
	got, found, err := product.Explanation(nil)
	astTestMust(t, err)
	if !found || got != explanation {
		t.Fatal("zero-column explanation", got, found)
	}
}

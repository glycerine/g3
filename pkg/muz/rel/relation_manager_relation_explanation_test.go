package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRelationManagerRelationExplanationStoreExchangeOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	if err != nil {
		t.Fatal(err)
	}
	predicate := defaultFinitePredicate(t, f, "relation-explanation-owner", []ast.ASTHandle{f.boolean, ruleSort})

	original, err := f.relations.GetRelationExplanation(predicate)
	if err != nil {
		t.Fatal(err)
	}
	borrowed, found, err := f.relations.TryGetRelationExplanation(predicate)
	if err != nil || !found || borrowed != original {
		t.Fatal("borrowed relation explanation", borrowed, found, err)
	}
	replacement, err := original.Clone()
	if err != nil {
		t.Fatal(err)
	}
	prior, err := f.relations.ExchangeRelationExplanation(predicate, replacement)
	if err != nil || prior != original {
		t.Fatal("exchange owner", prior, original, err)
	}
	retired, err := f.relations.ExchangeRelationExplanation(predicate, original)
	if err != nil || retired != replacement {
		t.Fatal("restore owner", retired, replacement, err)
	}
	if err = retired.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = retired.FiniteSignature(); err == nil {
		t.Fatal("retired owner remained open")
	}

	wrong, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.bits})
	if err != nil {
		t.Fatal(err)
	}
	if prior, err = f.relations.ExchangeRelationExplanation(predicate, wrong); err == nil || prior != nil {
		t.Fatal("mismatched replacement accepted", prior, err)
	}
	if _, err = wrong.FiniteSignature(); err != nil {
		t.Fatal("failed exchange consumed replacement", err)
	}
	if err = wrong.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRelationManagerRelationExplanationExcludesFiniteAndRestricts(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	if err != nil {
		t.Fatal(err)
	}
	keptPredicate := defaultFinitePredicate(t, f, "relation-explanation-kept", []ast.ASTHandle{f.boolean, ruleSort})
	droppedPredicate := defaultFinitePredicate(t, f, "relation-explanation-dropped", []ast.ASTHandle{f.boolean, ruleSort})
	kept, err := f.relations.GetRelationExplanation(keptPredicate)
	if err != nil {
		t.Fatal(err)
	}
	dropped, err := f.relations.GetRelationExplanation(droppedPredicate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.relations.GetFiniteRelation(keptPredicate); err == nil {
		t.Fatal("finite storage aliased relation explanation")
	}
	if err = f.relations.RestrictPredicates(map[ast.ASTHandle]bool{keptPredicate: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = kept.FiniteSignature(); err != nil {
		t.Fatal("kept product closed", err)
	}
	if _, err = dropped.FiniteSignature(); err == nil {
		t.Fatal("dropped product remained open")
	}
	borrowed, found, err := f.relations.TryGetRelationExplanation(droppedPredicate)
	if err != nil || found || borrowed != nil {
		t.Fatal("dropped product remained stored", borrowed, found, err)
	}
}

func TestRelationManagerRelationExplanationRequiresExplainedDeclaration(t *testing.T) {
	f := newTableRelationFixture(t)
	original := defaultFinitePredicate(t, f, "relation-explanation-original", []ast.ASTHandle{f.boolean})
	product, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.relations.StoreRelationExplanation(original, product); err == nil {
		t.Fatal("original finite declaration accepted as explained storage key")
	}
	if _, err = product.FiniteSignature(); err != nil {
		t.Fatal("failed store consumed product", err)
	}
	if err = product.Close(); err != nil {
		t.Fatal(err)
	}

	ruleSort, err := f.ast.MkDatalogRuleSort()
	if err != nil {
		t.Fatal(err)
	}
	explained := defaultFinitePredicate(t, f, "relation-explanation-proof-arity", []ast.ASTHandle{f.boolean, ruleSort})
	product, err = NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	if err = product.explanation.Close(); err != nil {
		t.Fatal(err)
	}
	product.explanation, err = NewExplanationRelation(f.ast, []ast.ASTHandle{ruleSort, ruleSort})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.relations.StoreRelationExplanation(explained, product); err == nil {
		t.Fatal("two-column proof product accepted for one-column explained declaration")
	}
	if _, err = product.FiniteSignature(); err != nil {
		t.Fatal("failed proof-signature store consumed product", err)
	}
	if err = product.Close(); err != nil {
		t.Fatal(err)
	}
}

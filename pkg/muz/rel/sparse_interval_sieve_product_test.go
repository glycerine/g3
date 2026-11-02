package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseIntervalSieveProductSelectionAndIndependentComponents(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	sorts := []ast.ASTHandle{f.boolean, integer}
	p, err := NewSparseIntervalSieveProduct(relations, sorts, []string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	if got := p.FiniteColumns(); len(got) != 1 || got[0] != 0 {
		t.Fatal("finite component selection", got)
	}
	two, four := numeral("2"), numeral("4")
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.falsehood, two}))
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.truth, four}))
	// product_relation components are independent: their conjunction admits
	// the cross-product tuple, unlike finite_product_relation.
	found, err := p.ContainsFact([]ast.ASTHandle{f.falsehood, four})
	if err != nil || !found {
		t.Fatal("source product component independence", found, err)
	}
	if reversed, reverseErr := NewSparseIntervalSieveProduct(relations, sorts, []string{"interval_relation", "tr_sparse"}, false); reverseErr != nil {
		t.Fatal("ordered source names should select the same concrete pair", reverseErr)
	} else {
		astTestMust(t, reversed.Close())
	}
	if _, err = NewSparseIntervalSieveProduct(relations, sorts, []string{"tr_sparse"}, false); err == nil {
		t.Fatal("single representation selected mixed product")
	}
}

func TestSparseIntervalSieveProductJoinDispatchesComponentEqualities(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	makeProduct := func(value ast.ASTHandle) *SparseIntervalSieveProduct {
		p, err := NewSparseIntervalSieveProduct(relations, []ast.ASTHandle{f.boolean, integer}, []string{"tr_sparse", "interval_relation"}, false)
		astTestMust(t, err)
		astTestMust(t, p.AddFact([]ast.ASTHandle{f.falsehood, value}))
		return p
	}
	left, right := makeProduct(numeral("2")), makeProduct(numeral("3"))
	defer func() { astTestMust(t, left.Close()); astTestMust(t, right.Close()) }()
	finiteOnly, err := JoinSparseIntervalSieveProducts(left, right, []int{0}, []int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, finiteOnly.Close()) }()
	empty, err := finiteOnly.Empty()
	if err != nil || empty {
		t.Fatal("finite equality incorrectly constrained ignored interval columns", empty, err)
	}
	withInterval, err := JoinSparseIntervalSieveProducts(left, right, []int{0, 1}, []int{0, 1})
	astTestMust(t, err)
	defer func() { astTestMust(t, withInterval.Close()) }()
	empty, err = withInterval.Empty()
	if err != nil || !empty {
		t.Fatal("interval component equality not dispatched", empty, err)
	}
}

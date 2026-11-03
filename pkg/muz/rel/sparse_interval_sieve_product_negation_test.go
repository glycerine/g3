package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseIntervalFiniteNegationExactAndRefusal(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	target, err := NewSparseIntervalFiniteOriginal(relations, []ast.ASTHandle{f.boolean}, false)
	astTestMust(t, err)
	negative, err := NewSparseIntervalHashOriginal(relations, []ast.ASTHandle{f.boolean}, false)
	astTestMust(t, err)
	astTestMust(t, target.AddFact([]ast.ASTHandle{f.truth}))
	astTestMust(t, target.AddFact([]ast.ASTHandle{f.falsehood}))
	astTestMust(t, negative.AddFact([]ast.ASTHandle{f.truth}))
	changed, err := target.FilterFiniteNegation(negative, []int{0}, []int{0})
	astTestMust(t, err)
	if !changed {
		t.Fatal("exact finite negation reported no change")
	}
	if found, cause := target.ContainsFact([]ast.ASTHandle{f.truth}); cause != nil || found {
		t.Fatal("matching finite row survived negation", found, cause)
	}
	if found, cause := target.ContainsFact([]ast.ASTHandle{f.falsehood}); cause != nil || !found {
		t.Fatal("nonmatching finite row was removed", found, cause)
	}
	if found, cause := negative.ContainsFact([]ast.ASTHandle{f.truth}); cause != nil || !found {
		t.Fatal("negative input was mutated", found, cause)
	}
	changed, err = target.FilterFiniteNegation(negative, []int{0}, []int{0})
	astTestMust(t, err)
	if changed {
		t.Fatal("stable finite negation reported a change")
	}

	mixed, err := NewSparseIntervalSieveProduct(relations, []ast.ASTHandle{f.boolean, integer}, []string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	raw, err := NewSparseIntervalArithmeticOriginal(relations, []ast.ASTHandle{integer}, false)
	astTestMust(t, err)
	astTestMust(t, raw.AddFact([]ast.ASTHandle{numeral("1")}))
	if _, cause := mixed.FilterFiniteNegation(target, []int{0}, []int{0}); cause == nil {
		t.Fatal("mixed product accepted exact finite negation")
	}
	if _, cause := raw.FilterFiniteNegation(raw, []int{0}, []int{0}); cause == nil {
		t.Fatal("raw interval product accepted exact finite negation")
	}

	nullaryTarget, err := NewSparseIntervalHashOriginal(relations, nil, true)
	astTestMust(t, err)
	nullaryNegative, err := NewSparseIntervalHashOriginal(relations, nil, true)
	astTestMust(t, err)
	changed, err = nullaryTarget.FilterFiniteNegation(nullaryNegative, nil, nil)
	astTestMust(t, err)
	if !changed {
		t.Fatal("full nullary exact negation reported no change")
	}
	if empty, cause := nullaryTarget.Empty(); cause != nil || !empty {
		t.Fatal("full nullary row survived negation", empty, cause)
	}

	astTestMust(t, nullaryNegative.Close())
	astTestMust(t, nullaryTarget.Close())
	astTestMust(t, raw.Close())
	astTestMust(t, mixed.Close())
	astTestMust(t, negative.Close())
	astTestMust(t, target.Close())
}

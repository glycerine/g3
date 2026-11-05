package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFormulaCheckedRelationExplanationFreshPredicateSelection(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewFiniteCheckedFactory("hashtable", "sparse")
	astTestMust(t, err)
	astTestMust(t, f.relations.SetCheckedFiniteFactory(factory))
	t.Cleanup(func() { astTestMust(t, f.relations.SetCheckedFiniteFactory(nil)) })
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)

	plain := defaultFinitePredicate(t, f, "fresh-proof-free", []ast.ASTHandle{f.boolean})
	product, eligible, err := NewFormulaCheckedRelationExplanationForPredicate(f.relations, plain, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	if eligible || product != nil {
		t.Fatal("proof-free magic predicate became an explanation product", product, eligible)
	}

	explained := defaultFinitePredicate(t, f, "fresh-proof-bearing", []ast.ASTHandle{f.boolean, ruleSort})
	product, eligible, err = NewFormulaCheckedRelationExplanationForPredicate(f.relations, explained, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	if !eligible || product == nil {
		t.Fatal("proof-bearing magic predicate lost nested explanation product")
	}
	defer func() { astTestMust(t, product.Close()) }()
	formula, err := product.FormulaChecked()
	astTestMust(t, err)
	primary, checker, pair, err := product.CheckedConcreteKinds()
	astTestMust(t, err)
	if !formula || !pair || primary != "hashtable" || checker != "sparse" {
		t.Fatal("fresh predicate checker composition", formula, primary, checker, pair)
	}
}

func TestFormulaCheckedRelationExplanationSelectedSparseBackings(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)

	for _, test := range []struct {
		name string
		kind string
	}{
		{name: "sparse", kind: "sparse"},
		{name: "lazy-sparse", kind: "lazy_sparse"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.kind == "lazy_sparse" {
				astTestMust(t, f.relations.SetLazySparseFiniteFactory(NewLazySparseFiniteFactory()))
			}
			predicate := defaultFinitePredicate(t, f, "fresh-selected-"+test.name, []ast.ASTHandle{f.boolean, ruleSort})
			product, eligible, err := NewFormulaCheckedRelationExplanationForPredicateWithBacking(
				f.relations, predicate, formulaCheckerConfig(t, f.ast), test.kind,
			)
			astTestMust(t, err)
			if !eligible || product == nil {
				t.Fatal("selected sparse explanation product not installed", eligible, product)
			}
			defer func() { astTestMust(t, product.Close()) }()
			primary, checker, formula, err := product.CheckedConcreteKinds()
			astTestMust(t, err)
			if primary != test.kind || checker != "hashtable" || !formula {
				t.Fatal("selected relation backing changed", primary, checker, formula)
			}
			checked, err := product.FormulaChecked()
			astTestMust(t, err)
			if !checked {
				t.Fatal("selected sparse product lost formula checking")
			}
		})
	}
}

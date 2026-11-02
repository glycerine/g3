package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func sparseIntervalProductTheory(t *testing.T, manager *ast.ASTManager) *astrewriter.TheoryRewriter {
	t.Helper()
	context, err := uparams.NewParamsContext(manager.Symbols, manager.NumeralService(), uparams.ParamsConfig{Budget: manager.Budget()})
	astTestMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(manager.Symbols, uparams.GlobalParamsCatalogConfig{Budget: manager.Budget()})
	astTestMust(t, err)
	globals, err := uparams.NewGlobalParamsService(manager.Symbols, context, uparams.GlobalParamsConfig{Budget: manager.Budget(), Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, globals.InitializeSource())
	theory, err := astrewriter.NewTheoryRewriter(manager, context, globals, nil)
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, theory.Close())
		astTestMust(t, globals.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, context.Close())
	})
	return theory
}

func TestSparseIntervalSieveProductFiltersDispatchByComponent(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	makeFull := func() *SparseIntervalSieveProduct {
		product, makeErr := NewSparseIntervalSieveProduct(relations, []ast.ASTHandle{f.boolean, integer}, []string{"tr_sparse", "interval_relation"}, true)
		astTestMust(t, makeErr)
		t.Cleanup(func() { astTestMust(t, product.Close()) })
		return product
	}

	equal := makeFull()
	astTestMust(t, equal.FilterEqual(0, f.falsehood))
	astTestMust(t, equal.FilterEqual(1, numeral("3")))
	if found, findErr := equal.ContainsFact([]ast.ASTHandle{f.falsehood, numeral("3")}); findErr != nil || !found {
		t.Fatal("component equal filters lost matching tuple", found, findErr)
	}
	if found, findErr := equal.ContainsFact([]ast.ASTHandle{f.truth, numeral("3")}); findErr != nil || found {
		t.Fatal("finite equal filter kept mismatching tuple", found, findErr)
	}
	if found, findErr := equal.ContainsFact([]ast.ASTHandle{f.falsehood, numeral("4")}); findErr != nil || found {
		t.Fatal("interval equal filter kept mismatching tuple", found, findErr)
	}

	cross := makeFull()
	astTestMust(t, cross.FilterIdentical([]int{0, 1}))
	if found, findErr := cross.ContainsFact([]ast.ASTHandle{f.falsehood, numeral("9")}); findErr != nil || !found {
		t.Fatal("cross-component identity filter was not the source identity", found, findErr)
	}

	theory := sparseIntervalProductTheory(t, f.ast)
	finite := makeFull()
	finiteVar, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, finiteVar)
	finiteCondition, err := f.ast.MkEq(finiteVar, f.truth)
	astTestMust(t, err)
	astTestPin(t, f.ast, finiteCondition)
	astTestMust(t, finite.FilterInterpreted(finiteCondition, theory))
	if found, findErr := finite.ContainsFact([]ast.ASTHandle{f.falsehood, numeral("2")}); findErr != nil || found {
		t.Fatal("finite interpreted filter kept false row", found, findErr)
	}
	arithmetic := makeFull()
	integerVar, err := f.ast.MkVar(1, integer)
	astTestMust(t, err)
	astTestPin(t, f.ast, integerVar)
	bound, err := f.ast.MkArithmeticLT(numeral("3"), integerVar)
	astTestMust(t, err)
	astTestPin(t, f.ast, bound)
	astTestMust(t, arithmetic.FilterInterpreted(bound, theory))
	if found, findErr := arithmetic.ContainsFact([]ast.ASTHandle{f.falsehood, numeral("2")}); findErr != nil || found {
		t.Fatal("arithmetic interpreted filter kept out-of-bound tuple", found, findErr)
	}
	if found, findErr := arithmetic.ContainsFact([]ast.ASTHandle{f.falsehood, numeral("4")}); findErr != nil || !found {
		t.Fatal("arithmetic interpreted filter lost matching tuple", found, findErr)
	}

	mixed := makeFull()
	mixedCondition, err := f.ast.MkAnd(finiteCondition, bound)
	astTestMust(t, err)
	astTestPin(t, f.ast, mixedCondition)
	astTestMust(t, mixed.FilterInterpreted(mixedCondition, theory))
	if found, findErr := mixed.ContainsFact([]ast.ASTHandle{f.falsehood, numeral("2")}); findErr != nil || !found {
		t.Fatal("mixed interpreted condition constrained independent components", found, findErr)
	}
}

func TestSparseIntervalSieveProductUnionAndWidenReportChanges(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	makeProduct := func() *SparseIntervalSieveProduct {
		product, makeErr := NewSparseIntervalSieveProduct(relations, []ast.ASTHandle{f.boolean, integer}, []string{"tr_sparse", "interval_relation"}, false)
		astTestMust(t, makeErr)
		t.Cleanup(func() { astTestMust(t, product.Close()) })
		return product
	}

	target, first := makeProduct(), makeProduct()
	astTestMust(t, first.AddFact([]ast.ASTHandle{f.falsehood, numeral("2")}))
	changed, err := target.Union(first, false)
	astTestMust(t, err)
	if !changed {
		t.Fatal("first union did not report a component change")
	}
	second := makeProduct()
	astTestMust(t, second.AddFact([]ast.ASTHandle{f.truth, numeral("4")}))
	changed, err = target.Union(second, false)
	astTestMust(t, err)
	if !changed {
		t.Fatal("second component union did not report a change")
	}
	for _, values := range [][]ast.ASTHandle{{f.falsehood, numeral("4")}, {f.truth, numeral("2")}} {
		if found, findErr := target.ContainsFact(values); findErr != nil || !found {
			t.Fatal("componentwise union lost independent cross-product tuple", found, findErr)
		}
	}
	changed, err = target.Union(second, false)
	astTestMust(t, err)
	if changed {
		t.Fatal("idempotent union reported a change")
	}
	astTestMust(t, target.FilterEqual(0, f.falsehood))
	if found, findErr := second.ContainsFact([]ast.ASTHandle{f.truth, numeral("4")}); findErr != nil || !found {
		t.Fatal("target filtering mutated union source", found, findErr)
	}

	widenSource := makeProduct()
	astTestMust(t, widenSource.AddFact([]ast.ASTHandle{f.falsehood, numeral("10")}))
	changed, err = target.Union(widenSource, true)
	astTestMust(t, err)
	if !changed {
		t.Fatal("interval widening did not report a change")
	}
	if found, findErr := target.ContainsFact([]ast.ASTHandle{f.falsehood, numeral("20")}); findErr != nil || !found {
		t.Fatal("widened interval omitted upper values", found, findErr)
	}

	reversed, err := NewSparseIntervalSieveProduct(relations, []ast.ASTHandle{integer, f.boolean}, []string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, reversed.Close()) })
	if _, err = target.Union(reversed, false); err == nil {
		t.Fatal("union accepted a different component layout")
	}
}

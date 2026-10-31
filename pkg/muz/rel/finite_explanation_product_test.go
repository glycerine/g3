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

func TestFiniteExplanationProductUnionAndDelta(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	makeExplanation := func(name string) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		h, e := f.ast.MkConst(sym, ruleSort)
		astTestMust(t, e)
		return h
	}
	old := makeExplanation("finite-union-old")
	fresh := makeExplanation("finite-union-new")
	prior := makeExplanation("finite-delta-prior")
	target, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	delta, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, delta.Close()) }()
	astTestMust(t, target.AddFact([]ast.ASTHandle{f.truth}, old))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth}, fresh))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood}, fresh))
	astTestMust(t, delta.AddFact([]ast.ASTHandle{f.truth}, prior))
	astTestMust(t, target.Union(source, delta))
	for _, tc := range []struct {
		p         *FiniteExplanationProduct
		key, want ast.ASTHandle
	}{{target, f.truth, old}, {target, f.falsehood, fresh}, {delta, f.truth, prior}, {delta, f.falsehood, fresh}} {
		got, found, e := tc.p.Explanation([]ast.ASTHandle{tc.key})
		astTestMust(t, e)
		if !found || got != tc.want {
			t.Fatal("union explanation", got, found, tc.want)
		}
	}
	astTestMust(t, target.Union(source, nil))
	count, err := target.Len()
	astTestMust(t, err)
	if count != 2 {
		t.Fatal("repeated union count", count)
	}
	if err = target.Union(source, target); err == nil {
		t.Fatal("accepted aliased delta")
	}
}

func TestFiniteExplanationProductProjectCollisions(t *testing.T) {
	f := newTableRelationFixture(t)
	sort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	makeExplanation := func(name string) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		h, e := f.ast.MkConst(sym, sort)
		astTestMust(t, e)
		return h
	}
	first := makeExplanation("finite-project-first")
	second := makeExplanation("finite-project-second")
	source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth, f.falsehood}, first))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth, f.truth}, second))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood, f.truth}, second))
	projected, err := source.Project([]int{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	got, found, err := projected.Explanation([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	if !found || got != first {
		t.Fatal("colliding explanation", got, found, first)
	}
	got, found, err = projected.Explanation([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	if !found || got != second {
		t.Fatal("distinct explanation", got, found, second)
	}
	count, err := projected.Len()
	astTestMust(t, err)
	if count != 2 {
		t.Fatal("projected count", count)
	}
	if _, err = source.Project([]int{2}); err == nil {
		t.Fatal("removed explanation column accepted")
	}
}

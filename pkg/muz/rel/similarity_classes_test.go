package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"testing"
)

func TestSimilarityClassesThresholdDuplicatesAndOutputs(t *testing.T) {
	m := astTestManager(t)
	family, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	integer, e := m.MkFamilySort(family, 1, nil)
	astTestMust(t, e)
	boolean, e := m.MkBoolSort()
	astTestMust(t, e)
	decl, e := m.MkFuncDecl(astTestSymbol(t, m, "group-p"), []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, decl)
	rm, e := base.NewRuleManager(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, rm.Close()) })
	astTestMust(t, rm.RegisterPredicate(decl))
	set, e := base.NewRuleSet(rm)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, set.Close()) })
	rule := func(value string) *base.Rule {
		n, e := m.ArithmeticIntegerNumeralText(value)
		astTestMust(t, e)
		astTestPin(t, m, n)
		h, e := m.MkApp(decl, []ast.ASTHandle{n})
		astTestMust(t, e)
		astTestPin(t, m, h)
		r, e := rm.NewRule(h, nil, nil, "")
		astTestMust(t, e)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		astTestMust(t, set.Add(r))
		return r
	}
	a, b, c := rule("1"), rule("2"), rule("3")
	rules := []*base.Rule{c, a, b, a}
	groups, duplicates, e := similarityClasses(set, rules, 2)
	astTestMust(t, e)
	if !duplicates || len(groups) != 1 || !groups[0].merge || len(groups[0].rules) != 3 || len(groups[0].constants) != 1 {
		t.Fatalf("eligible class: %+v duplicate %v", groups, duplicates)
	}
	// The threshold is strict and duplicate removal happens before counting.
	groups, duplicates, e = similarityClasses(set, rules, 3)
	astTestMust(t, e)
	if !duplicates || len(groups) != 1 || groups[0].merge || len(groups[0].rules) != 3 {
		t.Fatalf("threshold: %+v", groups)
	}
	astTestMust(t, set.SetOutputPredicate(decl))
	groups, duplicates, e = similarityClasses(set, rules, 2)
	astTestMust(t, e)
	if !duplicates || len(groups) != 1 || groups[0].merge {
		t.Fatalf("output predicate merged: %+v", groups)
	}
	groups, duplicates, e = similarityClasses(set, nil, 2)
	astTestMust(t, e)
	if len(groups) != 0 || duplicates {
		t.Fatal("empty source", groups, duplicates)
	}
	count, e := set.Len()
	astTestMust(t, e)
	if count != 3 {
		t.Fatal("planning mutated source", count)
	}
}

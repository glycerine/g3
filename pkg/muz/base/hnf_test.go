package base_test

// Source: src/muz/base/hnf.cpp no-proof Horn normalization worklist,
// disjunctive splitting, fresh predicate definitions, and memoization.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornNormalizerPreservesAlreadyHornUniversalPrefix(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	implication, err := f.manager.MkImplies(x, f.app["P"])
	ruleMust(t, err)
	innerName, err := f.manager.Symbols.Intern("hnf-inner")
	ruleMust(t, err)
	inner, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{innerName}, Body: implication,
	})
	ruleMust(t, err)
	outerName, err := f.manager.Symbols.Intern("hnf-outer")
	ruleMust(t, err)
	formula, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{outerName}, Body: inner,
	})
	ruleMust(t, err)
	formulaRef, err := f.manager.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&formulaRef)) }()

	normalizer, err := base.NewHornNormalizer(f.manager, "prefix")
	ruleMust(t, err)
	normalized, err := normalizer.Normalize(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, normalized.Close()) }()
	if len(normalized.Formulas) != 1 || len(normalized.FreshPredicates) != 0 {
		t.Fatal("universal-prefix normalization shape", len(normalized.Formulas), len(normalized.FreshPredicates))
	}
	result, err := f.manager.RefGet(normalized.Formulas[0])
	ruleMust(t, err)
	if result != formula {
		t.Fatal("already-Horn prefix was reconstructed instead of returned unchanged")
	}
	body, err := f.manager.GetQuantifierBody(result)
	ruleMust(t, err)
	if body != inner {
		t.Fatal("already-Horn nested universal prefix changed")
	}
	body, err = f.manager.GetQuantifierBody(body)
	ruleMust(t, err)
	antecedent, err := f.manager.GetArg(body, 0)
	ruleMust(t, err)
	if index, err := f.manager.GetVarIndex(antecedent); err != nil || index != 0 {
		t.Fatal("merged prefix changed inner de Bruijn index", index, err)
	}
}

func TestHornNormalizerKeepsVariableAntecedent(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	implication, err := f.manager.MkImplies(x, f.app["P"])
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("hnf-x")
	ruleMust(t, err)
	formula, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{name}, Body: implication,
	})
	ruleMust(t, err)
	formulaRef, err := f.manager.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&formulaRef)) }()

	ruleMust(t, f.rules.AddFormula(set, formula, "variable"))
	if count, err := set.Len(); err != nil || count != 1 {
		t.Fatal("variable-antecedent rule count", count, err)
	}
	rule, err := set.Rule(0)
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	if size, err := rule.TailSize(); err != nil || size != 1 {
		t.Fatal("variable-antecedent tail size", size, err)
	}
	tail, negated, err := rule.Tail(0)
	ruleMust(t, err)
	if operator, err := f.manager.GetDeclKind(tail); err != nil || operator != ast.ASTOpIff || negated {
		t.Fatal("variable antecedent was not retained as Boolean equality", operator, negated, err)
	}
}

func TestRuleManagerAddFormulaRemovesNestedLabels(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	labelName, err := f.manager.Symbols.Intern("source-label")
	ruleMust(t, err)
	inner, err := f.manager.MkLabel(true, []symbol.Symbol{labelName}, f.app["P"])
	ruleMust(t, err)
	outer, err := f.manager.MkLabel(false, []symbol.Symbol{labelName}, inner)
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(outer, f.app["Q"])
	ruleMust(t, err)
	formulaReference, err := f.manager.NewRef(formula)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, f.manager.DelRef(&formulaReference)) })
	ruleMust(t, f.rules.AddFormula(set, formula, "labelled-rule"))
	if count, getErr := set.Len(); getErr != nil || count != 1 {
		t.Fatalf("labelled formula generated %d rules, %v", count, getErr)
	}
	rule, err := set.Rule(0)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rule.Close()) })
	tail, negated, err := rule.Tail(0)
	ruleMust(t, err)
	if tail != f.app["P"] || negated {
		t.Fatalf("label removal produced tail %v (negated %v), want P", tail, negated)
	}
}

func TestHornNormalizerSplitsSingletonDisjunction(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	disjunction, err := f.manager.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(disjunction, f.app["R"])
	ruleMust(t, err)
	formulaRef, err := f.manager.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&formulaRef)) }()

	ruleMust(t, f.rules.AddFormula(set, formula, "split"))
	if count, err := set.Len(); err != nil || count != 2 {
		t.Fatal("split rule count", count, err)
	}
	wantTails := map[ast.ASTHandle]bool{f.decl["P"]: false, f.decl["Q"]: false}
	for i := 0; i < 2; i++ {
		rule, e := set.Rule(i)
		ruleMust(t, e)
		head, e := rule.Head()
		ruleMust(t, e)
		headDecl, e := f.manager.GetDecl(head)
		ruleMust(t, e)
		if headDecl != f.decl["R"] {
			t.Fatal("split rule head", headDecl, f.decl["R"])
		}
		if size, e := rule.TailSize(); e != nil || size != 1 {
			t.Fatal("split tail size", size, e)
		}
		tail, negated, e := rule.Tail(0)
		ruleMust(t, e)
		tailDecl, e := f.manager.GetDecl(tail)
		ruleMust(t, e)
		if _, found := wantTails[tailDecl]; !found || wantTails[tailDecl] || negated {
			t.Fatal("split tail", tailDecl, negated)
		}
		wantTails[tailDecl] = true
		ruleMust(t, rule.Close())
	}
	for declaration, seen := range wantTails {
		if !seen {
			t.Fatal("missing split tail", declaration)
		}
	}
	valid, err := set.Finalize()
	ruleMust(t, err)
	if !valid {
		t.Fatal("split program rejected")
	}
}

func TestHornNormalizerStopsWorklistWhenManagerIsCanceled(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	antecedent, err := f.manager.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(antecedent, f.app["Q"])
	ruleMust(t, err)
	reference, err := f.manager.NewRef(formula)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, f.manager.DelRef(&reference)) })
	normalizer, err := base.NewHornNormalizer(f.manager, "cancel")
	ruleMust(t, err)
	f.manager.Limit.Cancel()
	result, err := normalizer.Normalize(formula)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, result.Close()) })
	if len(result.Formulas) != 0 {
		t.Fatalf("canceled HNF worklist published %d formulas, want none", len(result.Formulas))
	}
}

func TestHornNormalizerIntroducesAndMemoizesFreshPredicate(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R", "S")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	disjunction, err := f.manager.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	body, err := f.manager.MkAndArgs([]ast.ASTHandle{disjunction, disjunction, f.app["S"]})
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(body, f.app["R"])
	ruleMust(t, err)
	formulaRef, err := f.manager.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&formulaRef)) }()

	ruleMust(t, f.rules.AddFormula(set, formula, "memo"))
	if count, err := set.Len(); err != nil || count != 3 {
		t.Fatal("fresh rule count", count, err)
	}
	var fresh ast.ASTHandle
	definitionTails := make(map[ast.ASTHandle]bool)
	mainFreshTails, mainSTails := 0, 0
	for i := 0; i < 3; i++ {
		rule, e := set.Rule(i)
		ruleMust(t, e)
		head, e := rule.Head()
		ruleMust(t, e)
		headDecl, e := f.manager.GetDecl(head)
		ruleMust(t, e)
		size, e := rule.TailSize()
		ruleMust(t, e)
		if headDecl == f.decl["R"] {
			if size != 3 {
				t.Fatal("main rule tail size", size)
			}
			for j := 0; j < size; j++ {
				tail, negated, x := rule.Tail(j)
				ruleMust(t, x)
				if negated {
					t.Fatal("main rule acquired a negative tail")
				}
				declaration, x := f.manager.GetDecl(tail)
				ruleMust(t, x)
				if declaration == f.decl["S"] {
					mainSTails++
				} else {
					if fresh.IsNull() {
						fresh = declaration
					} else if declaration != fresh {
						t.Fatal("memoized disjunction used distinct predicates", declaration, fresh)
					}
					mainFreshTails++
				}
			}
		} else {
			if fresh.IsNull() {
				fresh = headDecl
			} else if headDecl != fresh {
				t.Fatal("fresh definition head", headDecl, fresh)
			}
			if size != 1 {
				t.Fatal("fresh definition tail size", size)
			}
			tail, negated, x := rule.Tail(0)
			ruleMust(t, x)
			declaration, x := f.manager.GetDecl(tail)
			ruleMust(t, x)
			if negated || (declaration != f.decl["P"] && declaration != f.decl["Q"]) {
				t.Fatal("fresh definition tail", declaration, negated)
			}
			definitionTails[declaration] = true
		}
		ruleMust(t, rule.Close())
	}
	if fresh.IsNull() || mainFreshTails != 2 || mainSTails != 1 || len(definitionTails) != 2 {
		t.Fatal("fresh predicate layout", fresh, mainFreshTails, mainSTails, definitionTails)
	}
	valid, err := set.Finalize()
	ruleMust(t, err)
	if !valid {
		t.Fatal("fresh-definition program rejected")
	}
}

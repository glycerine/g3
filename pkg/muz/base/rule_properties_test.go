package base_test

// Source: src/muz/base/rule_properties.h; src/muz/base/rule_properties.cpp.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type rulePropertiesFixture struct {
	t       *testing.T
	context *cmd_context.Context
	manager *ast.ASTManager
	rules   *base.RuleManager
	p, q    ast.ASTHandle
}

func newRulePropertiesFixture(t *testing.T) *rulePropertiesFixture {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	manager, err := context.Manager()
	ruleMust(t, err)
	rules, err := base.NewRuleManager(manager)
	ruleMust(t, err)
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	apps := make([]ast.ASTHandle, 2)
	for i, name := range []string{"properties-P", "properties-Q"} {
		symbol, e := manager.Symbols.Intern(name)
		ruleMust(t, e)
		declaration, e := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		ruleMust(t, rules.RegisterPredicate(declaration))
		apps[i], e = manager.MkApp(declaration, nil)
		ruleMust(t, e)
	}
	refs := make([]ast.ASTRef, len(apps))
	for i, application := range apps {
		refs[i], err = manager.NewRef(application)
		ruleMust(t, err)
	}
	f := &rulePropertiesFixture{t: t, context: context, manager: manager, rules: rules, p: apps[0], q: apps[1]}
	t.Cleanup(func() {
		for i := len(refs) - 1; i >= 0; i-- {
			ruleMust(t, manager.DelRef(&refs[i]))
		}
		ruleMust(t, rules.Close())
		ruleMust(t, context.Close())
	})
	return f
}

func (f *rulePropertiesFixture) set(name string, interpreted ast.ASTHandle, negatedPredicate bool) *base.RuleSet {
	f.t.Helper()
	set, err := base.NewRuleSet(f.rules)
	ruleMust(f.t, err)
	tail := []ast.ASTHandle{interpreted}
	negated := []bool{false}
	if negatedPredicate {
		tail = []ast.ASTHandle{f.q}
		negated = []bool{true}
	}
	rule, err := f.rules.NewRule(f.p, tail, negated, name)
	ruleMust(f.t, err)
	ruleMust(f.t, set.Add(rule))
	ruleMust(f.t, rule.Close())
	return set
}

func propertyKind(t *testing.T, err error, want base.RulePropertyKind) {
	t.Helper()
	var property *base.RulePropertyError
	if !errors.As(err, &property) || property.Kind != want {
		t.Fatalf("property error = %v, want kind %d", err, want)
	}
}

func TestRulePropertiesExistentialTraversalDistinctionsAndReuse(t *testing.T) {
	f := newRulePropertiesFixture(t)
	truth, err := f.manager.MkTrue()
	ruleMust(t, err)
	and, err := f.manager.MkAnd(f.q, truth)
	ruleMust(t, err)
	antecedent, err := f.manager.MkImplies(f.q, truth)
	ruleMust(t, err)
	consequent, err := f.manager.MkImplies(truth, f.q)
	ruleMust(t, err)
	eqLeft, err := f.manager.MkEq(truth, f.q)
	ruleMust(t, err)
	eqRight, err := f.manager.MkEq(f.q, truth)
	ruleMust(t, err)
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("properties-x")
	ruleMust(t, err)
	quantifier, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{name}, Body: f.q})
	ruleMust(t, err)
	quantified, err := f.manager.MkNot(quantifier)
	ruleMust(t, err)

	properties, err := base.NewRuleProperties(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, properties.Close()) }()
	for _, test := range []struct {
		name             string
		formula          ast.ASTHandle
		existentialFails bool
		quantifierFails  bool
	}{
		{"top-level-and", and, false, false},
		{"implication-antecedent", antecedent, true, false},
		{"implication-consequent", consequent, false, false},
		{"equality-true-left", eqLeft, false, false},
		{"equality-true-right", eqRight, false, false},
		{"quantifier-body", quantified, true, true},
	} {
		set := f.set(test.name, test.formula, false)
		ruleMust(t, properties.Collect(set))
		propertyKind(t, properties.CheckNestedFree(), base.RulePropertyNested)
		existentialErr := properties.CheckExistentialTail()
		if test.existentialFails {
			propertyKind(t, existentialErr, base.RulePropertyExistentialTail)
		} else if existentialErr != nil {
			t.Fatalf("%s existential traversal: %v", test.name, existentialErr)
		}
		quantifierErr := properties.CheckQuantifierFree()
		if test.quantifierFails {
			propertyKind(t, quantifierErr, base.RulePropertyQuantifier)
		} else if quantifierErr != nil {
			t.Fatalf("%s quantifier collection: %v", test.name, quantifierErr)
		}
		ruleMust(t, set.Close())
	}

	clean, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	rule, err := f.rules.NewRule(f.p, []ast.ASTHandle{f.q}, nil, "clean")
	ruleMust(t, err)
	ruleMust(t, clean.Add(rule))
	ruleMust(t, rule.Close())
	ruleMust(t, properties.Collect(clean))
	if err = properties.CheckNestedFree(); err != nil {
		t.Fatal("collection state was not reset", err)
	}
	if err = properties.CheckExistentialTail(); err != nil {
		t.Fatal("clean predicate tail rejected", err)
	}
	ruleMust(t, clean.Close())
}

func TestRulePropertiesFirstRuleOrderNegativeAndOwnership(t *testing.T) {
	f := newRulePropertiesFixture(t)
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	makeUninterpreted := func(name string) ast.ASTHandle {
		symbol, e := f.manager.Symbols.Intern(name)
		ruleMust(t, e)
		declaration, e := f.manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		application, e := f.manager.MkApp(declaration, nil)
		ruleMust(t, e)
		return application
	}
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	for _, row := range []struct {
		name string
		body ast.ASTHandle
	}{{"first-rule", makeUninterpreted("first-uf")}, {"second-rule", makeUninterpreted("second-uf")}} {
		rule, e := f.rules.NewRule(f.p, []ast.ASTHandle{row.body}, nil, row.name)
		ruleMust(t, e)
		ruleMust(t, set.Add(rule))
		ruleMust(t, rule.Close())
	}
	properties, err := base.NewRuleProperties(f.rules)
	ruleMust(t, err)
	ruleMust(t, properties.Collect(set))
	var property *base.RulePropertyError
	if err = properties.CheckUninterpretedFree(); !errors.As(err, &property) || property.RuleName != "first-rule" || property.Symbol != "first-uf" {
		t.Fatal("first source rule was not reported", property, err)
	}
	copyOwner := *properties
	if err = copyOwner.CheckUninterpretedFree(); !errors.Is(err, base.ErrRuleProperties) {
		t.Fatal("copied collector accepted", err)
	}
	ruleMust(t, properties.Reset())
	if count, e := set.Len(); e != nil || count != 2 {
		t.Fatal("collector adopted the rule set", count, e)
	}
	ruleMust(t, set.Close())

	negative := f.set("negative-rule", f.q, true)
	ruleMust(t, properties.Collect(negative))
	propertyKind(t, properties.CheckNegativePredicates(), base.RulePropertyNegativePredicate)
	ruleMust(t, negative.Close())

	arithmetic, err := f.manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := f.manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("infinite-head")
	ruleMust(t, err)
	declaration, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, f.rules.RegisterPredicate(declaration))
	variable, err := f.manager.MkVar(0, integer)
	ruleMust(t, err)
	head, err := f.manager.MkApp(declaration, []ast.ASTHandle{variable})
	ruleMust(t, err)
	infinite, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	rule, err := f.rules.NewRule(head, nil, nil, "infinite-rule")
	ruleMust(t, err)
	ruleMust(t, infinite.Add(rule))
	ruleMust(t, rule.Close())
	ruleMust(t, properties.Collect(infinite))
	propertyKind(t, properties.CheckInfiniteSorts(), base.RulePropertyInfiniteSort)
	ruleMust(t, infinite.Close())

	foreignRules, err := base.NewRuleManager(f.manager)
	ruleMust(t, err)
	foreign, err := base.NewRuleSet(foreignRules)
	ruleMust(t, err)
	if err = properties.Collect(foreign); !errors.Is(err, base.ErrRuleProperties) {
		t.Fatal("foreign rule set accepted", err)
	}
	ruleMust(t, foreign.Close())
	ruleMust(t, foreignRules.Close())
	ruleMust(t, properties.Close())
	if err = properties.CheckNegativePredicates(); !errors.Is(err, base.ErrRuleProperties) {
		t.Fatal("closed collector accepted", err)
	}
}

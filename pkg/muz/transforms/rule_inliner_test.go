package transforms_test

// Source: src/muz/transforms/dl_mk_rule_inliner.cpp planning, rule resolution,
// protected outputs, and recursive-predicate exclusion.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestRuleInlinerHonorsDisabledModes(t *testing.T) {
	f := newConeFixture(t, "P", "Goal")
	f.add(t, f.applications["P"], "fact-P")
	f.edge(t, "P", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("disabled inliner fixture rejected")
	}
	plugin := transforms.NewRuleInlinerPluginWithModes(f.manager, f.rules, []ast.ASTHandle{f.declarations["Goal"]}, false, false, 0)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	replacement, err := plugin.Transform(f.set)
	coneMust(t, err)
	if replacement != nil {
		coneMust(t, replacement.Close())
		t.Fatal("disabled inliner changed rules")
	}
}

func TestRuleInlinerRefusesMixedQuantifiedProgram(t *testing.T) {
	f := newConeFixture(t, "P", "Goal", "Quantified")
	f.add(t, f.applications["P"], "fact-P")
	f.edge(t, "P", "Goal")
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	bound, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	quantified, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{boolean},
		Names:  []usymbol.Symbol{usymbol.Numerical(0)},
		Body:   bound,
	})
	coneMust(t, err)
	truth, err := f.manager.MkTrue()
	coneMust(t, err)
	quantifiedTail, err := f.manager.MkIff(quantified, truth)
	coneMust(t, err)
	quantifiedRule, err := f.rules.NewRule(f.applications["Quantified"], []ast.ASTHandle{quantifiedTail}, nil, "quantified")
	coneMust(t, err)
	coneMust(t, f.set.Add(quantifiedRule))
	coneMust(t, quantifiedRule.Close())
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("quantified inliner fixture rejected")
	}
	plugin := transforms.NewRuleInlinerPlugin(f.manager, f.rules, []ast.ASTHandle{f.declarations["Goal"]}, 0)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	replacement, err := plugin.Transform(f.set)
	coneMust(t, err)
	if replacement != nil {
		coneMust(t, replacement.Close())
		t.Fatal("inliner transformed a mixed quantified program")
	}
}

func TestRuleInlinerBreaksMutualCycleAtOnePredicate(t *testing.T) {
	f := newConeFixture(t, "A", "B", "Goal")
	f.add(t, f.applications["A"], "fact-A")
	f.edge(t, "B", "A")
	f.edge(t, "A", "B")
	f.edge(t, "B", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("mutual-cycle inliner fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewRuleInlinerPluginWithModes(f.manager, f.rules, []ast.ASTHandle{f.declarations["Goal"]}, true, false, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("mutual cycle did not inline after one SCC break")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	for index := 0; index < count; index++ {
		rule, getErr := f.set.Rule(index)
		coneMust(t, getErr)
		head, getErr := rule.HeadDeclaration()
		if getErr == nil && head == f.declarations["B"] {
			getErr = errors.New("inlined predicate B still has a defining rule")
		}
		uninterpreted, tailErr := rule.UninterpretedTailSize()
		for tailIndex := 0; getErr == nil && tailErr == nil && tailIndex < uninterpreted; tailIndex++ {
			declaration, declarationErr := rule.TailDeclaration(tailIndex)
			if declarationErr != nil {
				tailErr = declarationErr
			} else if declaration == f.declarations["B"] {
				tailErr = errors.New("inlined predicate B remains in a tail")
			}
		}
		getErr = errors.Join(getErr, tailErr, rule.Close())
		coneMust(t, getErr)
	}
}

func TestRuleInlinerForbidsSecondRuleMultiplier(t *testing.T) {
	f := newConeFixture(t, "P", "Q", "Goal")
	f.add(t, f.applications["P"], "fact-P-1")
	f.add(t, f.applications["P"], "fact-P-2")
	f.add(t, f.applications["Q"], "fact-Q-1")
	f.add(t, f.applications["Q"], "fact-Q-2")
	goal, err := f.rules.NewRule(f.applications["Goal"], []ast.ASTHandle{f.applications["P"], f.applications["Q"]}, nil, "goal")
	coneMust(t, err)
	coneMust(t, f.set.Add(goal))
	coneMust(t, goal.Close())
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("multiplier inliner fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewRuleInlinerPluginWithModes(f.manager, f.rules, []ast.ASTHandle{f.declarations["Goal"]}, true, false, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("multiplier fixture was not partially inlined")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 4 {
		t.Fatalf("multiplier guard produced %d rules, want 4", count)
	}
	qHeads := 0
	goalHeads := 0
	for index := 0; index < count; index++ {
		rule, getErr := f.set.Rule(index)
		coneMust(t, getErr)
		head, getErr := rule.HeadDeclaration()
		if getErr == nil {
			switch head {
			case f.declarations["P"]:
				getErr = errors.New("first multiplier P was not inlined")
			case f.declarations["Q"]:
				qHeads++
			case f.declarations["Goal"]:
				goalHeads++
				positive, tailErr := rule.PositiveTailSize()
				if tailErr != nil {
					getErr = tailErr
				} else if positive != 1 {
					getErr = errors.New("partially inlined goal does not have one tail")
				} else if tail, tailErr := rule.TailDeclaration(0); tailErr != nil {
					getErr = tailErr
				} else if tail != f.declarations["Q"] {
					getErr = errors.New("second multiplier Q was not retained")
				}
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		coneMust(t, getErr)
	}
	if qHeads != 2 || goalHeads != 2 {
		t.Fatal("multiplier guard heads", qHeads, goalHeads)
	}
}

func TestRuleInlinerOrientedEagerUsesUniqueUnifyingDefinition(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	one, err := f.manager.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	pName, err := f.manager.Symbols.Intern("oriented-P")
	coneMust(t, err)
	p, err := f.manager.MkFuncDecl(pName, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	goalName, err := f.manager.Symbols.Intern("oriented-Goal")
	coneMust(t, err)
	goal, err := f.manager.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(p))
	coneMust(t, f.rules.RegisterPredicate(goal))
	pZero, err := f.manager.MkApp(p, []ast.ASTHandle{zero})
	coneMust(t, err)
	pOne, err := f.manager.MkApp(p, []ast.ASTHandle{one})
	coneMust(t, err)
	goalApp, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, name string) {
		rule, addErr := f.rules.NewRule(head, tails, nil, name)
		coneMust(t, addErr)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	add(pZero, nil, "p-zero")
	add(pOne, nil, "p-one")
	add(goalApp, []ast.ASTHandle{pZero}, "goal")
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("oriented eager inliner fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewRuleInlinerPluginWithModes(f.manager, f.rules, []ast.ASTHandle{p, goal}, true, false, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("oriented eager pass did not use the unique unifying definition")
	}
	rules, err := f.set.PredicateRules(goal)
	coneMust(t, err)
	defer func() {
		for _, rule := range rules {
			coneMust(t, rule.Close())
		}
	}()
	if len(rules) != 1 {
		t.Fatalf("oriented eager goal rules %d, want 1", len(rules))
	}
	if tails, tailErr := rules[0].TailSize(); tailErr != nil || tails != 0 {
		t.Fatal("oriented eager result tail", tails, tailErr)
	}
}

func TestRuleInlinerDropsResolvedFalseInterpretedTail(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	one, err := f.manager.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	pName, err := f.manager.Symbols.Intern("resolved-false-P")
	coneMust(t, err)
	p, err := f.manager.MkFuncDecl(pName, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	goalName, err := f.manager.Symbols.Intern("resolved-false-Goal")
	coneMust(t, err)
	goal, err := f.manager.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(p))
	coneMust(t, f.rules.RegisterPredicate(goal))
	variable, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	pVariable, err := f.manager.MkApp(p, []ast.ASTHandle{variable})
	coneMust(t, err)
	equality, err := f.manager.MkEq(variable, one)
	coneMust(t, err)
	definition, err := f.rules.NewRule(pVariable, []ast.ASTHandle{equality}, nil, "definition")
	coneMust(t, err)
	coneMust(t, f.set.Add(definition))
	coneMust(t, definition.Close())
	pZero, err := f.manager.MkApp(p, []ast.ASTHandle{zero})
	coneMust(t, err)
	goalApp, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	query, err := f.rules.NewRule(goalApp, []ast.ASTHandle{pZero}, nil, "query")
	coneMust(t, err)
	coneMust(t, f.set.Add(query))
	coneMust(t, query.Close())
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("resolved-false inliner fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	plugin := transforms.NewRuleInlinerPluginWithServices(f.manager, f.rules, []ast.ASTHandle{goal}, true, false, f.globals.Context, f.globals, 0)
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("resolved false interpreted tail was not removed")
	}
	if count, countErr := f.set.Len(); countErr != nil || count != 0 {
		t.Fatal("resolved false rules", count, countErr)
	}
}

func TestRuleInlinerEliminatesAcyclicChainIntoProtectedOutput(t *testing.T) {
	f := newConeFixture(t, "P", "Q", "Goal")
	f.add(t, f.applications["P"], "fact-P")
	f.edge(t, "P", "Q")
	f.edge(t, "Q", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("inliner chain rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	plugin := transforms.NewRuleInlinerPlugin(f.manager, f.rules, []ast.ASTHandle{f.declarations["Goal"]}, 0)
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("acyclic chain was not inlined")
	}
	if count, err := f.set.Len(); err != nil || count != 1 {
		t.Fatal("inlined rule count", count, err)
	}
	modelRules, err := plugin.TakeModelRules()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transforms.CloseInlineModelRules(modelRules)) })
	if len(modelRules) != 2 {
		t.Fatal("inliner model-rule count", len(modelRules))
	}
	rule, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	head, err := rule.HeadDeclaration()
	coneMust(t, err)
	tails, err := rule.TailSize()
	coneMust(t, err)
	if head != f.declarations["Goal"] || tails != 0 {
		t.Fatal("inlined output rule", head, tails)
	}
}

func TestRuleInlinerUsesDisjointVariableBanks(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, e := f.manager.Symbols.Intern("inline-" + name)
		coneMust(t, e)
		declaration, e := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, e)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("P")
	q := declare("Q")
	pZero, err := f.manager.MkApp(p, []ast.ASTHandle{zero})
	coneMust(t, err)
	fact, err := f.rules.NewRule(pZero, nil, nil, "fact")
	coneMust(t, err)
	coneMust(t, f.set.Add(fact))
	coneMust(t, fact.Close())
	variable, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	pVariable, err := f.manager.MkApp(p, []ast.ASTHandle{variable})
	coneMust(t, err)
	qVariable, err := f.manager.MkApp(q, []ast.ASTHandle{variable})
	coneMust(t, err)
	edge, err := f.rules.NewRule(qVariable, []ast.ASTHandle{pVariable}, nil, "edge")
	coneMust(t, err)
	coneMust(t, f.set.Add(edge))
	coneMust(t, edge.Close())
	coneMust(t, f.set.SetOutputPredicate(q))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("variable inliner fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewRuleInlinerPlugin(f.manager, f.rules, []ast.ASTHandle{q}, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("variable rule was not inlined")
	}
	rule, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	head, err := rule.Head()
	coneMust(t, err)
	argument, err := f.manager.GetArg(head, 0)
	coneMust(t, err)
	if headDeclaration, err := f.manager.GetDecl(head); err != nil || headDeclaration != q || argument != zero {
		t.Fatal("resolved variable head", headDeclaration, argument, err)
	}
}

func TestRuleInlinerPreservesRecursivePredicate(t *testing.T) {
	f := newConeFixture(t, "P", "Goal")
	f.add(t, f.applications["P"], "fact-P")
	f.edge(t, "P", "P")
	f.edge(t, "P", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("recursive inliner fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewRuleInlinerPlugin(f.manager, f.rules, []ast.ASTHandle{f.declarations["Goal"]}, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if modified {
		t.Fatal("recursive predicate was inlined")
	}
}

func TestRuleInlinerContractsUniqueLinearGroundChain(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, e := f.manager.Symbols.Intern("linear-" + name)
		coneMust(t, e)
		declaration, e := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, e)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	predicate := declare("P", []ast.ASTHandle{integer})
	goal := declare("Goal", nil)
	values := make([]ast.ASTHandle, 3)
	applications := make([]ast.ASTHandle, 3)
	for index := range values {
		if index == 0 {
			values[index] = zero
		} else {
			values[index], err = f.manager.ArithmeticNumeralRat(big.NewRat(int64(index), 1), true)
			coneMust(t, err)
		}
		applications[index], err = f.manager.MkApp(predicate, []ast.ASTHandle{values[index]})
		coneMust(t, err)
	}
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, name string) {
		rule, e := f.rules.NewRule(head, tails, nil, name)
		coneMust(t, e)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	add(applications[0], nil, "fact")
	add(applications[1], []ast.ASTHandle{applications[0]}, "step-1")
	add(applications[2], []ast.ASTHandle{applications[1]}, "step-2")
	goalApplication, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	add(goalApplication, []ast.ASTHandle{applications[2]}, "goal")
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("linear chain fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewRuleInlinerPlugin(f.manager, f.rules, []ast.ASTHandle{goal}, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("unique linear chain was not contracted")
	}
	if count, getErr := f.set.Len(); getErr != nil || count != 1 {
		t.Fatal("contracted rule count", count, getErr)
	}
	rule, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	head, err := rule.HeadDeclaration()
	coneMust(t, err)
	if tails, getErr := rule.TailSize(); getErr != nil || head != goal || tails != 0 {
		t.Fatal("contracted goal fact", head, tails, getErr)
	}
}

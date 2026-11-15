package spacer_test

// Source: src/muz/spacer/spacer_context.cpp predicate rule initialization.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/spacer"
)

func TestSpacerPredicateRulesBuildTaggedInitialAndTransitionSystem(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	pName, err := manager.Symbols.Intern("spacer-rule-p")
	spacerMust(t, err)
	pDecl, err := manager.MkFuncDecl(pName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	transformer, err := spacer.NewRulePredicateTransformer(owner, pDecl, spacer.PropSolverConfig{})
	spacerMust(t, err)
	ruleManager, err := base.NewRuleManager(manager)
	spacerMust(t, err)
	spacerMust(t, ruleManager.RegisterPredicate(pDecl))
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	initialHead, err := manager.MkApp(pDecl, []ast.ASTHandle{truth})
	spacerMust(t, err)
	condition := spacerBool(t, manager, "spacer-rule-condition")
	one, err := manager.ArithmeticIntegerNumeralText("1")
	spacerMust(t, err)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	spacerMust(t, err)
	choice, err := manager.MkIte(condition, one, two)
	spacerMust(t, err)
	constraint, err := manager.MkEq(choice, one)
	spacerMust(t, err)
	initialRule, err := ruleManager.NewRule(initialHead, []ast.ASTHandle{constraint}, []bool{false}, "initial")
	spacerMust(t, err)
	variable, err := manager.MkVar(0, boolean)
	spacerMust(t, err)
	recursiveHead, err := manager.MkApp(pDecl, []ast.ASTHandle{variable})
	spacerMust(t, err)
	recursiveTail, err := manager.MkApp(pDecl, []ast.ASTHandle{variable})
	spacerMust(t, err)
	recursiveRule, err := ruleManager.NewRule(recursiveHead, []ast.ASTHandle{recursiveTail}, nil, "recursive")
	spacerMust(t, err)
	spacerMust(t, transformer.InitializeRules(map[ast.ASTHandle]*spacer.PredicateTransformer{pDecl: transformer}, []*base.Rule{initialRule, recursiveRule}))
	spacerMust(t, transformer.InitReachFacts())
	if has, err := transformer.HasReachFacts(); err != nil || !has {
		t.Fatal("initial Spacer reach facts", has, err)
	}
	recursiveTransition, err := transformer.RuleTransition(recursiveRule)
	spacerMust(t, err)
	if recursiveTransition.IsNull() {
		t.Fatal("missing normalized recursive transition")
	}
	computed, err := spacer.NewReachFact(manager, recursiveRule, truth, nil, false)
	spacerMust(t, err)
	if added, err := transformer.AddReachFact(computed); err != nil || !added {
		t.Fatal("computed Spacer reach fact", added, err)
	}
	spacerMust(t, computed.Close())
	reachFacts, err := transformer.ReachFacts()
	spacerMust(t, err)
	if len(reachFacts) != 2 {
		t.Fatal("Spacer reach fact count", len(reachFacts))
	}
	for _, fact := range reachFacts {
		spacerMust(t, fact.Close())
	}
	if _, found, err := transformer.LastReachCaseVariable(); err != nil || !found {
		t.Fatal("last Spacer reach-case variable", found, err)
	}
	if reachable, reachModel, err := transformer.IsMustReachable(truth, false); err != nil || !reachable || reachModel != nil {
		t.Fatal("Spacer must-reachability check", reachable, reachModel, err)
	}
	rules, err := transformer.Rules()
	spacerMust(t, err)
	if len(rules) != 2 {
		t.Fatal("normalized Spacer rules", len(rules))
	}
	for _, rule := range rules {
		spacerMust(t, rule.Close())
	}
	initial, err := transformer.InitialState()
	spacerMust(t, err)
	transition, err := transformer.Transition()
	spacerMust(t, err)
	if manager.IsTrue(initial) || manager.IsFalse(transition) {
		t.Fatal("tagged Spacer system", initial, transition)
	}
	features, err := ast.NewStaticFeatures(manager)
	spacerMust(t, err)
	spacerMust(t, features.Collect(transition))
	if features.NumITETerms != 0 {
		t.Fatal("Spacer rule normalization retained term ITE", features.NumITETerms)
	}
	spacerMust(t, features.Close())
	if all, err := transformer.AllInitial(); err != nil || all {
		t.Fatal("mixed initial/recursive classification", all, err)
	}
	signature, err := transformer.Signature()
	spacerMust(t, err)
	current, err := owner.OldToNew(signature[0], 0)
	spacerMust(t, err)
	state, err := manager.MkApp(current, nil)
	spacerMust(t, err)
	if added, err := transformer.AddLemma(state, 0); err != nil || !added {
		t.Fatal("self-predecessor lemma propagation", added, err)
	}
	tag, err := transformer.RuleTag(recursiveRule)
	spacerMust(t, err)
	md, err := model.NewModel(manager)
	spacerMust(t, err)
	tagDeclaration, err := manager.GetDecl(tag)
	spacerMust(t, err)
	spacerMust(t, md.Core().RegisterConstant(tagDeclaration, truth))
	lastReach, found, err := transformer.LastReachCaseVariable()
	spacerMust(t, err)
	if !found {
		t.Fatal("missing Spacer reach-case variable for rule selection")
	}
	lastReachDeclaration, err := manager.GetDecl(lastReach)
	spacerMust(t, err)
	falsity, err := manager.MkFalse()
	spacerMust(t, err)
	spacerMust(t, md.Core().RegisterConstant(lastReachDeclaration, falsity))
	originReach, err := owner.FormulaNewToOld(lastReach, 0, false)
	spacerMust(t, err)
	originReachHandle, err := manager.RefGet(originReach)
	spacerMust(t, err)
	originDeclaration, err := manager.GetDecl(originReachHandle)
	spacerMust(t, err)
	spacerMust(t, md.Core().RegisterConstant(originDeclaration, falsity))
	usedReach, found, err := transformer.UsedReachFact(md.Core(), true)
	spacerMust(t, err)
	if !found {
		t.Fatal("Spacer current-state reach fact not selected")
	}
	spacerMust(t, usedReach.Close())
	usedOrigin, found, err := transformer.UsedOriginReachFact(md.Core(), 0)
	spacerMust(t, err)
	if !found {
		t.Fatal("Spacer origin reach fact not selected")
	}
	spacerMust(t, usedOrigin.Close())
	originSummary, auxiliaries, err := transformer.OriginSummary(md.Core(), 0, 0, true)
	spacerMust(t, err)
	originSummaryHandle, err := manager.RefGet(originSummary)
	spacerMust(t, err)
	if !manager.IsTrue(originSummaryHandle) || len(auxiliaries) != 0 {
		t.Fatal("Spacer must origin summary", originSummaryHandle, auxiliaries)
	}
	spacerMust(t, manager.DelRef(&originSummary))
	originState, err := owner.FormulaNewToOld(state, 0, false)
	spacerMust(t, err)
	originStateHandle, err := manager.RefGet(originState)
	spacerMust(t, err)
	originStateDeclaration, err := manager.GetDecl(originStateHandle)
	spacerMust(t, err)
	spacerMust(t, md.Core().RegisterConstant(originStateDeclaration, truth))
	maySummary, auxiliaries, err := transformer.OriginSummary(md.Core(), 0, 0, false)
	spacerMust(t, err)
	if maySummary == (ast.ASTRef{}) || len(auxiliaries) != 0 {
		t.Fatal("Spacer may origin summary", maySummary, auxiliaries)
	}
	spacerMust(t, manager.DelRef(&maySummary))
	spacerMust(t, manager.DelRef(&originState))
	selection, err := transformer.SelectRule(md.Core())
	spacerMust(t, err)
	if selection.Rule == nil || !selection.IsConcrete || selection.NumReuseReach != 1 || len(selection.ReachPredUsed) != 1 || !selection.ReachPredUsed[0] {
		t.Fatal("Spacer model rule selection", selection.IsConcrete, selection.NumReuseReach, selection.ReachPredUsed)
	}
	selectedSame, err := selection.Rule.Same(recursiveRule)
	spacerMust(t, err)
	if !selectedSame {
		t.Fatal("Spacer model selected a different rule")
	}
	spacerMust(t, selection.Close())
	spacerMust(t, manager.DelRef(&originReach))
	spacerMust(t, md.Close())
	selected, found, err := transformer.TaggedRule(tag)
	spacerMust(t, err)
	if !found {
		t.Fatal("recursive Spacer rule tag not found")
	}
	same, err := selected.Same(recursiveRule)
	spacerMust(t, err)
	if !same {
		t.Fatal("Spacer rule tag selected a different rule")
	}
	spacerMust(t, selected.Close())
	predecessors, err := transformer.RulePredecessors(recursiveRule)
	spacerMust(t, err)
	if len(predecessors) != 1 || predecessors[0] != transformer {
		t.Fatal("recursive Spacer rule predecessors", len(predecessors))
	}
	notTag, err := manager.MkNot(tag)
	spacerMust(t, err)
	filtered, err := transformer.RemovePredecessors([]ast.ASTHandle{tag, state, notTag})
	spacerMust(t, err)
	if len(filtered) != 1 || filtered[0] != state {
		t.Fatal("Spacer predecessor tag removal", filtered)
	}
	rulePremises, err := transformer.RulePremises(0, recursiveRule)
	spacerMust(t, err)
	if len(rulePremises) != 1 {
		t.Fatal("recursive Spacer rule premises", len(rulePremises))
	}
	for i := range rulePremises {
		spacerMust(t, manager.DelRef(&rulePremises[i]))
	}
	premises, err := transformer.Premises(0)
	spacerMust(t, err)
	if len(premises) != 3 {
		t.Fatal("Spacer combined premises", len(premises))
	}
	for i := range premises {
		spacerMust(t, manager.DelRef(&premises[i]))
	}
	spacerMust(t, recursiveRule.Close())
	spacerMust(t, initialRule.Close())
	spacerMust(t, transformer.Close())
	spacerMust(t, ruleManager.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

// AUDIT.md muz_spacer_02-F1: pred_transformer::init_rule runs the standalone
// blast_term_ite rewriter, which hoists term ITEs into Boolean ITEs and
// introduces no fresh constants (elim_term_ite would add k!N symbols).
func TestSpacerPredicateRulesBlastTermITEIntroducesNoFreshConstants(t *testing.T) {
	_, manager, owner := lemmaOwnerFixture(t)
	arithmetic, err := manager.RegisterArithmeticPlugin()
	spacerMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	spacerMust(t, err)
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	pName, err := manager.Symbols.Intern("spacer-blast-ite-p")
	spacerMust(t, err)
	pDecl, err := manager.MkFuncDecl(pName, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	transformer, err := spacer.NewRulePredicateTransformer(owner, pDecl, spacer.PropSolverConfig{})
	spacerMust(t, err)
	ruleManager, err := base.NewRuleManager(manager)
	spacerMust(t, err)
	spacerMust(t, ruleManager.RegisterPredicate(pDecl))
	x, err := manager.MkVar(0, integer)
	spacerMust(t, err)
	y, err := manager.MkVar(1, integer)
	spacerMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	spacerMust(t, err)
	ten, err := manager.ArithmeticIntegerNumeralText("10")
	spacerMust(t, err)
	condition, err := manager.MkArithmeticLT(x, ten)
	spacerMust(t, err)
	successor, err := manager.MkArithmeticAdd(x, one)
	spacerMust(t, err)
	choice, err := manager.MkIte(condition, successor, x)
	spacerMust(t, err)
	constraint, err := manager.MkEq(y, choice)
	spacerMust(t, err)
	head, err := manager.MkApp(pDecl, []ast.ASTHandle{y})
	spacerMust(t, err)
	tail, err := manager.MkApp(pDecl, []ast.ASTHandle{x})
	spacerMust(t, err)
	recursive, err := ruleManager.NewRule(head, []ast.ASTHandle{tail, constraint}, nil, "recursive-ite")
	spacerMust(t, err)
	spacerMust(t, transformer.InitializeRules(map[ast.ASTHandle]*spacer.PredicateTransformer{pDecl: transformer}, []*base.Rule{recursive}))
	transition, err := transformer.RuleTransition(recursive)
	spacerMust(t, err)
	printer, err := ast.NewSimpleASTPrinterContext(manager, nil, nil)
	spacerMust(t, err)
	var text bytes.Buffer
	spacerMust(t, printer.DisplayExpr(&text, transition, 0))
	if strings.Contains(text.String(), "k!") {
		t.Fatal("blast_term_ite introduced fresh constants", text.String())
	}
	features, err := ast.NewStaticFeatures(manager)
	spacerMust(t, err)
	spacerMust(t, features.Collect(transition))
	if features.NumITETerms != 0 {
		t.Fatal("Spacer rule normalization retained term ITE", features.NumITETerms, text.String())
	}
	spacerMust(t, features.Close())
}

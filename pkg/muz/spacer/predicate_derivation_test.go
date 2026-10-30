package spacer_test

// Source: src/muz/spacer/spacer_context.cpp context::create_children.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerPredicateCreatesAndAttachesProjectedDerivation(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	name, err := manager.Symbols.Intern("spacer-create-children-p")
	spacerMust(t, err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	transformer, err := spacer.NewRulePredicateTransformer(owner, predicate, spacer.PropSolverConfig{LemmaSanityCheck: true, UseLemmaAsCTI: true})
	spacerMust(t, err)
	ruleManager, err := base.NewRuleManager(manager)
	spacerMust(t, err)
	spacerMust(t, ruleManager.RegisterPredicate(predicate))
	head, err := manager.MkApp(predicate, nil)
	spacerMust(t, err)
	initial, err := ruleManager.NewRule(head, nil, nil, "initial")
	spacerMust(t, err)
	recursive, err := ruleManager.NewRule(head, []ast.ASTHandle{head}, []bool{false}, "recursive")
	spacerMust(t, err)
	spacerMust(t, transformer.InitializeRules(map[ast.ASTHandle]*spacer.PredicateTransformer{predicate: transformer}, []*base.Rule{initial, recursive}))
	spacerMust(t, transformer.InitReachFacts())
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	obligation, err := transformer.NewProofObligation(nil, 2, 1, truth, nil)
	spacerMust(t, err)
	reachable, err := transformer.CheckReachability(obligation)
	spacerMust(t, err)
	if reachable.Status != lbool.True || reachable.Model == nil {
		t.Fatal("Spacer reachable obligation", reachable.Status, reachable.Model)
	}
	spacerMust(t, reachable.Close(manager))
	falsity, err := manager.MkFalse()
	spacerMust(t, err)
	blocked, err := transformer.NewProofObligation(nil, 0, 1, falsity, nil)
	spacerMust(t, err)
	unreachable, err := transformer.CheckReachability(blocked)
	spacerMust(t, err)
	if unreachable.Status != lbool.False {
		t.Fatal("Spacer unreachable obligation", unreachable.Status)
	}
	spacerMust(t, unreachable.Close(manager))
	blockedExpansion, err := transformer.ExpandObligation(blocked)
	spacerMust(t, err)
	if blockedExpansion.Status != lbool.False || !blockedExpansion.LemmaAdded {
		t.Fatal("Spacer blocking expansion", blockedExpansion.Status, blockedExpansion.LemmaAdded)
	}
	if useFarkas, farkasErr := blocked.UseFarkas(); farkasErr != nil || useFarkas {
		t.Fatal("Spacer lemma-as-CTI retained Farkas generalization", useFarkas, farkasErr)
	}
	md, err := model.NewModel(manager)
	spacerMust(t, err)
	lastReach, found, err := transformer.LastReachCaseVariable()
	spacerMust(t, err)
	if !found {
		t.Fatal("missing initial Spacer reach-case variable")
	}
	originReach, err := owner.FormulaNewToOld(lastReach, 0, false)
	spacerMust(t, err)
	originReachHandle, err := manager.RefGet(originReach)
	spacerMust(t, err)
	originReachDeclaration, err := manager.GetDecl(originReachHandle)
	spacerMust(t, err)
	spacerMust(t, md.Core().RegisterConstant(originReachDeclaration, falsity))
	child, err := transformer.CreateDerivationChild(obligation, recursive, md.Core(), []bool{false}, false)
	spacerMust(t, err)
	if child == nil {
		t.Fatal("Spacer create_children did not produce its first child")
	}
	parent, err := child.Parent()
	spacerMust(t, err)
	if parent != obligation {
		t.Fatal("Spacer create_children parent")
	}
	derivation, found, err := child.Derivation()
	spacerMust(t, err)
	if !found || derivation == nil {
		t.Fatal("Spacer create_children did not attach its derivation")
	}
	derivationParent, err := derivation.Parent()
	spacerMust(t, err)
	if derivationParent != obligation {
		t.Fatal("attached Spacer derivation tracks the wrong parent")
	}
	if level, err := child.Level(); err != nil || level != 1 {
		t.Fatal("Spacer create_children child level", level, err)
	}
	// A weak model can select an implicant without satisfying the original
	// post. C++ creates the child but drops its derivation in that case.
	weakPost := spacerBool(t, manager, "spacer-create-children-weak-post")
	weakDeclaration, err := manager.GetDecl(weakPost)
	spacerMust(t, err)
	spacerMust(t, md.Core().RegisterConstant(weakDeclaration, falsity))
	unknownPost := spacerBool(t, manager, "spacer-create-children-unknown-post")
	for _, post := range []ast.ASTHandle{weakPost, unknownPost} {
		for _, weakAbstraction := range []bool{true, false} {
			spacerMust(t, transformer.SetWeakAbstraction(weakAbstraction))
			weakObligation, err := transformer.NewProofObligation(nil, 2, 2, post, nil)
			spacerMust(t, err)
			weakChild, err := transformer.CreateDerivationChild(weakObligation, recursive, md.Core(), []bool{false}, false)
			spacerMust(t, err)
			if weakChild == nil {
				t.Fatal("missing child from weak model")
			}
			_, found, err := weakChild.Derivation()
			spacerMust(t, err)
			if found == weakAbstraction {
				t.Fatal("derivation attachment disagrees with source weak-model policy", weakAbstraction, found)
			}
		}
	}
	spacerMust(t, transformer.SetWeakAbstraction(true))
	spacerMust(t, transformer.SetDerivationPolicy(false, false))
	withoutDerivationParent, err := transformer.NewProofObligation(nil, 2, 2, truth, nil)
	spacerMust(t, err)
	withoutDerivation, err := transformer.CreateDerivationChild(withoutDerivationParent, recursive, md.Core(), []bool{false}, false)
	spacerMust(t, err)
	if withoutDerivation == nil {
		t.Fatal("Spacer create_children without retained derivations did not produce its first child")
	}
	if retained, found, err := withoutDerivation.Derivation(); err != nil || found || retained != nil {
		t.Fatal("Spacer create_children retained a disabled derivation", retained, found, err)
	}
	spacerMust(t, transformer.SetDerivationPolicy(true, false))
	concrete, err := transformer.MakeReachFact(obligation, recursive, md.Core(), true, true)
	spacerMust(t, err)
	justifications, err := concrete.Justifications()
	spacerMust(t, err)
	if len(justifications) != 1 {
		t.Fatal("projected Spacer reach-fact justifications", len(justifications))
	}
	for _, justification := range justifications {
		spacerMust(t, justification.Close())
	}
	if initial, err := concrete.IsInitial(); err != nil || initial {
		t.Fatal("computed Spacer reach fact marked initial", initial, err)
	}
	if added, err := transformer.AddReachFact(concrete); err != nil || added {
		t.Fatal("duplicate projected Spacer reach fact", added, err)
	}
	spacerMust(t, concrete.Close())
	spacerMust(t, transformer.SetProofObligationReuse(false))
	concreteObligation, err := transformer.NewProofObligation(nil, 0, 2, truth, nil)
	spacerMust(t, err)
	concreteExpansion, err := transformer.ExpandObligation(concreteObligation)
	spacerMust(t, err)
	if concreteExpansion.Status != lbool.True || !concreteExpansion.Concrete || concreteExpansion.Child != nil {
		t.Fatal("Spacer concrete expansion", concreteExpansion.Status, concreteExpansion.Concrete, concreteExpansion.Child)
	}
	queueRoot, err := transformer.NewProofObligation(nil, 0, 3, truth, nil)
	spacerMust(t, err)
	var queue spacer.ProofObligationQueue
	spacerMust(t, queue.SetRoot(queueRoot))
	if reached, err := queue.CheckReachability(); err != nil || !reached {
		t.Fatal("concrete Spacer obligation search", reached, err)
	}
	spacerMust(t, manager.DelRef(&originReach))
	spacerMust(t, md.Close())
	spacerMust(t, recursive.Close())
	spacerMust(t, initial.Close())
	spacerMust(t, ruleManager.Close())
	spacerMust(t, transformer.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerAbstractExpansionCreatesDerivationChild(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	name, err := manager.Symbols.Intern("spacer-abstract-child-p")
	spacerMust(t, err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	transformer, err := spacer.NewRulePredicateTransformer(owner, predicate, spacer.PropSolverConfig{})
	spacerMust(t, err)
	ruleManager, err := base.NewRuleManager(manager)
	spacerMust(t, err)
	spacerMust(t, ruleManager.RegisterPredicate(predicate))
	head, err := manager.MkApp(predicate, nil)
	spacerMust(t, err)
	recursive, err := ruleManager.NewRule(head, []ast.ASTHandle{head}, []bool{false}, "recursive")
	spacerMust(t, err)
	spacerMust(t, transformer.InitializeRules(map[ast.ASTHandle]*spacer.PredicateTransformer{predicate: transformer}, []*base.Rule{recursive}))
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	obligation, err := transformer.NewProofObligation(nil, 1, 0, truth, nil)
	spacerMust(t, err)
	expansion, err := transformer.ExpandObligation(obligation)
	spacerMust(t, err)
	if expansion.Status != lbool.Undef || expansion.Concrete || expansion.Child == nil {
		t.Fatal("Spacer abstract expansion", expansion.Status, expansion.Concrete, expansion.Child)
	}
	if _, found, err := expansion.Child.Derivation(); err != nil || !found {
		t.Fatal("Spacer abstract child derivation", found, err)
	}
	spacerMust(t, transformer.SetProofObligationReuse(false))
	queueRoot, err := transformer.NewProofObligation(nil, 1, 1, truth, nil)
	spacerMust(t, err)
	var queue spacer.ProofObligationQueue
	spacerMust(t, queue.SetRoot(queueRoot))
	if reached, err := queue.CheckReachability(); err != nil || reached {
		t.Fatal("blocked recursive Spacer obligation search", reached, err)
	}
	if expanded := queue.ExpandedLevel(); expanded != 0 {
		t.Fatal("blocked Spacer obligation expansion level", expanded)
	}
	spacerMust(t, recursive.Close())
	spacerMust(t, ruleManager.Close())
	spacerMust(t, transformer.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

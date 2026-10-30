package spacer_test

// Source: src/muz/spacer/spacer_context.cpp derivation premise ownership,
// origin variables, summary replacement, and proof-obligation attachment.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/spacer"
)

func TestSpacerDerivationOwnsPremisesAndAttachesToObligation(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	predicateName, err := manager.Symbols.Intern("spacer-derivation-p")
	spacerMust(t, err)
	predicate, err := manager.MkFuncDecl(predicateName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	transformer, err := spacer.NewRulePredicateTransformer(owner, predicate, spacer.PropSolverConfig{})
	spacerMust(t, err)
	ruleManager, err := base.NewRuleManager(manager)
	spacerMust(t, err)
	spacerMust(t, ruleManager.RegisterPredicate(predicate))
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	head, err := manager.MkApp(predicate, []ast.ASTHandle{truth})
	spacerMust(t, err)
	rule, err := ruleManager.NewRule(head, nil, nil, "derivation")
	spacerMust(t, err)
	root, err := transformer.NewProofObligation(nil, 2, 1, truth, nil)
	spacerMust(t, err)
	auxiliaryName, err := manager.Symbols.Intern("spacer-derivation-aux")
	spacerMust(t, err)
	auxiliaryDeclaration, err := manager.MkFuncDecl(auxiliaryName, nil, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	auxiliaryDeclaration, err = owner.NewPredicate(auxiliaryDeclaration)
	spacerMust(t, err)
	auxiliary, err := manager.MkApp(auxiliaryDeclaration, nil)
	spacerMust(t, err)
	derivation, err := spacer.NewDerivation(root, rule, truth, []ast.ASTHandle{auxiliary})
	spacerMust(t, err)
	spacerMust(t, derivation.AddPremise(transformer, 0, truth, false, []ast.ASTHandle{auxiliary}))
	md, err := model.NewModel(manager)
	spacerMust(t, err)
	firstChild, err := derivation.CreateFirstChild(md.Core())
	spacerMust(t, err)
	if firstChild == nil {
		t.Fatal("Spacer derivation did not create its first child")
	}
	if level, err := firstChild.Level(); err != nil || level != 1 {
		t.Fatal("Spacer derivation child level", level, err)
	}
	if parent, err := firstChild.Parent(); err != nil || parent != root {
		t.Fatal("Spacer derivation child parent", parent, err)
	}
	bindings, err := firstChild.Bindings()
	spacerMust(t, err)
	if len(bindings) != 1 {
		t.Fatal("Spacer derivation child bindings", len(bindings))
	}
	spacerMust(t, md.Close())
	projected, err := spacer.NewDerivation(root, rule, truth, nil)
	spacerMust(t, err)
	spacerMust(t, projected.AddPremise(transformer, 0, truth, true, nil))
	spacerMust(t, projected.AddPremise(transformer, 1, truth, false, nil))
	projectionModel, err := model.NewModel(manager)
	spacerMust(t, err)
	projectedChild, err := projected.CreateFirstChild(projectionModel.Core())
	spacerMust(t, err)
	if projectedChild == nil {
		t.Fatal("Spacer derivation did not project the leading must premise")
	}
	projectedBindings, err := projectedChild.Bindings()
	spacerMust(t, err)
	if len(projectedBindings) != 0 {
		t.Fatal("Spacer derivation retained eliminated variables", len(projectedBindings))
	}
	spacerMust(t, projectionModel.Close())
	spacerMust(t, projected.Close())
	reachFact, err := spacer.NewReachFact(manager, rule, truth, nil, true)
	spacerMust(t, err)
	if added, err := transformer.AddReachFact(reachFact); err != nil || !added {
		t.Fatal("Spacer derivation reach fact", added, err)
	}
	spacerMust(t, reachFact.Close())
	successive, err := spacer.NewDerivation(root, rule, truth, nil)
	spacerMust(t, err)
	spacerMust(t, successive.AddPremise(transformer, 0, truth, false, nil))
	spacerMust(t, successive.AddPremise(transformer, 1, truth, false, nil))
	successiveModel, err := model.NewModel(manager)
	spacerMust(t, err)
	if child, err := successive.CreateFirstChild(successiveModel.Core()); err != nil || child == nil {
		t.Fatal("first successive Spacer derivation child", child, err)
	}
	spacerMust(t, successiveModel.Close())
	secondChild, err := successive.CreateNextChild()
	spacerMust(t, err)
	if secondChild == nil {
		t.Fatal("second successive Spacer derivation child")
	}
	if child, err := successive.CreateNextChild(); err != nil || child != nil {
		t.Fatal("exhausted Spacer derivation produced a child", child, err)
	}
	spacerMust(t, successive.Close())
	premises, err := derivation.Premises()
	spacerMust(t, err)
	if len(premises) != 1 || premises[0].Must() || premises[0].OldIndex() != 0 || premises[0].Transformer() != transformer {
		t.Fatal("Spacer derivation premise metadata", len(premises))
	}
	variables, err := premises[0].Variables()
	spacerMust(t, err)
	if len(variables) != 2 {
		t.Fatal("Spacer derivation origin variables", len(variables))
	}
	falsity, err := manager.MkFalse()
	spacerMust(t, err)
	spacerMust(t, premises[0].SetSummary(falsity, true, nil))
	variables, err = premises[0].Variables()
	spacerMust(t, err)
	if !premises[0].Must() || len(variables) != 1 {
		t.Fatal("Spacer derivation premise update", premises[0].Must(), len(variables))
	}
	spacerMust(t, root.SetDerivation(derivation))
	attached, found, err := root.Derivation()
	spacerMust(t, err)
	if !found || attached != derivation {
		t.Fatal("Spacer derivation attachment", found)
	}
	detached, err := root.DetachDerivation()
	spacerMust(t, err)
	if detached != derivation {
		t.Fatal("Spacer derivation detachment")
	}
	spacerMust(t, root.SetDerivation(detached))
	spacerMust(t, root.SetUseFarkas(false))
	spacerMust(t, root.BumpWeakness())
	reused, err := transformer.NewProofObligation(nil, 3, 2, truth, nil)
	spacerMust(t, err)
	if reused != root {
		t.Fatal("expected reuse of the obligation with the same normalized post")
	}
	if _, found, err := reused.Derivation(); err != nil || found {
		t.Fatal("pob::inherit must clear the previous derivation", found, err)
	}
	if _, err := detached.Transition(); err == nil {
		t.Fatal("reusing the obligation must destroy its old derivation")
	}
	if _, err := premises[0].Summary(); err == nil {
		t.Fatal("old derivation premise survived obligation reuse")
	}
	if level, err := reused.Level(); err != nil || level != 3 {
		t.Fatal("inherited level", level, err)
	}
	if depth, err := reused.Depth(); err != nil || depth != 2 {
		t.Fatal("inherited depth", depth, err)
	}
	if farkas, err := reused.UseFarkas(); err != nil || !farkas {
		t.Fatal("inherited Farkas policy", farkas, err)
	}
	if weakness, err := reused.Weakness(); err != nil || weakness != 0 {
		t.Fatal("inherited weakness", weakness, err)
	}

	spacerMust(t, root.Reset())
	if _, found, err = root.Derivation(); err != nil || found {
		t.Fatal("Spacer reset retained derivation", found, err)
	}
	childName, err := manager.Symbols.Intern("spacer-derivation-child")
	spacerMust(t, err)
	childPredicate, err := manager.MkFuncDecl(childName, nil, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	childTransformer, err := spacer.NewRulePredicateTransformer(owner, childPredicate, spacer.PropSolverConfig{})
	spacerMust(t, err)
	child, err := childTransformer.NewProofObligation(root, 1, 1, truth, nil)
	spacerMust(t, err)
	if parent, err := child.Parent(); err != nil || parent != root {
		t.Fatal("cross-transformer Spacer derivation child", parent, err)
	}
	spacerMust(t, childTransformer.Close())
	spacerMust(t, rule.Close())
	spacerMust(t, ruleManager.Close())
	spacerMust(t, transformer.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerDerivationSkipsProjectionWithoutVariables(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	defer func() { spacerMust(t, command.Close()) }()
	defer func() { spacerMust(t, owner.Close()) }()
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	name, err := manager.Symbols.Intern("derivation-no-vars-p")
	spacerMust(t, err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	transformer, err := spacer.NewRulePredicateTransformer(owner, predicate, spacer.PropSolverConfig{})
	spacerMust(t, err)
	defer func() { spacerMust(t, transformer.Close()) }()
	rules, err := base.NewRuleManager(manager)
	spacerMust(t, err)
	defer func() { spacerMust(t, rules.Close()) }()
	spacerMust(t, rules.RegisterPredicate(predicate))
	head, err := manager.MkApp(predicate, nil)
	spacerMust(t, err)
	rule, err := rules.NewRule(head, nil, nil, "no-vars")
	spacerMust(t, err)
	defer func() { spacerMust(t, rule.Close()) }()
	recursive, err := rules.NewRule(head, []ast.ASTHandle{head}, []bool{false}, "no-vars-recursive")
	spacerMust(t, err)
	defer func() { spacerMust(t, recursive.Close()) }()
	spacerMust(t, transformer.InitializeRules(map[ast.ASTHandle]*spacer.PredicateTransformer{predicate: transformer}, []*base.Rule{rule, recursive}))
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	root, err := transformer.NewProofObligation(nil, 2, 1, truth, nil)
	spacerMust(t, err)
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			spacerMust(t, manager.DelRef(&pins[i]))
		}
	}()
	own := func(value ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		spacerMust(t, err)
		ref, err := manager.NewRef(value)
		spacerMust(t, err)
		pins = append(pins, ref)
		return value
	}
	constant := func(text string, sort ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		name, err := manager.Symbols.Intern(text)
		spacerMust(t, err)
		decl, err := manager.MkFuncDecl(name, nil, sort, ast.ASTFuncDeclInfo{})
		spacerMust(t, err)
		return own(manager.MkApp(decl, nil))
	}
	zero := own(manager.ArithmeticIntegerNumeralText("0"))
	one := own(manager.ArithmeticIntegerNumeralText("1"))
	integer, err := manager.GetSort(zero)
	spacerMust(t, err)
	arraySort := own(manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), integer))
	a := constant("derivation-no-vars-a", arraySort)
	i := constant("derivation-no-vars-i", integer)
	j := constant("derivation-no-vars-j", integer)
	arrays, err := manager.ArrayFamilyID()
	spacerMust(t, err)
	stored := own(manager.MkFamilyApp(arrays, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, one}))
	selected := own(manager.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{stored, j}))
	transition := own(manager.MkEq(selected, one))
	md, err := model.NewModel(manager)
	spacerMust(t, err)
	defer func() { spacerMust(t, md.Close()) }()
	baseArray := own(manager.MkConstArray(arraySort, zero))
	for _, binding := range [][2]ast.ASTHandle{{a, baseArray}, {i, zero}, {j, zero}} {
		decl, err := manager.GetDecl(binding[0])
		spacerMust(t, err)
		spacerMust(t, md.Core().RegisterConstant(decl, binding[1]))
	}
	// Exercise both an active first premise and a leading must premise. Neither
	// zero-arity predicate contributes variables to either projection stage.
	for _, leadingMust := range []bool{false, true} {
		derivation, err := spacer.NewDerivation(root, rule, transition, nil)
		spacerMust(t, err)
		if leadingMust {
			spacerMust(t, derivation.AddPremise(transformer, 0, truth, true, nil))
		}
		spacerMust(t, derivation.AddPremise(transformer, 1, truth, false, nil))
		child, err := derivation.CreateFirstChild(md.Core())
		spacerMust(t, err)
		if child == nil {
			t.Fatal("missing child for zero-arity premise", leadingMust)
		}
		actual, err := derivation.Transition()
		spacerMust(t, err)
		containsStore, err := manager.OccursExpr(stored, actual)
		spacerMust(t, err)
		if !containsStore || (!leadingMust && actual != transition) {
			t.Fatal("empty-variable projection specialized the symbolic transition", leadingMust)
		}
		post, err := child.Post()
		spacerMust(t, err)
		containsStore, err = manager.OccursExpr(stored, post)
		spacerMust(t, err)
		if !containsStore {
			t.Fatal("empty-variable post projection removed the symbolic store", leadingMust)
		}
		spacerMust(t, derivation.Close())
	}
	// context::create_children calls QE unconditionally, even for a zero-arity
	// head and no auxiliaries. Its initial reduction must still happen before
	// the guarded derivation stages above.
	obligation, err := transformer.NewProofObligation(nil, 2, 1, transition, nil)
	spacerMust(t, err)
	child, err := transformer.CreateDerivationChild(obligation, recursive, md.Core(), []bool{false}, false)
	spacerMust(t, err)
	if child == nil {
		t.Fatal("missing child after initial projection")
	}
	derivation, found, err := child.Derivation()
	spacerMust(t, err)
	if !found {
		t.Fatal("missing projected derivation")
	}
	actual, err := derivation.Transition()
	spacerMust(t, err)
	containsStore, err := manager.OccursExpr(stored, actual)
	spacerMust(t, err)
	if containsStore {
		t.Fatal("create_children skipped unconditional initial store/select reduction")
	}

	// mk_reach_fact uses the same driver, but requires full elimination and
	// disables reduction of stores belonging to arrays outside the variable set.
	t.Run("reach-fact-projection-policy", func(t *testing.T) {
		name, err := manager.Symbols.Intern("reach-policy-p")
		spacerMust(t, err)
		predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		spacerMust(t, err)
		pt, err := spacer.NewRulePredicateTransformer(owner, predicate, spacer.PropSolverConfig{})
		spacerMust(t, err)
		defer func() { spacerMust(t, pt.Close()) }()
		spacerMust(t, rules.RegisterPredicate(predicate))
		head := own(manager.MkApp(predicate, nil))
		x := own(manager.MkVar(0, integer))
		four := own(manager.ArithmeticIntegerNumeralText("4"))
		square := own(manager.MkArithmeticMul(x, x))
		nonlinear := own(manager.MkEq(square, four))
		rule, err := rules.NewRule(head, []ast.ASTHandle{transition, nonlinear}, nil, "reach-policy")
		spacerMust(t, err)
		defer func() { spacerMust(t, rule.Close()) }()
		spacerMust(t, pt.InitializeRules(map[ast.ASTHandle]*spacer.PredicateTransformer{predicate: pt}, []*base.Rule{rule}))
		auxiliaries, err := pt.RuleAuxiliaries(rule)
		spacerMust(t, err)
		if len(auxiliaries) != 1 {
			t.Fatal("expected one nonlinear rule auxiliary", len(auxiliaries))
		}
		two := own(manager.ArithmeticIntegerNumeralText("2"))
		decl, err := manager.GetDecl(auxiliaries[0])
		spacerMust(t, err)
		spacerMust(t, md.Core().RegisterConstant(decl, two))
		obligation, err := pt.NewProofObligation(nil, 0, 1, truth, nil)
		spacerMust(t, err)
		fact, err := pt.MakeReachFact(obligation, rule, md.Core(), true, false)
		spacerMust(t, err)
		defer func() { spacerMust(t, fact.Close()) }()
		formula, err := fact.Fact()
		spacerMust(t, err)
		containsStore, err := manager.OccursExpr(stored, formula)
		spacerMust(t, err)
		if !containsStore {
			t.Fatal("reach projection reduced an array outside its elimination set")
		}
		containsAuxiliary, err := manager.OccursExpr(auxiliaries[0], formula)
		spacerMust(t, err)
		if containsAuxiliary {
			t.Fatal("reach projection failed to substitute the remaining nonlinear auxiliary")
		}
	})

}

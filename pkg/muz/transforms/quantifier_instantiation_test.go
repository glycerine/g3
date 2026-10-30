package transforms_test

// Source: src/muz/transforms/dl_mk_quantifier_instantiation.cpp extraction,
// trigger matching, binding reversal, and quantifier removal.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierInstantiationMatchesPatternAgainstHornEGraph(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, context.Close()) })
	manager, err := context.Manager()
	coneMust(t, err)
	rules, err := base.NewRuleManager(manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, rules.Close()) })
	set, err := base.NewRuleSet(rules)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, set.Close()) })

	boolean, err := manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	falsity, err := manager.MkFalse()
	coneMust(t, err)
	truth, err := manager.MkTrue()
	coneMust(t, err)
	array, err := manager.MkConstArray(arraySort, falsity)
	coneMust(t, err)
	arrayFamily, err := manager.ArrayFamilyID()
	coneMust(t, err)
	groundSelect, err := manager.MkFamilyApp(arrayFamily, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, truth})
	coneMust(t, err)
	groundEquality, err := manager.MkEq(groundSelect, falsity)
	coneMust(t, err)
	bound, err := manager.MkVar(0, boolean)
	coneMust(t, err)
	trigger, err := manager.MkFamilyApp(arrayFamily, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, bound})
	coneMust(t, err)
	body, err := manager.MkEq(trigger, falsity)
	coneMust(t, err)
	quantifier, err := manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)},
		Body: body,
	})
	coneMust(t, err)
	quantifiedTail, err := manager.MkIff(quantifier, truth)
	coneMust(t, err)

	name, err := manager.Symbols.Intern("quantifier-instantiation-goal")
	coneMust(t, err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, rules.RegisterPredicate(declaration))
	head, err := manager.MkApp(declaration, nil)
	coneMust(t, err)
	rule, err := rules.NewRule(head, []ast.ASTHandle{groundEquality, quantifiedTail}, nil, "instantiate")
	coneMust(t, err)
	coneMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	ruleFormula, err := rule.ToFormula()
	coneMust(t, err)
	ruleFact, err := manager.RefGet(ruleFormula)
	coneMust(t, err)
	sourceProof, err := manager.MkProofAsserted(ruleFact)
	coneMust(t, err)
	coneMust(t, rule.SetProof(sourceProof))
	coneMust(t, manager.DelRef(&ruleFormula))
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("source rule set is not stratified")
	}

	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(transforms.NewQuantifierInstantiationPlugin(manager, rules, 37000)))
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	modified, err := transformer.Apply(set)
	coneMust(t, err)
	if !modified {
		t.Fatal("quantifier instantiation did not modify the rule set")
	}
	rewritten, err := set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rewritten.Close()) }()
	proof, hasProof, err := rewritten.Proof()
	if err != nil || !hasProof {
		t.Fatalf("instantiated rule lost the source weakening proof: found=%v err=%v", hasProof, err)
	}
	provedFact, err := manager.GetProofFact(proof)
	coneMust(t, err)
	rewrittenFormula, err := rewritten.ToFormula()
	coneMust(t, err)
	rewrittenFact, err := manager.RefGet(rewrittenFormula)
	coneMust(t, err)
	equal, err := manager.AreEqual(provedFact, rewrittenFact)
	coneMust(t, err)
	coneMust(t, manager.DelRef(&rewrittenFormula))
	if !equal {
		t.Fatal("instantiation weakening proof does not establish rewritten rule")
	}
	size, err := rewritten.TailSize()
	coneMust(t, err)
	if size != 2 {
		t.Fatalf("instantiated tail size %d, want ground conjunct plus instance", size)
	}
	for i := 0; i < size; i++ {
		tail, _, getErr := rewritten.Tail(i)
		coneMust(t, getErr)
		quantified, getErr := manager.HasQuantifiers(tail)
		coneMust(t, getErr)
		if quantified {
			t.Fatal("quantifier survived E-matching transform")
		}
		if tail != groundEquality {
			t.Fatal("unexpected quantifier instance", tail, groundEquality)
		}
	}
}

func TestQuantifierInstantiationRefusesNegatedRulesAndNoQuantifiers(t *testing.T) {
	t.Run("negation", func(t *testing.T) {
		f := newConeFixture(t, "P", "Q")
		addTransformRule(t, f, f.applications["P"], []ast.ASTHandle{f.applications["Q"]}, []bool{true}, "negative")
		plugin := transforms.NewQuantifierInstantiationPlugin(f.manager, f.rules, 37000)
		defer func() { coneMust(t, plugin.Close()) }()
		result, err := plugin.Transform(f.set)
		coneMust(t, err)
		if result != nil {
			coneMust(t, result.Close())
			t.Fatal("quantifier instantiation changed a rule set with negation")
		}
	})
	t.Run("no-quantifiers", func(t *testing.T) {
		f := newConeFixture(t, "P")
		f.add(t, f.applications["P"], "plain-fact")
		plugin := transforms.NewQuantifierInstantiationPlugin(f.manager, f.rules, 37000)
		defer func() { coneMust(t, plugin.Close()) }()
		result, err := plugin.Transform(f.set)
		coneMust(t, err)
		if result != nil {
			coneMust(t, result.Close())
			t.Fatal("quantifier-free set produced an instantiation replacement")
		}
	})
}

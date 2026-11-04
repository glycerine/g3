package transforms_test

// Source: src/muz/transforms/dl_mk_quantifier_abstraction.cpp predicate
// expansion, fresh head indices, shifted quantified tails, and output retention.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/dataflow"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestQuantifierAbstractionExpandsInternalArrayPredicate(t *testing.T) {
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
	pName, err := manager.Symbols.Intern("quantifier-abstraction-p")
	coneMust(t, err)
	pDecl, err := manager.MkFuncDecl(pName, []ast.ASTHandle{arraySort}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	goalName, err := manager.Symbols.Intern("quantifier-abstraction-goal")
	coneMust(t, err)
	goalDecl, err := manager.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, rules.RegisterPredicate(pDecl))
	coneMust(t, rules.RegisterPredicate(goalDecl))
	arrayVariable, err := manager.MkVar(0, arraySort)
	coneMust(t, err)
	pApp, err := manager.MkApp(pDecl, []ast.ASTHandle{arrayVariable})
	coneMust(t, err)
	goalApp, err := manager.MkApp(goalDecl, nil)
	coneMust(t, err)
	fact, err := rules.NewRule(pApp, nil, nil, "array-fact")
	coneMust(t, err)
	query, err := rules.NewRule(goalApp, []ast.ASTHandle{pApp}, nil, "array-query")
	coneMust(t, err)
	coneMust(t, set.Add(fact))
	coneMust(t, set.Add(query))
	coneMust(t, fact.Close())
	coneMust(t, query.Close())
	coneMust(t, set.SetOutputPredicate(goalDecl))
	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("source rule set is not stratified")
	}

	plugin := transforms.NewQuantifierAbstractionPlugin(manager, rules, 38000)
	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(plugin))
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	modified, err := transformer.Apply(set)
	coneMust(t, err)
	if !modified {
		t.Fatal("array predicate abstraction did not modify the rule set")
	}
	fresh := plugin.FreshPredicates()[pDecl]
	if fresh.IsNull() {
		t.Fatal("missing expanded predicate declaration")
	}
	if arity, getErr := manager.GetArity(fresh); getErr != nil || arity != 2 {
		t.Fatal("expanded predicate arity", arity, getErr)
	}
	if original, getErr := set.OriginalPredicate(fresh); getErr != nil || original != pDecl {
		t.Fatal("expanded predicate mapping", original, getErr)
	}
	if output, getErr := set.IsOutputPredicate(goalDecl); getErr != nil || !output {
		t.Fatal("output predicate was not retained", output, getErr)
	}

	count, err := set.Len()
	coneMust(t, err)
	if count != 2 {
		t.Fatal("transformed rule count", count)
	}
	foundFreshHead, foundQuantifiedTail := false, false
	for i := 0; i < count; i++ {
		rule, getErr := set.Rule(i)
		coneMust(t, getErr)
		head, getErr := rule.HeadDeclaration()
		coneMust(t, getErr)
		if head == fresh {
			foundFreshHead = true
		}
		tails, getErr := rule.TailSize()
		coneMust(t, getErr)
		for j := 0; j < tails; j++ {
			tail, _, tailErr := rule.Tail(j)
			coneMust(t, tailErr)
			quantified, tailErr := manager.HasQuantifiers(tail)
			coneMust(t, tailErr)
			foundQuantifiedTail = foundQuantifiedTail || quantified
		}
		coneMust(t, rule.Close())
	}
	if !foundFreshHead || !foundQuantifiedTail {
		t.Fatal("missing transformed head or quantified tail", foundFreshHead, foundQuantifiedTail)
	}
	// Abstraction makes the formerly direct dependency occur inside a
	// quantified interpreted tail. Cone analysis must preserve its definition.
	reachable, err := dataflow.TopDown(set)
	coneMust(t, err)
	if !reachable.IsReachable(fresh) {
		t.Fatal("cone analysis lost the abstracted predicate definition")
	}
}

func TestQuantifierAbstractionRefusesNegationAndRulesWithoutArrays(t *testing.T) {
	t.Run("negation", func(t *testing.T) {
		f := newConeFixture(t, "P", "Q")
		addTransformRule(t, f, f.applications["P"], []ast.ASTHandle{f.applications["Q"]}, []bool{true}, "negative")
		plugin := transforms.NewQuantifierAbstractionPlugin(f.manager, f.rules, 38000)
		defer func() { coneMust(t, plugin.Close()) }()
		result, err := plugin.Transform(f.set)
		coneMust(t, err)
		if result != nil {
			coneMust(t, result.Close())
			t.Fatal("array abstraction transformed a rule set with negation")
		}
	})
	t.Run("no-array-predicates", func(t *testing.T) {
		f := newConeFixture(t, "P")
		f.add(t, f.applications["P"], "plain-fact")
		plugin := transforms.NewQuantifierAbstractionPlugin(f.manager, f.rules, 38000)
		defer func() { coneMust(t, plugin.Close()) }()
		result, err := plugin.Transform(f.set)
		coneMust(t, err)
		if result != nil {
			coneMust(t, result.Close())
			t.Fatal("array-free rule set produced an abstraction replacement")
		}
	})
}

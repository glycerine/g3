package transforms_test

// Source: src/muz/transforms/dl_mk_scale.cpp homogeneous predicate and
// interpreted-tail scaling.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestScaleAddsHomogeneousPredicateArgumentAndGuards(t *testing.T) {
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
	zero, err := manager.ArithmeticRealNumeralText("0")
	coneMust(t, err)
	one, err := manager.ArithmeticRealNumeralText("1")
	coneMust(t, err)
	two, err := manager.ArithmeticRealNumeralText("2")
	coneMust(t, err)
	realSort, err := manager.GetSort(two)
	coneMust(t, err)
	pName, err := manager.Symbols.Intern("scale-p")
	coneMust(t, err)
	p, err := manager.MkFuncDecl(pName, []ast.ASTHandle{realSort, realSort, realSort}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, rules.RegisterPredicate(p))
	pOfTwo, err := manager.MkApp(p, []ast.ASTHandle{zero, one, two})
	coneMust(t, err)
	rule, err := rules.NewRule(pOfTwo, nil, nil, "scale-fact")
	coneMust(t, err)
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, set.SetOutputPredicate(p))
	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("scale source is unstratified")
	}
	plugin := transforms.NewScalePlugin(manager, rules, 36030)
	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(plugin))
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	modified, err := transformer.Apply(set)
	coneMust(t, err)
	if !modified {
		t.Fatal("scale did not modify the rule set")
	}
	fresh := plugin.FreshPredicates()[p]
	if fresh.IsNull() {
		t.Fatal("missing scaled predicate")
	}
	if arity, getErr := manager.GetArity(fresh); getErr != nil || arity != 4 {
		t.Fatal("scaled predicate arity", arity, getErr)
	}
	if sort, getErr := manager.GetDomain(fresh, 3); getErr != nil || sort != realSort {
		t.Fatal("scale domain", sort, getErr)
	}
	transformed, err := set.Rule(0)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transformed.Close()) })
	if tails, getErr := transformed.TailSize(); getErr != nil || tails != 2 {
		t.Fatal("scale equality and positivity guard", tails, getErr)
	}
	head, err := transformed.Head()
	coneMust(t, err)
	zeroArg, err := manager.GetArg(head, 0)
	coneMust(t, err)
	oneArg, err := manager.GetArg(head, 1)
	coneMust(t, err)
	sigmaArg, err := manager.GetArg(head, 3)
	coneMust(t, err)
	if zeroArg != zero || oneArg != sigmaArg {
		t.Fatal("scale 0/1 argument branches", zeroArg, zero, oneArg, sigmaArg)
	}
	if original, getErr := set.OriginalPredicate(fresh); getErr != nil || original != p {
		t.Fatal("scaled predicate mapping", original, getErr)
	}
}

func TestScaleReusedDeclarationInheritsCurrentOutputs(t *testing.T) {
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
	zero, err := manager.ArithmeticRealNumeralText("0")
	coneMust(t, err)
	one, err := manager.ArithmeticRealNumeralText("1")
	coneMust(t, err)
	two, err := manager.ArithmeticRealNumeralText("2")
	coneMust(t, err)
	realSort, err := manager.GetSort(two)
	coneMust(t, err)
	pName, err := manager.Symbols.Intern("scale-p")
	coneMust(t, err)
	p, err := manager.MkFuncDecl(pName, []ast.ASTHandle{realSort, realSort, realSort}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, rules.RegisterPredicate(p))
	pOfTwo, err := manager.MkApp(p, []ast.ASTHandle{zero, one, two})
	coneMust(t, err)
	rule, err := rules.NewRule(pOfTwo, nil, nil, "scale-fact")
	coneMust(t, err)
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())

	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("scale source is unstratified")
	}
	plugin := transforms.NewScalePlugin(manager, rules, 36030)
	defer func() { coneMust(t, plugin.Close()) }()
	var previous ast.ASTHandle
	for pass := 0; pass < 3; pass++ {
		if pass == 1 {
			coneMust(t, set.SetOutputPredicate(p))
		}
		result, cause := plugin.Transform(set)
		coneMust(t, cause)
		if result == nil {
			t.Fatal("missing scaled rules")
		}
		fresh := plugin.FreshPredicates()[p]
		if pass != 0 && fresh != previous {
			t.Fatal("cached declaration changed")
		}
		previous = fresh
		output, cause := result.IsOutputPredicate(fresh)
		coneMust(t, cause)
		if output != (pass != 0) {
			t.Fatalf("pass %d: scaled output = %v", pass, output)
		}
		original, cause := result.OriginalPredicate(fresh)
		coneMust(t, cause)
		if original != p {
			t.Fatal("cached scaled declaration lost origin")
		}
		scaled, cause := result.Rule(0)
		coneMust(t, cause)
		if n, e := scaled.TailSize(); e != nil || n != 2 {
			t.Fatal("cached scale lost equality/positivity guards", n, e)
		}
		coneMust(t, scaled.Close())
		coneMust(t, result.Close())
	}
}

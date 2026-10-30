package transforms_test

// Source: src/muz/transforms/dl_mk_karr_invariants.cpp/.h forward/backward
// invariant discovery over a two-coordinate affine recurrence.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestKarrInvariantsAnnotatesAffineRecurrence(t *testing.T) {
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
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	coneMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	coneMust(t, err)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	coneMust(t, err)
	integer, err := manager.GetSort(zero)
	coneMust(t, err)
	name, err := manager.Symbols.Intern("karr-affine-p")
	coneMust(t, err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, rules.RegisterPredicate(predicate))
	factHead, err := manager.MkApp(predicate, []ast.ASTHandle{zero, two})
	coneMust(t, err)
	fact, err := rules.NewRule(factHead, nil, nil, "base")
	coneMust(t, err)
	coneMust(t, set.Add(fact))
	coneMust(t, fact.Close())
	variables := make([]ast.ASTHandle, 4)
	for i := range variables {
		variables[i], err = manager.MkVar(uint32(i), integer)
		coneMust(t, err)
	}
	tail, err := manager.MkApp(predicate, variables[:2])
	coneMust(t, err)
	head, err := manager.MkApp(predicate, variables[2:])
	coneMust(t, err)
	xSuccessor, err := manager.MkArithmeticAdd(variables[0], one)
	coneMust(t, err)
	xStep, err := manager.MkEq(variables[2], xSuccessor)
	coneMust(t, err)
	ySuccessor, err := manager.MkArithmeticAdd(variables[1], two)
	coneMust(t, err)
	yStep, err := manager.MkEq(variables[3], ySuccessor)
	coneMust(t, err)
	step, err := rules.NewRule(head, []ast.ASTHandle{tail, xStep, yStep}, nil, "step")
	coneMust(t, err)
	coneMust(t, set.Add(step))
	coneMust(t, step.Close())
	coneMust(t, set.SetOutputPredicate(predicate))
	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("Karr fixture is unstratified")
	}
	transformer := base.NewRuleTransformer()
	plugin := transforms.NewKarrInvariantsPlugin(manager, rules, 36010)
	coneMust(t, transformer.Register(plugin))
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	modified, err := transformer.Apply(set)
	coneMust(t, err)
	if !modified {
		t.Fatal("Karr transformer did not annotate the recurrence")
	}
	if matrix := plugin.ModelInvariants()[predicate]; matrix.Len() == 0 || len(matrix.A[0]) != 2 {
		t.Fatal("Karr model invariant was not projected back to the original predicate", matrix)
	}
	transformed, err := set.Rule(1)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transformed.Close()) })
	if tails, getErr := transformed.TailSize(); getErr != nil || tails <= 3 {
		t.Fatalf("Karr invariant was not appended: tails=%d err=%v", tails, getErr)
	}
	if declaration, getErr := transformed.HeadDeclaration(); getErr != nil || declaration != predicate {
		t.Fatal("loop-counter predicate was not reverted", declaration, getErr)
	}
}

func TestKarrInvariantsStopsAfterCanceledForwardAnalysis(t *testing.T) {
	manager, rules, set, declaration, application := karrSupportFixture(t)
	fact, err := rules.NewRule(application, nil, nil, "base")
	coneMust(t, err)
	recursive, err := rules.NewRule(application, []ast.ASTHandle{application}, nil, "step")
	coneMust(t, err)
	coneMust(t, set.Add(fact))
	coneMust(t, set.Add(recursive))
	coneMust(t, fact.Close())
	coneMust(t, recursive.Close())
	coneMust(t, set.SetOutputPredicate(declaration))
	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("Karr cancellation fixture is unstratified")
	}
	plugin := transforms.NewKarrInvariantsPlugin(manager, rules, 36010)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	manager.Limit.Cancel()
	result, err := plugin.Transform(set)
	coneMust(t, err)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("canceled forward analysis continued to reverse analysis")
	}
}

func TestKarrInvariantsSkipsNegatedPrograms(t *testing.T) {
	manager, rules, set, declaration, application := karrSupportFixture(t)
	rule, err := rules.NewRule(application, []ast.ASTHandle{application}, []bool{true}, "negative")
	coneMust(t, err)
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, set.SetOutputPredicate(declaration))
	plugin := transforms.NewKarrInvariantsPlugin(manager, rules, 36010)
	result, err := plugin.Transform(set)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("Karr transformer accepted negation")
	}
}

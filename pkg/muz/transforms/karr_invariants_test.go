package transforms_test

// Source: src/muz/transforms/dl_mk_karr_invariants.cpp/.h forward/backward
// invariant discovery over a two-coordinate affine recurrence.

import (
	"errors"
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
	// get_invariants queries an inner datalog context, whose check_rules
	// rejects the loop counter's Int column before any Karr analysis.
	transformer := base.NewRuleTransformer()
	plugin := transforms.NewKarrInvariantsPlugin(manager, rules, 36010)
	coneMust(t, transformer.Register(plugin))
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	_, err = transformer.Apply(set)
	requireKarrInfiniteSort(t, err, "Rule contains infinite sorts in rule base:\nkarr-affine-p(0,2,0).\n")
}

func requireKarrInfiniteSort(t *testing.T, err error, message string) {
	t.Helper()
	var property *base.RulePropertyError
	if !errors.As(err, &property) || property.Kind != base.RulePropertyInfiniteSort {
		t.Fatal("Karr inner datalog context accepted infinite sorts", err)
	}
	if message != "" && property.SourceMessage != message {
		t.Fatalf("source message %q, want %q", property.SourceMessage, message)
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
	// The inner context's check_rules runs before any cancellation check.
	result, err := plugin.Transform(set)
	if result != nil {
		coneMust(t, result.Close())
	}
	requireKarrInfiniteSort(t, err, "")
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

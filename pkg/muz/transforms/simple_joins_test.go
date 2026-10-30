package transforms_test

// Source: src/muz/rel/dl_mk_simple_joins.cpp planner-generated predicate names.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestSimpleJoinsUsesSourceSplitPredicateNames(t *testing.T) {
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
	applications := make([]ast.ASTHandle, 4)
	for index, name := range []string{"A", "B", "C", "Goal"} {
		declarationName, x := manager.Symbols.Intern("simple-name-" + name)
		coneMust(t, x)
		declaration, x := manager.MkFuncDecl(declarationName, nil, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, x)
		coneMust(t, rules.RegisterPredicate(declaration))
		applications[index], x = manager.MkApp(declaration, nil)
		coneMust(t, x)
	}
	rule, err := rules.NewRule(applications[3], applications[:3], nil, "three-tail")
	coneMust(t, err)
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	plugin := transforms.NewSimpleJoinsPlugin(manager, rules, 0)
	transformed, err := plugin.Transform(set)
	coneMust(t, err)
	if transformed == nil {
		t.Fatal("three-tail rule was not transformed")
	}
	t.Cleanup(func() {
		coneMust(t, transformed.Close())
		coneMust(t, plugin.Close())
	})
	fresh := plugin.FreshPredicates()
	if len(fresh) != 1 {
		t.Fatalf("fresh split predicates = %d, want 1", len(fresh))
	}
	name, err := manager.GetASTName(fresh[0])
	coneMust(t, err)
	text, err := manager.Symbols.String(name)
	coneMust(t, err)
	if !strings.HasPrefix(text, "simple-name-Goal!split!") {
		t.Fatalf("split predicate name %q does not retain the source parent and split suffix", text)
	}
}

func TestSimpleJoinsSelectsLowestEstimatedPair(t *testing.T) {
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
	coneMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	sorts := make([]ast.ASTHandle, 5)
	for i, width := range []uint32{1, 2, 3, 4, 5} {
		sorts[i], err = manager.BitVectorSort(width)
		coneMust(t, err)
	}
	var handles [5]ast.ASTHandle
	for i := range handles {
		handles[i], err = manager.MkVar(uint32(i), sorts[i])
		coneMust(t, err)
	}
	makePredicate := func(name string, domain []ast.ASTHandle, args []ast.ASTHandle) ast.ASTHandle {
		symbol, x := manager.Symbols.Intern("simple-cost-" + name)
		coneMust(t, x)
		boolean, x := manager.MkBoolSort()
		coneMust(t, x)
		declaration, x := manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, x)
		coneMust(t, rules.RegisterPredicate(declaration))
		application, x := manager.MkApp(declaration, args)
		coneMust(t, x)
		return application
	}
	a := makePredicate("A", sorts[:2], handles[0:2])
	b := makePredicate("B", []ast.ASTHandle{sorts[0], sorts[2]}, []ast.ASTHandle{handles[0], handles[2]})
	c := makePredicate("C", sorts[3:], handles[3:])
	head := makePredicate("Goal", sorts, handles[:])
	rule, err := rules.NewRule(head, []ast.ASTHandle{a, b, c}, nil, "estimated-pair")
	coneMust(t, err)
	formula, err := rule.ToFormula()
	coneMust(t, err)
	formulaHandle, err := manager.RefGet(formula)
	coneMust(t, err)
	asserted, err := manager.MkProofAsserted(formulaHandle)
	coneMust(t, err)
	coneMust(t, rule.SetProof(asserted))
	coneMust(t, manager.DelRef(&formula))
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	plugin := transforms.NewSimpleJoinsPlugin(manager, rules, 0)
	transformed, err := plugin.Transform(set)
	coneMust(t, err)
	if transformed == nil {
		t.Fatal("three-tail rule was not transformed")
	}
	t.Cleanup(func() {
		coneMust(t, transformed.Close())
		coneMust(t, plugin.Close())
	})
	main, err := transformed.Rule(0)
	coneMust(t, err)
	mainName, err := main.Name()
	coneMust(t, err)
	if _, found, proofErr := main.Proof(); proofErr != nil || !found {
		t.Fatalf("rewritten source rule lost its proof: found=%v err=%v", found, proofErr)
	}
	coneMust(t, main.Close())
	if mainName != "estimated-pair" {
		t.Fatalf("first transformed rule name %q, want rewritten source rule", mainName)
	}
	definition, err := transformed.Rule(1)
	coneMust(t, err)
	if _, found, proofErr := definition.Proof(); proofErr != nil || !found {
		t.Fatalf("introduced helper rule has no asserted proof: found=%v err=%v", found, proofErr)
	}
	tails := make([]string, 2)
	for i := range tails {
		declaration, x := definition.TailDeclaration(i)
		coneMust(t, x)
		name, x := manager.GetASTName(declaration)
		coneMust(t, x)
		tails[i], x = manager.Symbols.String(name)
		coneMust(t, x)
	}
	coneMust(t, definition.Close())
	if !strings.Contains(tails[0], "simple-cost-A") || !strings.Contains(tails[1], "simple-cost-B") {
		t.Fatalf("selected helper pair %v, want A and B", tails)
	}
}

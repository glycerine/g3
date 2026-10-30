package transforms_test

// Source: src/muz/rel/dl_mk_simple_joins.cpp planner-generated predicate names.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
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
	applications := make([]ast.ASTHandle, 5)
	for index, name := range []string{"A", "B", "C", "D", "Goal"} {
		declarationName, x := manager.Symbols.Intern("simple-name-" + name)
		coneMust(t, x)
		declaration, x := manager.MkFuncDecl(declarationName, nil, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, x)
		coneMust(t, rules.RegisterPredicate(declaration))
		applications[index], x = manager.MkApp(declaration, nil)
		coneMust(t, x)
	}
	withNegated := append(append([]ast.ASTHandle(nil), applications[:4]...), applications[0])
	negated := []bool{false, false, false, false, true}
	rule, err := rules.NewRule(applications[4], withNegated, negated, "four-tail")
	coneMust(t, err)
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	plugin := transforms.NewSimpleJoinsPlugin(manager, rules, 0)
	transformed, err := plugin.Transform(set)
	coneMust(t, err)
	if transformed == nil {
		t.Fatal("three-tail rule was not transformed")
	}
	main, err := transformed.Rule(0)
	coneMust(t, err)
	if tails, x := main.TailSize(); x != nil || tails != 3 {
		t.Fatalf("rewritten rule tail count %d, want two positives and one negated tail: %v", tails, x)
	}
	if positives, x := main.PositiveTailSize(); x != nil || positives != 2 {
		t.Fatalf("rewritten positive tail count %d, want 2: %v", positives, x)
	}
	lastTail, lastNegated, err := main.Tail(2)
	coneMust(t, err)
	if !lastNegated || lastTail != applications[0] {
		t.Fatalf("nonpositive tail after global joins = %v negated=%v, want original negated A", lastTail, lastNegated)
	}
	coneMust(t, main.Close())
	t.Cleanup(func() {
		coneMust(t, transformed.Close())
		coneMust(t, plugin.Close())
	})
	fresh := plugin.FreshPredicates()
	if len(fresh) != 2 {
		t.Fatalf("fresh split predicates = %d, want two staged binary joins", len(fresh))
	}
	for _, declaration := range fresh {
		name, x := manager.GetASTName(declaration)
		coneMust(t, x)
		text, x := manager.Symbols.String(name)
		coneMust(t, x)
		if !strings.HasPrefix(text, "simple-name-Goal!split!") {
			t.Fatalf("split predicate name %q does not retain the source parent and split suffix", text)
		}
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
		if x != nil {
			t.Fatalf("make predicate %s: %v", name, x)
		}
		return application
	}
	a := makePredicate("A", sorts[:2], handles[0:2])
	b := makePredicate("B", []ast.ASTHandle{sorts[0], sorts[2]}, []ast.ASTHandle{handles[0], handles[2]})
	c := makePredicate("C", sorts[3:], handles[3:])
	head := makePredicate("Goal", sorts, handles[:])
	relations, err := rel.NewRelationManager(manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, relations.Close()) })
	for _, fixture := range []struct {
		application ast.ASTHandle
		rows        int
		row         rel.Fact
	}{
		{a, 8, rel.Fact{0, 0}},
		{b, 1, rel.Fact{0, 0}},
		{c, 1, rel.Fact{0, 0}},
	} {
		declaration, x := manager.GetDecl(fixture.application)
		coneMust(t, x)
		table, x := relations.GetRelation(declaration)
		coneMust(t, x)
		for i := 0; i < fixture.rows; i++ {
			row := append(rel.Fact(nil), fixture.row...)
			row[0] = rel.Element(i % 2)
			row[1] = rel.Element(i / 2)
			_, x = table.Add(row)
			coneMust(t, x)
		}
		coneMust(t, relations.MarkSaturated(declaration))
	}
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
	var secondHandles [5]ast.ASTHandle
	secondPins := make([]ast.ASTRef, len(secondHandles))
	for i := range secondHandles {
		secondHandles[i], err = manager.MkVar(uint32(i+10), sorts[i])
		coneMust(t, err)
		secondPins[i], err = manager.NewRef(secondHandles[i])
		coneMust(t, err)
	}
	t.Cleanup(func() {
		for i := len(secondPins) - 1; i >= 0; i-- {
			coneMust(t, manager.DelRef(&secondPins[i]))
		}
	})
	a2 := makePredicate("A", sorts[:2], secondHandles[0:2])
	b2 := makePredicate("B", []ast.ASTHandle{sorts[0], sorts[2]}, []ast.ASTHandle{secondHandles[0], secondHandles[2]})
	c2 := makePredicate("C", sorts[3:], secondHandles[3:])
	for _, pair := range [][2]ast.ASTHandle{{a, a2}, {b, b2}, {c, c2}} {
		firstDecl, x := manager.GetDecl(pair[0])
		coneMust(t, x)
		secondDecl, x := manager.GetDecl(pair[1])
		coneMust(t, x)
		if firstDecl != secondDecl {
			t.Fatalf("same source predicate applications use distinct declarations")
		}
	}
	head2 := makePredicate("Goal", sorts, secondHandles[:])
	secondApps := []ast.ASTHandle{a2, b2, c2, head2}
	secondAppPins := make([]ast.ASTRef, len(secondApps))
	for i, application := range secondApps {
		secondAppPins[i], err = manager.NewRef(application)
		coneMust(t, err)
	}
	t.Cleanup(func() {
		for i := len(secondAppPins) - 1; i >= 0; i-- {
			coneMust(t, manager.DelRef(&secondAppPins[i]))
		}
	})
	rule2, err := rules.NewRule(head2, []ast.ASTHandle{a2, b2, c2}, nil, "renamed-estimated-pair")
	coneMust(t, err)
	coneMust(t, set.Add(rule2))
	for i, variable := range secondHandles {
		if kind, x := manager.GetKind(variable); x != nil || kind != ast.ASTVar {
			t.Fatalf("second variable %d lost before alternate head: kind=%v err=%v", i, kind, x)
		}
	}
	head3 := makePredicate("Goal2", []ast.ASTHandle{sorts[0], sorts[2], sorts[3], sorts[4]}, []ast.ASTHandle{secondHandles[0], secondHandles[2], secondHandles[3], secondHandles[4]})
	rule3, err := rules.NewRule(head3, []ast.ASTHandle{a2, b2, c2}, nil, "different-live-outputs")
	coneMust(t, err)
	coneMust(t, set.Add(rule3))
	coneMust(t, rule3.Close())
	coneMust(t, rule2.Close())
	plugin := transforms.NewSimpleJoinsPlugin(manager, rules, 0, relations)
	transformed, err := plugin.Transform(set)
	coneMust(t, err)
	if transformed == nil {
		t.Fatal("three-tail rule was not transformed")
	}
	if transformedCount, x := transformed.Len(); x != nil || transformedCount != 4 {
		t.Fatalf("renamed pair consumers yielded %d rules, want three parents and one shared pair helper: %v", transformedCount, x)
	}
	fresh := plugin.FreshPredicates()
	if len(fresh) != 1 {
		t.Fatalf("renamed equivalent pairs created %d helper predicates, want one selected pair group", len(fresh))
	} else {
		freshName, x := manager.GetASTName(fresh[0])
		coneMust(t, x)
		freshText, x := manager.Symbols.String(freshName)
		coneMust(t, x)
		if !strings.HasPrefix(freshText, "simple-cost-Goal2_and_2!split!") {
			t.Fatalf("shared helper name %q does not use the final consumer and pair registration count", freshText)
		}
		var sharedDecl ast.ASTHandle
		for index := 0; index < 3; index++ {
			parent, x := transformed.Rule(index)
			coneMust(t, x)
			var found ast.ASTHandle
			for tailIndex := 0; tailIndex < 2; tailIndex++ {
				declaration, y := parent.TailDeclaration(tailIndex)
				coneMust(t, y)
				for _, candidate := range fresh {
					if declaration == candidate {
						found = candidate
					}
				}
			}
			coneMust(t, parent.Close())
			if found.IsNull() {
				t.Fatalf("rewritten parent %d does not call the shared helper", index)
			}
			if index == 0 {
				sharedDecl = found
			} else if sharedDecl != found {
				t.Fatal("consumers of the selected pair did not share one helper")
			}
		}
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
	var definition *base.Rule
	for index := 3; index < 4; index++ {
		candidate, x := transformed.Rule(index)
		coneMust(t, x)
		pairNames := make([]string, 2)
		for i := range pairNames {
			declaration, y := candidate.TailDeclaration(i)
			coneMust(t, y)
			name, y := manager.GetASTName(declaration)
			coneMust(t, y)
			pairNames[i], y = manager.Symbols.String(name)
			coneMust(t, y)
		}
		if (strings.Contains(pairNames[0], "simple-cost-B") && strings.Contains(pairNames[1], "simple-cost-C")) || (strings.Contains(pairNames[0], "simple-cost-C") && strings.Contains(pairNames[1], "simple-cost-B")) {
			definition = candidate
		} else {
			coneMust(t, candidate.Close())
		}
	}
	if definition == nil {
		t.Fatal("no helper definition for the selected B/C pair")
	}
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
	if !(strings.Contains(tails[0], "simple-cost-B") && strings.Contains(tails[1], "simple-cost-C") || strings.Contains(tails[0], "simple-cost-C") && strings.Contains(tails[1], "simple-cost-B")) {
		t.Fatalf("selected helper pair %v, want saturated low-cardinality B and C", tails)
	}
	plain := transforms.NewSimpleJoinsPlugin(manager, rules, 0)
	plainResult, err := plain.Transform(set)
	coneMust(t, err)
	if plainResult == nil {
		t.Fatal("unsaturated three-tail rule was not transformed")
	}
	t.Cleanup(func() {
		coneMust(t, plainResult.Close())
		coneMust(t, plain.Close())
	})
	if plainCount, x := plainResult.Len(); x != nil || plainCount != 4 {
		t.Fatalf("unsaturated renamed pair consumers yielded %d rules, want three parents and one shared helper: %v", plainCount, x)
	}
	if fresh := plain.FreshPredicates(); len(fresh) != 1 {
		t.Fatalf("unsaturated renamed equivalent pairs created %d helpers, want one", len(fresh))
	} else if arity, x := manager.GetArity(fresh[0]); x != nil || arity != 3 {
		t.Fatalf("shared helper arity = %d, want union of three output variables: %v", arity, x)
	}
	plainHelper, err := plainResult.Rule(3)
	coneMust(t, err)
	plainTails := make([]string, 2)
	for i := range plainTails {
		declaration, x := plainHelper.TailDeclaration(i)
		coneMust(t, x)
		name, x := manager.GetASTName(declaration)
		coneMust(t, x)
		plainTails[i], x = manager.Symbols.String(name)
		coneMust(t, x)
	}
	coneMust(t, plainHelper.Close())
	if !(strings.Contains(plainTails[0], "simple-cost-A") && strings.Contains(plainTails[1], "simple-cost-B") || strings.Contains(plainTails[0], "simple-cost-B") && strings.Contains(plainTails[1], "simple-cost-A")) {
		t.Fatalf("unsaturated estimate selected %v, want A and B", plainTails)
	}
}

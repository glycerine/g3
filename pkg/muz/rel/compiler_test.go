package rel_test

// Source: src/muz/rel/dl_compiler.cpp retained finite ground and identity-rule
// compilation path.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func compilerDeclaration(t *testing.T, manager *ast.ASTManager, name string, domain []ast.ASTHandle, boolean ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	symbol, err := manager.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	return declaration
}

func compilerRule(t *testing.T, rules *base.RuleManager, set *base.RuleSet, head ast.ASTHandle, tail []ast.ASTHandle, name string) {
	t.Helper()
	rule, err := rules.NewRule(head, tail, nil, name)
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(rule); err != nil {
		t.Fatal(err)
	}
	if err = rule.Close(); err != nil {
		t.Fatal(err)
	}
}

func compilerWantFacts(t *testing.T, table *rel.HashTable, expected ...rel.Fact) {
	t.Helper()
	count, err := table.Len()
	if err != nil || count != len(expected) {
		t.Fatal("compiled fact count", count, expected, err)
	}
	for _, fact := range expected {
		found, x := table.Contains(fact)
		if x != nil || !found {
			t.Fatal("missing compiled fact", fact, x)
		}
	}
}

func TestFiniteCompilerOrdersFactsAndIdentityRules(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Close() })
	manager, err := command.Manager()
	if err != nil {
		t.Fatal(err)
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relations.Close() })
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rules.Close() })
	set, err := base.NewRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = set.Close() })
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	p := compilerDeclaration(t, manager, "compiled-P", []ast.ASTHandle{boolean, boolean}, boolean)
	q := compilerDeclaration(t, manager, "compiled-Q", []ast.ASTHandle{boolean, boolean}, boolean)
	r := compilerDeclaration(t, manager, "compiled-R", []ast.ASTHandle{boolean}, boolean)
	s := compilerDeclaration(t, manager, "compiled-S", nil, boolean)
	z := compilerDeclaration(t, manager, "compiled-Z", nil, boolean)
	for _, predicate := range []ast.ASTHandle{p, q, r, s, z} {
		if err = rules.RegisterPredicate(predicate); err != nil {
			t.Fatal(err)
		}
	}
	variable0, err := manager.MkVar(0, boolean)
	if err != nil {
		t.Fatal(err)
	}
	variable1, err := manager.MkVar(1, boolean)
	if err != nil {
		t.Fatal(err)
	}
	pVariables, err := manager.MkApp(p, []ast.ASTHandle{variable0, variable1})
	if err != nil {
		t.Fatal(err)
	}
	qVariables, err := manager.MkApp(q, []ast.ASTHandle{variable0, variable1})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, qVariables, []ast.ASTHandle{pVariables}, "identity-first")
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	falsity, err := manager.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	pFact, err := manager.MkApp(p, []ast.ASTHandle{truth, falsity})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, pFact, nil, "fact-after-dependent")
	rHead, err := manager.MkApp(r, []ast.ASTHandle{variable0})
	if err != nil {
		t.Fatal(err)
	}
	rTail, err := manager.MkApp(p, []ast.ASTHandle{variable0, falsity})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, rHead, []ast.ASTHandle{rTail}, "select-tail-constant")
	sFact, err := manager.MkApp(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	sTail, err := manager.MkApp(p, []ast.ASTHandle{truth, falsity})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, sFact, []ast.ASTHandle{sTail}, "project-selected-tail")
	zFact, err := manager.MkApp(z, nil)
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, zFact, nil, "zero-arity-fact")
	valid, err := set.Finalize()
	if err != nil || !valid {
		t.Fatal("finalize", valid, err)
	}
	execution, termination, err := rel.CompileFiniteRules(relations, set)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = execution.Close(); _ = termination.Close() })
	context, err := rel.NewExecutionContextWithRelations(relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	complete, err := execution.Perform(context)
	if err != nil || !complete {
		t.Fatal("compiled execution", complete, err)
	}
	complete, err = termination.Perform(context)
	if err != nil || !complete {
		t.Fatal("compiled termination", complete, err)
	}
	for _, predicate := range []ast.ASTHandle{p, q} {
		table, found, x := relations.TryGetRelation(predicate)
		if x != nil || !found || table == nil {
			t.Fatal("compiled predicate relation", predicate, table, found, x)
		}
		facts, x := table.Facts()
		if x != nil || len(facts) != 1 || len(facts[0]) != 2 || facts[0][0] != 1 || facts[0][1] != 0 {
			t.Fatal("compiled facts", predicate, facts, x)
		}
	}
	rTable, found, err := relations.TryGetRelation(r)
	if err != nil || !found || rTable == nil {
		t.Fatal("selected compiled relation", rTable, found, err)
	}
	rFacts, err := rTable.Facts()
	if err != nil || len(rFacts) != 1 || len(rFacts[0]) != 1 || rFacts[0][0] != 1 {
		t.Fatal("selected compiled facts", rFacts, err)
	}
	sTable, found, err := relations.TryGetRelation(s)
	if err != nil || !found || sTable == nil {
		t.Fatal("projected compiled relation", sTable, found, err)
	}
	sFacts, err := sTable.Facts()
	if err != nil || len(sFacts) != 1 || len(sFacts[0]) != 0 {
		t.Fatal("projected compiled fact", sFacts, err)
	}
	zTable, found, err := relations.TryGetRelation(z)
	if err != nil || !found || zTable == nil {
		t.Fatal("zero-arity compiled relation", zTable, found, err)
	}
	zFacts, err := zTable.Facts()
	if err != nil || len(zFacts) != 1 || len(zFacts[0]) != 0 {
		t.Fatal("zero-arity compiled fact", zFacts, err)
	}
}

func TestFiniteCompilerRejectsRicherRulesWithoutPartialExecution(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Close() })
	manager, err := command.Manager()
	if err != nil {
		t.Fatal(err)
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relations.Close() })
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rules.Close() })
	set, err := base.NewRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = set.Close() })
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	p := compilerDeclaration(t, manager, "unsupported-P", []ast.ASTHandle{boolean}, boolean)
	if err = rules.RegisterPredicate(p); err != nil {
		t.Fatal(err)
	}
	variable, err := manager.MkVar(0, boolean)
	if err != nil {
		t.Fatal(err)
	}
	head, err := manager.MkApp(p, []ast.ASTHandle{variable})
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, head, []ast.ASTHandle{truth}, "interpreted-tail")
	if _, _, err = rel.CompileFiniteRules(relations, set); !errors.Is(err, rel.ErrCompilerUnsupported) {
		t.Fatal("unsupported rule was not rejected", err)
	}
}

func TestFiniteCompilerAssemblesRepeatedPermutedAndUnboundHeads(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Close() })
	manager, err := command.Manager()
	if err != nil {
		t.Fatal(err)
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relations.Close() })
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rules.Close() })
	set, err := base.NewRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = set.Close() })
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	domain2 := []ast.ASTHandle{boolean, boolean}
	predicates := make(map[string]ast.ASTHandle)
	for _, name := range []string{"P", "Swap", "Dup", "Diagonal", "Const", "All", "Equal"} {
		domain := domain2
		if name == "Diagonal" {
			domain = []ast.ASTHandle{boolean}
		}
		predicates[name] = compilerDeclaration(t, manager, "assembly-"+name, domain, boolean)
		if err = rules.RegisterPredicate(predicates[name]); err != nil {
			t.Fatal(err)
		}
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	falsity, err := manager.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	for index, values := range [][]ast.ASTHandle{{truth, falsity}, {falsity, truth}, {truth, truth}} {
		head, x := manager.MkApp(predicates["P"], values)
		if x != nil {
			t.Fatal(x)
		}
		compilerRule(t, rules, set, head, nil, "assembly-fact-"+string(rune('0'+index)))
	}
	x, err := manager.MkVar(0, boolean)
	if err != nil {
		t.Fatal(err)
	}
	y, err := manager.MkVar(1, boolean)
	if err != nil {
		t.Fatal(err)
	}
	pXY, err := manager.MkApp(predicates["P"], []ast.ASTHandle{x, y})
	if err != nil {
		t.Fatal(err)
	}
	addSingle := func(name string, arguments []ast.ASTHandle, tail ast.ASTHandle) {
		t.Helper()
		head, x := manager.MkApp(predicates[name], arguments)
		if x != nil {
			t.Fatal(x)
		}
		compilerRule(t, rules, set, head, []ast.ASTHandle{tail}, "assembly-"+name)
	}
	addSingle("Swap", []ast.ASTHandle{y, x}, pXY)
	addSingle("Dup", []ast.ASTHandle{x, x}, pXY)
	pXX, err := manager.MkApp(predicates["P"], []ast.ASTHandle{x, x})
	if err != nil {
		t.Fatal(err)
	}
	addSingle("Diagonal", []ast.ASTHandle{x}, pXX)
	addSingle("Const", []ast.ASTHandle{falsity, x}, pXY)
	for _, name := range []string{"All", "Equal"} {
		arguments := []ast.ASTHandle{x, y}
		if name == "Equal" {
			arguments = []ast.ASTHandle{x, x}
		}
		head, x := manager.MkApp(predicates[name], arguments)
		if x != nil {
			t.Fatal(x)
		}
		compilerRule(t, rules, set, head, nil, "assembly-"+name)
	}
	valid, err := set.Finalize()
	if err != nil || !valid {
		t.Fatal("finalize", valid, err)
	}
	execution, termination, err := rel.CompileFiniteRules(relations, set)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = execution.Close(); _ = termination.Close() })
	context, err := rel.NewExecutionContextWithRelations(relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	complete, err := execution.Perform(context)
	if err != nil || !complete {
		t.Fatal("assembly execution", complete, err)
	}
	complete, err = termination.Perform(context)
	if err != nil || !complete {
		t.Fatal("assembly termination", complete, err)
	}
	table := func(name string) *rel.HashTable {
		t.Helper()
		value, found, x := relations.TryGetRelation(predicates[name])
		if x != nil || !found || value == nil {
			t.Fatal("missing assembly relation", name, value, found, x)
		}
		return value
	}
	compilerWantFacts(t, table("Swap"), rel.Fact{0, 1}, rel.Fact{1, 0}, rel.Fact{1, 1})
	compilerWantFacts(t, table("Dup"), rel.Fact{1, 1}, rel.Fact{0, 0})
	compilerWantFacts(t, table("Diagonal"), rel.Fact{1})
	compilerWantFacts(t, table("Const"), rel.Fact{0, 1}, rel.Fact{0, 0})
	compilerWantFacts(t, table("All"), rel.Fact{0, 0}, rel.Fact{0, 1}, rel.Fact{1, 0}, rel.Fact{1, 1})
	compilerWantFacts(t, table("Equal"), rel.Fact{0, 0}, rel.Fact{1, 1})
}

func TestFiniteCompilerJoinsTwoPositiveTails(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Close() })
	manager, err := command.Manager()
	if err != nil {
		t.Fatal(err)
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relations.Close() })
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rules.Close() })
	set, err := base.NewRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = set.Close() })
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	domain := []ast.ASTHandle{boolean, boolean}
	p := compilerDeclaration(t, manager, "join-P", domain, boolean)
	q := compilerDeclaration(t, manager, "join-Q", domain, boolean)
	r := compilerDeclaration(t, manager, "join-R", domain, boolean)
	for _, predicate := range []ast.ASTHandle{p, q, r} {
		if err = rules.RegisterPredicate(predicate); err != nil {
			t.Fatal(err)
		}
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	falsity, err := manager.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	addFact := func(predicate ast.ASTHandle, values []ast.ASTHandle, name string) {
		t.Helper()
		head, x := manager.MkApp(predicate, values)
		if x != nil {
			t.Fatal(x)
		}
		compilerRule(t, rules, set, head, nil, name)
	}
	addFact(p, []ast.ASTHandle{truth, falsity}, "join-p-10")
	addFact(p, []ast.ASTHandle{falsity, truth}, "join-p-01")
	addFact(p, []ast.ASTHandle{truth, truth}, "join-p-11")
	addFact(q, []ast.ASTHandle{falsity, truth}, "join-q-01")
	addFact(q, []ast.ASTHandle{truth, falsity}, "join-q-10")
	x, err := manager.MkVar(0, boolean)
	if err != nil {
		t.Fatal(err)
	}
	y, err := manager.MkVar(1, boolean)
	if err != nil {
		t.Fatal(err)
	}
	z, err := manager.MkVar(2, boolean)
	if err != nil {
		t.Fatal(err)
	}
	pXY, err := manager.MkApp(p, []ast.ASTHandle{x, y})
	if err != nil {
		t.Fatal(err)
	}
	qYZ, err := manager.MkApp(q, []ast.ASTHandle{y, z})
	if err != nil {
		t.Fatal(err)
	}
	rZX, err := manager.MkApp(r, []ast.ASTHandle{z, x})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, rZX, []ast.ASTHandle{pXY, qYZ}, "join-shared-variable")
	valid, err := set.Finalize()
	if err != nil || !valid {
		t.Fatal("finalize", valid, err)
	}
	execution, termination, err := rel.CompileFiniteRules(relations, set)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = execution.Close(); _ = termination.Close() })
	context, err := rel.NewExecutionContextWithRelations(relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	complete, err := execution.Perform(context)
	if err != nil || !complete {
		t.Fatal("join execution", complete, err)
	}
	complete, err = termination.Perform(context)
	if err != nil || !complete {
		t.Fatal("join termination", complete, err)
	}
	table, found, err := relations.TryGetRelation(r)
	if err != nil || !found || table == nil {
		t.Fatal("joined relation", table, found, err)
	}
	compilerWantFacts(t, table, rel.Fact{1, 1}, rel.Fact{0, 0}, rel.Fact{0, 1})
}

func TestFiniteCompilerSaturatesRecursiveRules(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Close() })
	manager, err := command.Manager()
	if err != nil {
		t.Fatal(err)
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relations.Close() })
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rules.Close() })
	set, err := base.NewRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = set.Close() })
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	domain := []ast.ASTHandle{boolean, boolean}
	edge := compilerDeclaration(t, manager, "recursive-Edge", domain, boolean)
	path := compilerDeclaration(t, manager, "recursive-Path", domain, boolean)
	mutualA := compilerDeclaration(t, manager, "recursive-MutualA", []ast.ASTHandle{boolean}, boolean)
	mutualB := compilerDeclaration(t, manager, "recursive-MutualB", []ast.ASTHandle{boolean}, boolean)
	allowed := compilerDeclaration(t, manager, "recursive-Allowed", []ast.ASTHandle{boolean}, boolean)
	blocked := compilerDeclaration(t, manager, "recursive-Blocked", []ast.ASTHandle{boolean}, boolean)
	accepted := compilerDeclaration(t, manager, "recursive-Accepted", []ast.ASTHandle{boolean}, boolean)
	for _, predicate := range []ast.ASTHandle{edge, path, mutualA, mutualB, allowed, blocked, accepted} {
		if err = rules.RegisterPredicate(predicate); err != nil {
			t.Fatal(err)
		}
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	falsity, err := manager.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	for index, values := range [][]ast.ASTHandle{{truth, falsity}, {falsity, truth}} {
		fact, x := manager.MkApp(edge, values)
		if x != nil {
			t.Fatal(x)
		}
		compilerRule(t, rules, set, fact, nil, "recursive-edge-"+string(rune('0'+index)))
	}
	mutualSeed, err := manager.MkApp(mutualA, []ast.ASTHandle{truth})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, mutualSeed, nil, "recursive-mutual-seed")
	allowedTrue, err := manager.MkApp(allowed, []ast.ASTHandle{truth})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, allowedTrue, nil, "recursive-allowed-true")
	allowedFalse, err := manager.MkApp(allowed, []ast.ASTHandle{falsity})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, allowedFalse, nil, "recursive-allowed-false")
	blockedFalse, err := manager.MkApp(blocked, []ast.ASTHandle{falsity})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, blockedFalse, nil, "recursive-blocked-false")
	acceptedVariable, err := manager.MkVar(4, boolean)
	if err != nil {
		t.Fatal(err)
	}
	acceptedVariableRef, err := manager.NewRef(acceptedVariable)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.DelRef(&acceptedVariableRef) })
	acceptedHead, err := manager.MkApp(accepted, []ast.ASTHandle{acceptedVariable})
	if err != nil {
		t.Fatal(err)
	}
	allowedTail, err := manager.MkApp(allowed, []ast.ASTHandle{acceptedVariable})
	if err != nil {
		t.Fatal(err)
	}
	blockedTail, err := manager.MkApp(blocked, []ast.ASTHandle{acceptedVariable})
	if err != nil {
		t.Fatal(err)
	}
	negativeRule, err := rules.NewRule(acceptedHead, []ast.ASTHandle{allowedTail, blockedTail}, []bool{false, true}, "recursive-stratified-negation")
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(negativeRule); err != nil {
		t.Fatal(err)
	}
	if err = negativeRule.Close(); err != nil {
		t.Fatal(err)
	}
	x, err := manager.MkVar(0, boolean)
	if err != nil {
		t.Fatal(err)
	}
	y, err := manager.MkVar(1, boolean)
	if err != nil {
		t.Fatal(err)
	}
	z, err := manager.MkVar(2, boolean)
	if err != nil {
		t.Fatal(err)
	}
	edgeXY, err := manager.MkApp(edge, []ast.ASTHandle{x, y})
	if err != nil {
		t.Fatal(err)
	}
	pathXY, err := manager.MkApp(path, []ast.ASTHandle{x, y})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, pathXY, []ast.ASTHandle{edgeXY}, "recursive-seed")
	pathXZ, err := manager.MkApp(path, []ast.ASTHandle{x, z})
	if err != nil {
		t.Fatal(err)
	}
	pathXY, err = manager.MkApp(path, []ast.ASTHandle{x, y})
	if err != nil {
		t.Fatal(err)
	}
	edgeYZ, err := manager.MkApp(edge, []ast.ASTHandle{y, z})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, pathXZ, []ast.ASTHandle{pathXY, edgeYZ}, "recursive-step")
	mutualVariable, err := manager.MkVar(3, boolean)
	if err != nil {
		t.Fatal(err)
	}
	mutualVariableRef, err := manager.NewRef(mutualVariable)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.DelRef(&mutualVariableRef) })
	mutualAFromB, err := manager.MkApp(mutualA, []ast.ASTHandle{mutualVariable})
	if err != nil {
		t.Fatal(err)
	}
	mutualBFromA, err := manager.MkApp(mutualB, []ast.ASTHandle{mutualVariable})
	if err != nil {
		t.Fatal(err)
	}
	compilerRule(t, rules, set, mutualBFromA, []ast.ASTHandle{mutualAFromB}, "recursive-mutual-forward")
	mutualAHead, err := manager.MkApp(mutualA, []ast.ASTHandle{mutualVariable})
	if err != nil {
		t.Fatal(err)
	}
	mutualBTail, err := manager.MkApp(mutualB, []ast.ASTHandle{mutualVariable})
	if err != nil {
		t.Fatal(err)
	}
	mutualBackward, err := rules.NewRule(mutualAHead, []ast.ASTHandle{mutualBTail}, nil, "recursive-mutual-backward")
	if err != nil {
		t.Fatal("construct mutually recursive backward rule", err)
	}
	if err = set.Add(mutualBackward); err != nil {
		t.Fatal("add mutually recursive backward rule", err)
	}
	if err = mutualBackward.Close(); err != nil {
		t.Fatal("close mutually recursive backward rule", err)
	}
	valid, err := set.Finalize()
	if err != nil || !valid {
		t.Fatal("finalize", valid, err)
	}
	execution, termination, err := rel.CompileFiniteRules(relations, set)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = execution.Close(); _ = termination.Close() })
	context, err := rel.NewExecutionContextWithRelations(relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	complete, err := execution.Perform(context)
	if err != nil || !complete {
		t.Fatal("recursive execution", complete, err)
	}
	complete, err = termination.Perform(context)
	if err != nil || !complete {
		t.Fatal("recursive termination", complete, err)
	}
	for _, predicate := range []ast.ASTHandle{edge, path} {
		saturated, x := relations.IsSaturated(predicate)
		if x != nil || !saturated {
			t.Fatal("recursive predicate not marked saturated", predicate, saturated, x)
		}
	}
	for _, predicate := range []ast.ASTHandle{mutualA, mutualB} {
		table, found, x := relations.TryGetRelation(predicate)
		if x != nil || !found || table == nil {
			t.Fatal("mutually recursive relation", predicate, table, found, x)
		}
		compilerWantFacts(t, table, rel.Fact{1})
	}
	acceptedTable, found, err := relations.TryGetRelation(accepted)
	if err != nil || !found || acceptedTable == nil {
		t.Fatal("stratified negation relation", acceptedTable, found, err)
	}
	compilerWantFacts(t, acceptedTable, rel.Fact{1})
	table, found, err := relations.TryGetRelation(path)
	if err != nil || !found || table == nil {
		t.Fatal("recursive path relation", table, found, err)
	}
	compilerWantFacts(t, table, rel.Fact{0, 0}, rel.Fact{0, 1}, rel.Fact{1, 0}, rel.Fact{1, 1})
}

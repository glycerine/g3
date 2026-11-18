package pdr

// Source: focused formula-shape, level-boundary, and adoption checks for
// core_induction_generalizer in src/muz/pdr/pdr_generalizers.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

const (
	inductionTestEmpty = iota
	inductionTestFact
	inductionTestRecursive
	inductionTestThreeRules
)

func inductionTestTransformer(t *testing.T, mode int) (*ast.ASTManager, *Manager, *PredicateTransformer, ast.ASTHandle, func()) {
	t.Helper()
	manager, owner, transformer, term, closeBase := closureFixture(t)
	if mode == inductionTestEmpty {
		closureMust(t, transformer.InitializeRules(map[ast.ASTHandle]*PredicateTransformer{transformer.head: transformer}, nil))
		return manager, owner, transformer, term, closeBase
	}
	rules, err := base.NewRuleManager(manager)
	closureMust(t, err)
	closureMust(t, rules.RegisterPredicate(transformer.head))
	variableSort, err := manager.GetSort(term)
	closureMust(t, err)
	variable, err := manager.MkVar(0, variableSort)
	closureMust(t, err)
	head, err := manager.MkApp(transformer.head, []ast.ASTHandle{variable})
	closureMust(t, err)
	var tail []ast.ASTHandle
	if mode == inductionTestRecursive {
		tail = []ast.ASTHandle{head}
	}
	count := 1
	if mode == inductionTestThreeRules {
		count = 3
	}
	var made []*base.Rule
	for i := 0; i < count; i++ {
		rule, err := rules.NewRule(head, tail, nil, "induction_rule")
		closureMust(t, err)
		made = append(made, rule)
	}
	closureMust(t, transformer.InitializeRules(map[ast.ASTHandle]*PredicateTransformer{transformer.head: transformer}, made))
	for _, rule := range made {
		closureMust(t, rule.Close())
	}
	closureMust(t, rules.Close())
	return manager, owner, transformer, term, closeBase
}

func inductionCloseFixture(t *testing.T, owner *Manager, transformer *PredicateTransformer, closeBase func()) {
	t.Helper()
	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	closeBase()
}

func inductionBasicOperation(t *testing.T, manager *ast.ASTManager, expression ast.ASTHandle, operator int32) ast.AppArguments {
	t.Helper()
	family, err := manager.GetFamilyID(expression)
	closureMust(t, err)
	kind, err := manager.GetDeclKind(expression)
	closureMust(t, err)
	if family != manager.BasicFamilyID() || kind != operator {
		t.Fatalf("basic operation got family=%d op=%d want family=%d op=%d", family, kind, manager.BasicFamilyID(), operator)
	}
	arguments, err := manager.AppArgumentView(expression)
	closureMust(t, err)
	return arguments
}

func TestInductionGeneralizerTransitionRuleLevelZeroAndQuantifierShape(t *testing.T) {
	manager, owner, transformer, _, closeBase := inductionTestTransformer(t, inductionTestFact)
	builder, err := newInductionGeneralizerBuilder(transformer)
	closureMust(t, err)
	representatives, err := builder.representatives(transformer)
	closureMust(t, err)
	transition, err := builder.transitionRule(transformer, 0, transformer.rules[0], representatives)
	closureMust(t, err)
	if !manager.IsTrue(transition) {
		t.Fatal("fact transition at level zero", transition)
	}
	axiom, err := builder.transitionAxiom(transformer, 1)
	closureMust(t, err)
	kind, err := manager.GetKind(axiom)
	closureMust(t, err)
	if kind != ast.ASTQuantifier {
		t.Fatal("transition axiom did not bind head representatives", kind)
	}
	forall, err := manager.IsForall(axiom)
	closureMust(t, err)
	if !forall {
		t.Fatal("transition axiom is not universal")
	}
	closureMust(t, builder.Close())
	inductionCloseFixture(t, owner, transformer, closeBase)
}

func TestInductionGeneralizerRecursiveRuleLevelZeroAndQueueOrdering(t *testing.T) {
	manager, owner, transformer, _, closeBase := inductionTestTransformer(t, inductionTestRecursive)
	builder, err := newInductionGeneralizerBuilder(transformer)
	closureMust(t, err)
	representatives, err := builder.representatives(transformer)
	closureMust(t, err)
	zero, err := builder.transitionRule(transformer, 0, transformer.rules[0], representatives)
	closureMust(t, err)
	if !manager.IsFalse(zero) {
		t.Fatal("recursive transition at level zero", zero)
	}
	one, err := builder.transitionRule(transformer, 1, transformer.rules[0], representatives)
	closureMust(t, err)
	declaration, err := manager.GetDecl(one)
	closureMust(t, err)
	wantDeclaration, err := builder.predicate(0, transformer)
	closureMust(t, err)
	if declaration != wantDeclaration {
		t.Fatal("recursive tail was not staged at the preceding level", declaration, wantDeclaration)
	}
	goal, err := builder.goal(transformer, 2, inductionGeneralizerDepth)
	closureMust(t, err)
	arguments := inductionBasicOperation(t, manager, goal, ast.ASTOpAnd)
	// Target, one nonzero induction hypothesis, then axiom/property pairs
	// for the dynamically queued levels 2, 1, and 0.
	if arguments.Count() != 8 {
		t.Fatal("source induction-goal ordering/queue size", arguments.Count())
	}
	closureMust(t, builder.Close())
	inductionCloseFixture(t, owner, transformer, closeBase)
}

func TestInductionGeneralizerLevelOnePreservesUnsignedUnderflowBoundary(t *testing.T) {
	manager, owner, transformer, _, closeBase := inductionTestTransformer(t, inductionTestEmpty)
	builder, err := newInductionGeneralizerBuilder(transformer)
	closureMust(t, err)
	goal, err := builder.goal(transformer, 1, inductionGeneralizerDepth)
	closureMust(t, err)
	if goal.IsNull() {
		t.Fatal("level-one source boundary did not produce a goal")
	}
	arguments := inductionBasicOperation(t, manager, goal, ast.ASTOpAnd)
	if arguments.Count() != 3 {
		t.Fatal("underflow boundary added induction hypotheses", arguments.Count())
	}
	closureMust(t, builder.Close())
	inductionCloseFixture(t, owner, transformer, closeBase)
}

func TestInductionGeneralizerUnsatAdoptsBlockedTransitionAndOwnsIt(t *testing.T) {
	manager, owner, transformer, term, closeBase := inductionTestTransformer(t, inductionTestEmpty)
	parent, err := NewModelNode(nil, term, transformer, 2)
	closureMust(t, err)
	child, err := NewModelNode(parent, term, transformer, 1)
	closureMust(t, err)
	truth, err := manager.MkTrue()
	closureMust(t, err)
	input, err := pinGeneralizedCore(manager, []ast.ASTHandle{truth}, false)
	closureMust(t, err)
	result, err := generalizeInductionCore(child, input)
	closureMust(t, err)
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{input}))
	if len(result.literals) != 1 || !result.usesLevel {
		t.Fatal("induction replacement", result.literals, result.usesLevel)
	}
	negated, blocked, err := manager.IsNot(result.literals[0])
	closureMust(t, err)
	if !negated || !manager.IsTrue(blocked) {
		t.Fatal("empty-rule blocked transition", negated, blocked)
	}
	if _, err = manager.GetKind(result.literals[0]); err != nil {
		t.Fatal("replacement was not independently owned", err)
	}
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{result}))
	parent.children = nil
	closureMust(t, child.release())
	closureMust(t, parent.release())
	inductionCloseFixture(t, owner, transformer, closeBase)
}

func TestInductionGeneralizerRootFallsBackWithIndependentOwnership(t *testing.T) {
	manager, owner, transformer, term, closeBase := inductionTestTransformer(t, inductionTestFact)
	parent, err := NewModelNode(nil, term, transformer, 2)
	closureMust(t, err)
	truth, err := manager.MkTrue()
	closureMust(t, err)
	input, err := pinGeneralizedCore(manager, []ast.ASTHandle{truth}, false)
	closureMust(t, err)
	rootFallback, err := generalizeInductionCore(parent, input)
	closureMust(t, err)
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{input}))
	if len(rootFallback.literals) != 1 || rootFallback.literals[0] != truth || rootFallback.usesLevel {
		t.Fatal("root fallback", rootFallback.literals, rootFallback.usesLevel)
	}
	if _, err = manager.GetKind(rootFallback.literals[0]); err != nil {
		t.Fatal("fallback was not independently owned", err)
	}
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{rootFallback}))
	closureMust(t, parent.release())
	inductionCloseFixture(t, owner, transformer, closeBase)
}

// mk_transition_axiom: fml = (i == 0)? tr : m.mk_or(fml, tr) nests binary
// ors to the left (AUDIT muz_pdr_01-F7).
func TestInductionGeneralizerTransitionAxiomNestsOr(t *testing.T) {
	manager, owner, transformer, _, closeBase := inductionTestTransformer(t, inductionTestThreeRules)
	if len(transformer.rules) != 3 {
		t.Fatal("three-rule fixture", len(transformer.rules))
	}
	builder, err := newInductionGeneralizerBuilder(transformer)
	closureMust(t, err)
	axiom, err := builder.transitionAxiom(transformer, 1)
	closureMust(t, err)
	body, err := manager.GetQuantifierBody(axiom)
	closureMust(t, err)
	right, err := manager.GetArg(body, 1)
	closureMust(t, err)
	outer := inductionBasicOperation(t, manager, right, ast.ASTOpOr)
	if outer.Count() != 2 {
		t.Fatal("outer or arity", outer.Count())
	}
	left, err := outer.At(0)
	closureMust(t, err)
	if inner := inductionBasicOperation(t, manager, left, ast.ASTOpOr); inner.Count() != 2 {
		t.Fatal("inner or arity", inner.Count())
	}
	closureMust(t, builder.Close())
	inductionCloseFixture(t, owner, transformer, closeBase)
}

// core_generalizer's new_cores wrapper generalizes only a non-empty core:
// an empty one stays empty and keeps uses_level (AUDIT muz_pdr_01-F6).
func TestInductionGeneralizerSkipsEmptyCore(t *testing.T) {
	manager, owner, transformer, term, closeBase := inductionTestTransformer(t, inductionTestEmpty)
	parent, err := NewModelNode(nil, term, transformer, 2)
	closureMust(t, err)
	child, err := NewModelNode(parent, term, transformer, 1)
	closureMust(t, err)
	input, err := pinGeneralizedCore(manager, nil, false)
	closureMust(t, err)
	result, err := generalizeInductionCore(child, input)
	closureMust(t, err)
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{input}))
	if len(result.literals) != 0 || result.usesLevel {
		t.Fatal("empty core was generalized", result.literals, result.usesLevel)
	}
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{result}))
	parent.children = nil
	closureMust(t, child.release())
	closureMust(t, parent.release())
	inductionCloseFixture(t, owner, transformer, closeBase)
}

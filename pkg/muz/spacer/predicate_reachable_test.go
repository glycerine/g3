package spacer_test

// Source: src/muz/spacer/spacer_context.cpp pred_transformer::get_reachable.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/spacer"
)

func TestSpacerReachableSimplifiesEachRenamedFactBeforeDisjunction(t *testing.T) {
	command, m, owner := lemmaOwnerFixture(t)
	defer func() { spacerMust(t, command.Close()) }()
	defer func() { spacerMust(t, owner.Close()) }()
	boolean, err := m.MkBoolSort()
	spacerMust(t, err)
	symbol, err := m.Symbols.Intern("reachable-rewrite-p")
	spacerMust(t, err)
	predicate, err := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	pt, err := spacer.NewRulePredicateTransformer(owner, predicate, spacer.PropSolverConfig{})
	spacerMust(t, err)
	defer func() { spacerMust(t, pt.Close()) }()
	rules, err := base.NewRuleManager(m)
	spacerMust(t, err)
	defer func() { spacerMust(t, rules.Close()) }()
	spacerMust(t, rules.RegisterPredicate(predicate))
	truth, err := m.MkTrue()
	spacerMust(t, err)
	falsity, err := m.MkFalse()
	spacerMust(t, err)
	head, err := m.MkApp(predicate, []ast.ASTHandle{truth})
	spacerMust(t, err)
	rule, err := rules.NewRule(head, nil, nil, "reachable-rewrite")
	spacerMust(t, err)
	defer func() { spacerMust(t, rule.Close()) }()
	empty, err := pt.Reachable()
	spacerMust(t, err)
	emptyHandle, err := m.RefGet(empty)
	spacerMust(t, err)
	if !m.IsFalse(emptyHandle) {
		t.Fatal("empty reach set must be false")
	}
	spacerMust(t, m.DelRef(&empty))
	signature, err := pt.Signature()
	spacerMust(t, err)
	currentDecl, err := owner.OldToNew(signature[0], 0)
	spacerMust(t, err)
	current, err := m.MkApp(currentDecl, nil)
	spacerMust(t, err)
	variable, err := m.MkVar(0, boolean)
	spacerMust(t, err)
	variableRef, err := m.NewRef(variable)
	spacerMust(t, err)
	defer func() { spacerMust(t, m.DelRef(&variableRef)) }()
	for _, value := range []ast.ASTHandle{truth, falsity} {
		equality, err := m.MkEq(current, value)
		spacerMust(t, err)
		fact, err := spacer.NewReachFact(m, rule, equality, nil, true)
		spacerMust(t, err)
		_, err = pt.AddReachFact(fact)
		spacerMust(t, err)
		spacerMust(t, fact.Close())
		result, err := pt.Reachable()
		spacerMust(t, err)
		actual, err := m.RefGet(result)
		spacerMust(t, err)
		expected := variable
		if value == falsity {
			notVariable, err := m.MkNot(variable)
			spacerMust(t, err)
			expected, err = m.MkOrArray([]ast.ASTHandle{variable, notVariable})
			spacerMust(t, err)
		}
		if actual != expected {
			t.Fatal("reach summary must simplify renamed facts separately before combining them", value, actual, expected)
		}
		spacerMust(t, m.DelRef(&result))
	}
	aux := spacerBool(t, m, "reachable-unsupported-aux")
	equality, err := m.MkEq(current, aux)
	spacerMust(t, err)
	fact, err := spacer.NewReachFact(m, rule, equality, []ast.ASTHandle{aux}, false)
	spacerMust(t, err)
	_, err = pt.AddReachFact(fact)
	spacerMust(t, err)
	spacerMust(t, fact.Close())
	if _, err = pt.Reachable(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("source does not implement reachable summaries with auxiliaries", err)
	}
}

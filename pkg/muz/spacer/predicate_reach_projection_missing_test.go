package spacer

// Source: spacer_context.cpp context::mk_reach_fact selects one origin fact
// for each predecessor before constructing and projecting the path.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestReachFactMissingLaterPredecessorReleasesEarlierSelection(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c, m := f.context, f.context.manager
	boolean, err := m.MkBoolSort()
	depthMust(t, err)
	name, err := m.Symbols.Intern("reach-missing-root")
	depthMust(t, err)
	rootDecl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	depthMust(t, err)
	depthMust(t, f.manager.RegisterPredicate(rootDecl))
	depthMust(t, f.manager.RegisterPredicate(f.other))
	rootHead, err := m.MkApp(rootDecl, nil)
	depthMust(t, err)
	firstHead, err := f.rule.Head()
	depthMust(t, err)
	secondHead, err := m.MkApp(f.other, nil)
	depthMust(t, err)
	rootRule, err := f.manager.NewRule(rootHead, []ast.ASTHandle{firstHead, secondHead}, nil, "reach-missing-root")
	depthMust(t, err)
	defer func() { depthMust(t, rootRule.Close()) }()
	secondRule, err := f.manager.NewRule(secondHead, nil, nil, "reach-missing-second")
	depthMust(t, err)
	defer func() { depthMust(t, secondRule.Close()) }()
	depthMust(t, f.rules.Add(rootRule))
	depthMust(t, f.rules.Add(secondRule))
	depthMust(t, c.UpdateRules(f.rules))
	depthMust(t, c.SetQuery(rootDecl))
	truth, err := m.MkTrue()
	depthMust(t, err)
	falsity, err := m.MkFalse()
	depthMust(t, err)
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()
	children := []*PredicateTransformer{c.transformers[f.query], c.transformers[f.other]}
	var originDecls []ast.ASTHandle
	var selected []*reachFactState
	for i, child := range children {
		if len(child.reachFacts) != 1 || len(child.reachCaseVariables) != 1 {
			t.Fatal("expected one initial fact per child")
		}
		selected = append(selected, child.reachFacts[0].state)
		h, err := m.RefGet(child.reachCaseVariables[0])
		depthMust(t, err)
		shifted, err := child.owner.FormulaNewToOld(h, uint32(i), false)
		depthMust(t, err)
		defer func() { depthMust(t, m.DelRef(&shifted)) }()
		h, err = m.RefGet(shifted)
		depthMust(t, err)
		decl, err := m.GetDecl(h)
		depthMust(t, err)
		originDecls = append(originDecls, decl)
		value := falsity
		if i == 1 {
			value = truth
		}
		depthMust(t, md.Core().RegisterConstant(decl, value))
	}
	root := c.transformers[rootDecl]
	obligation, err := root.NewProofObligation(nil, 0, 1, truth, nil)
	depthMust(t, err)
	beforeRefs := m.Stats().LiveReferences
	firstOwners, secondOwners := selected[0].references, selected[1].references
	result, err := root.MakeReachFact(obligation, rootRule, md.Core(), true, false)
	if result != nil || !errors.Is(err, ast.ErrASTDomain) || !strings.Contains(err.Error(), "predecessor 1 origin") {
		t.Fatalf("missing second fact returned %v, %v", result, err)
	}
	if selected[0].references != firstOwners || selected[1].references != secondOwners {
		t.Fatal("missing predecessor retained a selected fact")
	}
	if got := m.Stats().LiveReferences; got != beforeRefs {
		t.Fatalf("missing predecessor leaked references: %d vs %d", got, beforeRefs)
	}
	depthMust(t, md.Core().RegisterConstant(originDecls[1], falsity))
	result, err = root.MakeReachFact(obligation, rootRule, md.Core(), true, false)
	depthMust(t, err)
	justifications, err := result.Justifications()
	depthMust(t, err)
	if len(justifications) != 2 || justifications[0].state != selected[0] || justifications[1].state != selected[1] {
		t.Fatal("retry selected wrong facts")
	}
	for _, j := range justifications {
		depthMust(t, j.Close())
	}
	depthMust(t, result.Close())
	if selected[0].references != firstOwners || selected[1].references != secondOwners {
		t.Fatal("retry leaked justifications")
	}
}

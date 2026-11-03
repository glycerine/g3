package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestBlockingLemmaPipelineStopsAfterFalseStrengthening(t *testing.T) {
	m, g := newSpacerTestServices(t)
	owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
	depthMust(t, err)
	defer func() { depthMust(t, owner.Close()) }()
	truth, err := m.MkTrue()
	depthMust(t, err)
	boolean, err := m.MkBoolSort()
	depthMust(t, err)
	name, err := m.Symbols.Intern("pipeline-false")
	depthMust(t, err)
	head, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	depthMust(t, err)
	p, err := NewRulePredicateTransformer(owner, head, PropSolverConfig{})
	depthMust(t, err)
	defer func() { depthMust(t, p.Close()) }()
	depthMust(t, p.InitializeRules(map[ast.ASTHandle]*PredicateTransformer{head: p}, nil))
	a, err := m.MkFreshConst("pipeline-a", boolean)
	depthMust(t, err)
	b, err := m.MkFreshConst("pipeline-b", boolean)
	depthMust(t, err)
	obligation, err := NewProofObligation(nil, p, 1, 0, truth, nil)
	depthMust(t, err)
	defer func() { depthMust(t, obligation.Close()) }()
	p.useInductiveGeneralizer = true
	p.lemmaSanityCheck = true
	lemma, err := NewLemmaFromProofObligation(obligation, []ast.ASTHandle{a, b}, 1)
	depthMust(t, err)
	before := p.solver.ownedBackends[0].factory.checks + p.solver.ownedBackends[1].factory.checks
	result, err := p.generalizeBlockingLemma(obligation, lemma)
	depthMust(t, err)
	defer func() { depthMust(t, result.Close()) }()
	body, err := result.Body()
	depthMust(t, err)
	if !m.IsFalse(body) {
		t.Fatal("inductive generalizer did not produce false", body)
	}
	after := p.solver.ownedBackends[0].factory.checks + p.solver.ownedBackends[1].factory.checks
	if after-before != 1 {
		t.Fatalf("false lemma ran later sanity solver: %d checks, want one inductive check", after-before)
	}
}

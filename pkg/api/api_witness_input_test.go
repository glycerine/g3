package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

func TestWitnessInstancesFreeVariableInSharedBody(t *testing.T) {
	ctx, s := orderSolver(t, witnessTestDeclarations+"(assert (forall ((x A)) (p x)))")
	m, err := ctx.Manager()
	astTestMust(t, err)
	forms, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	body, err := m.GetQuantifierBody(forms[0])
	astTestMust(t, err)
	trial := witnessClauseSet{m: m}
	ok, err := trial.acceptsInput(m, forms)
	if err != nil || !ok {
		t.Fatalf("closed quantifier: %v, %v", ok, err)
	}
	for _, roots := range [][]ast.ASTHandle{{forms[0], body}, {body, forms[0]}} {
		ok, err = trial.acceptsInput(m, roots)
		if err != nil || ok {
			t.Fatalf("shared open body: %v, %v", ok, err)
		}
	}
}

func TestWitnessInstancesFreshSymbolCollision(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	defer source.Close()
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer target.Close()
	m, err := source.Manager()
	astTestMust(t, err)
	aux, err := target.Manager()
	astTestMust(t, err)
	bs, err := m.MkBoolSort()
	astTestMust(t, err)
	original, err := m.MkFreshConst("g3!ground", bs)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(original))
	defer m.DecRef(original)
	tr, err := ast.NewASTTranslation(m, aux, true)
	astTestMust(t, err)
	defer tr.Close()
	copied, err := tr.Translate(original)
	astTestMust(t, err)
	astTestMust(t, aux.IncRef(copied))
	defer aux.DecRef(copied)
	sort, err := aux.GetSort(copied)
	astTestMust(t, err)
	trial := witnessClauseSet{m: aux}
	defer func() { _, err := aux.ShrinkHeldRefs(trial.held, 0); astTestMust(t, err) }()
	h := trial.freshConst(sort)
	if trial.err != nil && !errors.Is(trial.err, errWitnessLimit) {
		t.Fatal(trial.err)
	}
	if trial.err == nil && h == copied {
		t.Fatal("reused an input declaration as a fresh witness")
	}
}

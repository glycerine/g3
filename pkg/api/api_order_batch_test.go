package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// Put the contradiction in the retained base, leaving a harmless last
// assertion. The next query changes a premise and keeps the same last line.
func TestOrderBatchDropsContradictoryPremises(t *testing.T) {
	_, s := orderSolver(t, orderPremises)
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	const declarations = "(declare-const flag Bool)(declare-const tail Bool)"
	for _, tc := range []struct {
		body string
		want lbool.LBool
	}{
		{"(assert flag)(assert (not flag))(assert tail)", lbool.False},
		{"(assert flag)(assert tail)", lbool.True},
		{"(assert flag)(assert (not flag))(assert tail)", lbool.False},
		{"(assert (not flag))(assert tail)", lbool.True},
	} {
		astTestMust(t, s.Push())
		astTestMust(t, s.LoadSMT2String(declarations+tc.body))
		yes, err := batch.Prove()
		if err != nil || yes != (tc.want == lbool.False) {
			t.Fatalf("proof of %s = %t, %v", tc.body, yes, err)
		}
		if result, err := s.Check(); err != nil || result != tc.want {
			t.Fatalf("concrete check of %s = %v, %v", tc.body, result, err)
		}
		astTestMust(t, s.Pop(1))
	}
}

func TestOrderBatchBoundsDeclarationsAcrossGoals(t *testing.T) {
	_, s := orderSolver(t, orderPremises)
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	var large strings.Builder
	for i := 0; i < 64; i++ {
		fmt.Fprintf(&large, "(declare-const extra%d Bool)(assert extra%d)", i, i)
	}
	const contradiction = "(declare-const c Bool)(declare-const d Bool)(assert (or c d))(assert (not d))(assert (not c))"
	astTestMust(t, s.Push())
	astTestMust(t, s.LoadSMT2String(large.String()+contradiction))
	if yes, err := batch.Prove(); err != nil || !yes {
		t.Fatal(yes, err)
	}
	old := batch.state.parser
	n := len(batch.state.declared)
	astTestMust(t, s.Pop(1))
	astTestMust(t, s.LoadSMT2String(contradiction))
	if yes, err := batch.Prove(); err != nil || !yes {
		t.Fatal(yes, err)
	}
	if batch.state.parser == old || len(batch.state.declared)*2 >= n {
		t.Fatal("large declaration world survived the small successor", n, len(batch.state.declared))
	}
}

func TestOrderBatchRebindsChangedFunctionSignature(t *testing.T) {
	_, s := orderSolver(t, "")
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	// Every translation names its first predicate f0. Only its signature
	// changes from one query to the next, so the parser must rebind it.
	const unary = "(declare-fun q ((_ BitVec 2)) Bool)(assert (forall ((x (_ BitVec 2))) (q x)))"
	const binary = "(declare-fun q ((_ BitVec 2) (_ BitVec 2)) Bool)(assert (forall ((x (_ BitVec 2))) (q x x)))"
	var previous *APISimpleSolver
	for i, query := range []struct {
		input string
		want  lbool.LBool
	}{
		{unary + "(declare-const g Bool)(assert (or g (not (q #b00))))(assert (not g))", lbool.False},
		{binary + "(declare-const g Bool)(assert (or g (not (q #b00 #b00))))(assert (not g))", lbool.False},
		{unary + "(assert (q #b00))", lbool.True},
	} {
		astTestMust(t, s.Push())
		astTestMust(t, s.LoadSMT2String(query.input))
		proved, err := batch.Prove()
		if err != nil || proved != (query.want == lbool.False) {
			t.Fatalf("query %d, proof %t, error %v", i, proved, err)
		}
		if i == 1 && previous == batch.state.child {
			t.Fatal("reused the kernel after f0 changed arity")
		}
		previous = batch.state.child
		if answer, err := s.Check(); err != nil || answer != query.want {
			t.Fatal(i, answer, err)
		}
		astTestMust(t, s.Pop(1))
	}
}

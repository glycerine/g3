package api

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestWitnessPremisesTwoRounds(t *testing.T) {
	const problem = witnessTestDeclarations + `
(declare-fun downstream (A) Bool)(declare-fun last (A) Bool)(declare-fun other (A) Bool)
(assert (and
  (forall ((x A)) (=> (downstream x) (last x)))
  (forall ((x A)) (other x))
  (or switch (forall ((x A)) (=> (p x) (downstream x))))
  (not (p a))))
(assert (not (forall ((x A)) (p x))))`
	ctx, s := orderSolver(t, problem)
	m, err := ctx.Manager()
	astTestMust(t, err)
	fs, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	if len(fs) != 2 {
		t.Fatalf("expected conjunction and query, got %d roots", len(fs))
	}
	var want []ast.ASTHandle
	for _, i := range []uint32{0, 2, 3} {
		root, err := m.GetArg(fs[0], i)
		astTestMust(t, err)
		want = append(want, root)
	}
	want = append(want, fs[1])
	full := &witnessInstancePlan{assertions: fs, width: 6, rounds: 3}
	p := orderProofState{source: s}
	trial, err := p.selectWitnessPremises(m, full)
	astTestMust(t, err)
	if trial == nil || !reflect.DeepEqual(trial.assertions, want) || !reflect.DeepEqual(trial.source, fs) || trial.rounds != 1 {
		t.Fatalf("unexpected two-round selection %v, want %v", trial, want)
	}
	// The disjunction holds as a whole. Its quantified disjunct cannot be
	// asserted on its own. A free variable in an omitted premise must still
	// disqualify the private proof, even when the selected formulas conflict.
	omitted, err := m.GetArg(fs[0], 1)
	astTestMust(t, err)
	open, err := m.GetQuantifierBody(omitted)
	astTestMust(t, err)
	conflict, err := m.MkNot(want[2])
	astTestMust(t, err)
	astTestMust(t, m.IncRef(conflict))
	defer m.DecRef(conflict)
	full.assertions = []ast.ASTHandle{fs[0], open, conflict, fs[1]}
	trial, err = p.selectWitnessPremises(m, full)
	astTestMust(t, err)
	if trial == nil {
		t.Fatal("expected a subset for the second query")
	}
	for _, h := range trial.assertions {
		if h == open {
			t.Fatal("the unrelated open term was selected")
		}
	}
	ok, err := p.checkProofPlan(m, trial, orderProofSteps)
	astTestMust(t, err)
	if ok {
		t.Fatal("private check accepted an open source premise")
	}
}

func TestWitnessPremisesBatchReplacesPoppedBase(t *testing.T) {
	ctx, input := orderSolver(t, witnessTestDeclarations+`
(assert (forall ((x A)) (p x)))
(assert (or switch (forall ((x A)) (p x))))(assert `+witnessTestGoal("(not (p a))")+")")
	m, err := ctx.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(input.solver)
	astTestMust(t, err)
	s, err := ctx.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	for i, premise := range roots[:2] {
		astTestMust(t, s.Push())
		astTestMust(t, s.Assert(premise))
		astTestMust(t, s.Assert(roots[2]))
		fs, err := solver.Assertions(s.solver)
		astTestMust(t, err)
		yes, err := batch.state.proveGoalInstances(m, fs, orderProofSteps)
		astTestMust(t, err)
		if yes != (i == 0) || !batch.state.witnessPremises.matches(fs[:len(fs)-1]) {
			t.Fatalf("scope %d: proof=%v, index does not match current base or verdict", i, yes)
		}
		if i == 1 {
			answer, err := s.Check()
			astTestMust(t, err)
			if answer != lbool.True {
				t.Fatalf("guarded model returned %v", answer)
			}
			yes, err = batch.Prove()
			astTestMust(t, err)
			if yes {
				t.Fatal("refuted the guarded model")
			}
			model, err := s.Model()
			astTestMust(t, err)
			if model == nil {
				t.Fatal("private check lost the source model")
			}
			astTestMust(t, model.Close())
		}
		astTestMust(t, s.Pop(1))
	}
	astTestMust(t, batch.Close())
	if batch.state.witnessPremises != nil {
		t.Fatal("closed batch retained the premise index")
	}
}

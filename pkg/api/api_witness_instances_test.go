package api

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const witnessTestDeclarations = `(declare-sort A 0)(declare-sort B 0)
(declare-const a A)(declare-const b A)(declare-const c B)
(declare-fun p (A) Bool)(declare-fun q (A) Bool)
(declare-fun bad (A) Bool)(declare-fun excluded (A) Bool)
(declare-fun rel (B A) Bool)(declare-fun allow (Bool) Bool)
(declare-const switch Bool)
`

// Each test goal meets the multi-quantifier admission rule. The three guards
// have a common model on any nonempty domain, with q true and bad/excluded false.
func witnessTestGoal(term string) string {
	return "(and (forall ((x A)) (q x)) (not (forall ((x A)) (bad x))) (not (exists ((x A)) (excluded x))) " + term + ")"
}

func TestWitnessInstancesDirections(t *testing.T) {
	for _, tc := range []struct {
		name, premise, term string
		unsat               bool
	}{
		{"universal counterexample", "(and (p a) (not (p b)))", "(not (forall ((x A)) (p x)))", false},
		{"universal covers its counterexample", "(forall ((x A)) (p x))", "(not (forall ((x A)) (p x)))", true},
		{"existential witness differs from existing constant", "(and (not (p a)) (p b))", "(exists ((x A)) (p x))", false},
		{"existential witness forbidden", "(forall ((x A)) (not (p x)))", "(exists ((x A)) (p x))", true},
		{"negated existential excludes every term", "(excluded a)", "true", true},
		{"multiparameter binding order", "(forall ((x A)(y B)) (rel y x))", "(not (rel c a))", true},
		{"conditional can avoid universal", "(and switch (not (p a)))", "(ite switch true (forall ((x A)) (p x)))", false},
		{"opaque Boolean argument", "(and (not (p a)) (allow false))", "(allow (forall ((x A)) (p x)))", false},
		{"Boolean equality can require existential", "(forall ((x A)) (not (p x)))", "(= true (exists ((x A)) (p x)))", true},
		{"negated implication", "(p a)", "(not (=> (forall ((x A)) (q x)) (exists ((x A)) (p x))))", true},
		{"nested witnesses depend on outer binder", "(and (distinct a b) (p a) (p b))", "(forall ((x A)) (exists ((y A)) (distinct x y)))", false},
		{"unused differently typed binders", "(and (p a) (not (p b)))", "(and (exists ((x A)(u B)) (p x)) (not (forall ((x A)(u A)) (p x))))", false},
		{"shadowed variable", "(and (not (p a)) (p b))", "(forall ((x A)) (exists ((x A)) (p x)))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, witnessTestDeclarations+"(assert "+tc.premise+")(assert "+witnessTestGoal(tc.term)+")")
			forms, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			m, err := c.Manager()
			astTestMust(t, err)
			plan, err := planWitnessInstances(m, forms)
			if err != nil || plan == nil {
				t.Fatalf("expected an admitted goal, got %v, %v", plan, err)
			}
			p := orderProofState{source: s}
			proved, err := p.checkProofPlan(m, plan, orderProofSteps)
			astTestMust(t, err)
			if proved != tc.unsat {
				t.Fatalf("ground proof returned %v, want %v", proved, tc.unsat)
			}
			want := lbool.True
			if tc.unsat {
				want = lbool.False
			}
			answer, err := s.Check()
			astTestMust(t, err)
			if answer != want {
				t.Fatalf("source solver returned %v, want %v", answer, want)
			}
		})
	}
}

func TestWitnessInstancesAllTwoElementPredicateModels(t *testing.T) {
	// Fix a two-element interpretation of p, including both mixed cases. The
	// witnesses for “some p” and “some not p” may be different domain elements.
	for bits := 0; bits < 4; bits++ {
		t.Run(fmt.Sprintf("p=%02b", bits), func(t *testing.T) {
			lit := func(args string, flag int) string {
				v := "(p " + args + ")"
				if bits&flag == 0 {
					return "(not " + v + ")"
				}
				return v
			}
			body := "(and (distinct a b) (forall ((x A)) (or (= x a) (= x b))) " + lit("a", 1) + " " + lit("b", 2) + ")"
			goal := witnessTestGoal("(and (exists ((x A)) (p x)) (not (forall ((x A)) (p x))))")
			c, s := orderSolver(t, witnessTestDeclarations+"(assert "+body+")(assert "+goal+")")
			forms, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			m, err := c.Manager()
			astTestMust(t, err)
			plan, err := planWitnessInstances(m, forms)
			astTestMust(t, err)
			if plan == nil {
				t.Fatal("missing witness plan")
			}
			p := orderProofState{source: s}
			got, err := p.checkProofPlan(m, plan, orderProofSteps)
			astTestMust(t, err)
			unsat := bits == 0 || bits == 3
			if got != unsat {
				t.Fatalf("proof=%v, exhaustive model unsat=%v", got, unsat)
			}
			want := lbool.True
			if unsat {
				want = lbool.False
			}
			answer, err := s.Check()
			astTestMust(t, err)
			if answer != want {
				t.Fatalf("Check=%v, want %v", answer, want)
			}
		})
	}
}

func TestWitnessInstancesBudgetScopeAndModel(t *testing.T) {
	c, s := orderSolver(t, witnessTestDeclarations+"(assert (forall ((x A)) (p x)))(assert "+witnessTestGoal("true")+")")
	m, err := c.Manager()
	astTestMust(t, err)
	source, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	base := source[len(source)-1]
	plan, err := planWitnessInstances(m, source)
	astTestMust(t, err)
	if plan == nil {
		t.Fatal("missing plan")
	}
	p := orderProofState{source: s}
	if yes, err := p.checkProofPlan(m, plan, 1); err != nil || yes || len(m.ResourceLimit().Children) != 0 {
		t.Fatalf("one-step attempt: %v, %v", yes, err)
	}
	answer, err := s.Check()
	astTestMust(t, err)
	if answer != lbool.True {
		t.Fatalf("base Check=%v", answer)
	}
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	if yes, err := batch.Prove(); err != nil || yes {
		t.Fatalf("satisfiable base: %v, %v", yes, err)
	}
	model, err := s.Model()
	astTestMust(t, err)
	if model == nil {
		t.Fatal("lost source model")
	}
	astTestMust(t, model.Close())
	astTestMust(t, s.Push())
	neg, err := m.MkNot(source[0])
	astTestMust(t, err)
	astTestMust(t, s.Assert(neg))
	astTestMust(t, s.Assert(base))
	if yes, err := batch.Prove(); err != nil || !yes {
		t.Fatalf("scoped proof: %v, %v", yes, err)
	}
	astTestMust(t, s.Pop(1))
	if yes, err := batch.Prove(); err != nil || yes {
		t.Fatalf("after Pop: %v, %v", yes, err)
	}
	done := make(chan struct{})
	close(done)
	m.ResourceLimit().RunSignals = &control.RunSignals{Done: done}
	defer func() { m.ResourceLimit().RunSignals = nil }()
	if yes, err := batch.Prove(); err != nil || yes || len(m.ResourceLimit().Children) != 0 {
		t.Fatalf("canceled proof: %v, %v", yes, err)
	}
}

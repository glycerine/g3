package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const quantifiedGoalDeclarations = `(declare-sort A 0)(declare-sort B 0)
(declare-const a A)(declare-const b B)
(declare-const flag Bool)(declare-fun p (A) Bool)
(declare-fun r (A B) Bool)(declare-fun f (A) A)
`

func TestQuantifiedGoalImplicationDirections(t *testing.T) {
	cases := []struct {
		name, premises, goal string
		want                 bool
	}{
		{"positive universal", "(not (p a))", "(and flag (forall ((x A)) (p x)))", true},
		{"negated existential", "(p a)", "(not (exists ((x A)) (p x)))", true},
		{"negated universal", "(p a)", "(not (forall ((x A)) (p x)))", false},
		{"positive existential", "(not (p a))", "(exists ((x A)) (p x))", false},
		{"disjunction permits other disjunct", "(not (p a))", "(or flag (forall ((x A)) (p x)))", false},
		{"false disjunct", "(and (not flag) (not (p a)))", "(or flag (forall ((x A)) (p x)))", true},
		{"equivalence permits false universal", "(and (p a) (not flag))", "(= flag (forall ((x A)) (p x)))", false},
		{"true equivalence requires universal", "(and (not (p a)) flag)", "(= flag (forall ((x A)) (p x)))", true},
		{"conditional", "(and flag (not (p a)))", "(ite flag (forall ((x A)) (p x)) true)", true},
		{"xor", "(and (not flag) (not (p a)))", "(xor flag (forall ((x A)) (p x)))", true},
		{"ground compound term", "(not (p (f a)))", "(forall ((x A)) (p (f x)))", true},
		{"symmetric data equality", "(not (= a (f a)))", "(forall ((x A)) (= (f x) x))", true},
		{"two binder sorts", "(not (r a b))", "(forall ((x A)(y B)) (r x y))", true},
		{"nested existential", "(not (p a))", "(forall ((x A)) (exists ((y A)) (p y)))", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, s := orderSolver(t, quantifiedGoalDeclarations+"(assert "+tc.premises+")(assert "+tc.goal+")")
			got, err := s.ProveUnsatByOrderAbstraction()
			if err != nil || got != tc.want {
				t.Fatalf("private proof returned %v, %v, want %v", got, err, tc.want)
			}
			answer, err := s.Check()
			want := lbool.True
			if tc.want {
				want = lbool.False
			}
			if err != nil || answer != want {
				t.Fatalf("independent check returned %v, %v, want %v", answer, err, want)
			}
		})
	}
}

func TestQuantifiedGoalAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, goal string
		eligible   bool
	}{
		{"ground", "(p a)", false},
		{"single", "(or flag (forall ((x A)) (p x)))", true},
		{"two flat", "(and (forall ((x A)) (p x)) (exists ((x A)) (p (f x))))", false},
		{"nested", "(and (forall ((x A)) (p x)) (exists ((x A)) (forall ((y B)) (r x y))))", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, quantifiedGoalDeclarations+"(assert (p a))(assert "+tc.goal+")")
			m, err := c.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			problem, err := order.ProjectQuantifiedGoal(m, roots, orderProofSteps)
			if err != nil || (problem != nil) != tc.eligible {
				t.Fatalf("eligible = %v, %v, want %v", problem != nil, err, tc.eligible)
			}
		})
	}
}

func TestQuantifiedGoalDistinguishesOverloadedDeclarations(t *testing.T) {
	_, s := orderSolver(t, quantifiedGoalDeclarations+
		"(declare-fun p (B) Bool)(assert (and (p a) (not (p b))))(assert (forall ((x A)) (p x)))")
	got, err := s.ProveUnsatByOrderAbstraction()
	if err != nil || got {
		t.Fatalf("distinct predicates collapsed: %v, %v", got, err)
	}
	answer, err := s.Check()
	if err != nil || answer != lbool.True {
		t.Fatalf("independent check returned %v, %v", answer, err)
	}
}

func TestQuantifiedGoalRetryScopeAndModel(t *testing.T) {
	c, s := orderSolver(t, quantifiedGoalDeclarations+
		"(assert (not (p a)))(assert (or flag (forall ((x A)) (p x))))")
	m, err := c.Manager()
	astTestMust(t, err)
	v, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	goal := v[len(v)-1]
	flag, err := m.GetArg(goal, 0)
	astTestMust(t, err)
	neg, err := m.MkNot(flag)
	astTestMust(t, err)
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(neg))
	astTestMust(t, s.Assert(goal))
	if got, err := s.proveUnsatByOrderAbstraction(1); err != nil || got {
		t.Fatalf("one-step proof returned %v, %v", got, err)
	}
	if got, err := batch.Prove(); err != nil || !got {
		t.Fatalf("retry returned %v, %v", got, err)
	}
	astTestMust(t, s.Pop(1))
	if got, err := batch.Prove(); err != nil || got {
		t.Fatalf("after pop returned %v, %v", got, err)
	}
	if answer, err := s.Check(); err != nil || answer != lbool.True {
		t.Fatalf("after pop Check returned %v, %v", answer, err)
	}
	model, err := s.Model()
	astTestMust(t, err)
	if model == nil {
		t.Fatal("missing concrete model after inconclusive proof")
	}
	astTestMust(t, model.Close())
	done := make(chan struct{})
	close(done)
	m.ResourceLimit().RunSignals = &control.RunSignals{Done: done}
	defer func() { m.ResourceLimit().RunSignals = nil }()
	if got, err := batch.Prove(); err != nil || got || len(m.ResourceLimit().Children) != 0 {
		t.Fatalf("canceled proof returned %v, %v with %d children", got, err, len(m.ResourceLimit().Children))
	}
}

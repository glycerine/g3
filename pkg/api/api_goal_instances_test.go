package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// The two sorts differ so reversing a substitution's de Bruijn order cannot
// accidentally prove the example. Numbered, independent predicates make the
// problem eligible for the same private proof as a large verification condition.
func goalInstanceInput(extra, goal string) string {
	var text strings.Builder
	text.WriteString(`(declare-sort A 0)(declare-sort B 0)
(declare-const a A)(declare-const another A)(declare-const b B)
(declare-fun property (A B) Bool)(declare-const guard Bool)
`)
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&text, "(declare-fun p%d (A B) Bool)(assert (forall ((x A)(y B)) (p%d x y)))\n", i, i)
	}
	text.WriteString(extra)
	text.WriteString("(assert ")
	text.WriteString(goal)
	text.WriteString(")")
	return text.String()
}

func TestGoalInstancesProveUniversalConsequenceAndPreserveParent(t *testing.T) {
	ctx, s := orderSolver(t, goalInstanceInput("(assert (forall ((x A)(y B)) (property x y)))", "(not (property a b))"))
	m, err := ctx.Manager()
	astTestMust(t, err)
	batch := s.BeginOrderProofBatch()
	defer func() { astTestMust(t, batch.Close()) }()
	for i := 0; i < 2; i++ {
		proved, err := batch.Prove()
		if err != nil || !proved {
			t.Fatalf("attempt %d returned %v, %v", i, proved, err)
		}
		if len(m.ResourceLimit().Children) != 0 || batch.state.pendingCondition != nil {
			t.Fatal("ground proof retained a child solver or cancellation registration")
		}
	}
	stats, err := s.Statistics()
	astTestMust(t, err)
	after, err := stats.String()
	astTestMust(t, err)
	astTestMust(t, stats.Close())
	// rlimit-count includes source-side setup. Search belongs to the private solver.
	if strings.Contains(after, ":decisions ") || strings.Contains(after, ":quant-instantiations ") {
		t.Fatalf("source solver searched during private proof: %s", after)
	}
	answer, err := s.Check()
	if err != nil || answer != lbool.False {
		t.Fatalf("independent source check returned %v, %v", answer, err)
	}
}

func TestGoalInstancesAcceptOnlyPositiveUniversalConjuncts(t *testing.T) {
	cases := []struct{ name, premise string }{
		{"disjunction", "(assert (or guard (forall ((x A)(y B)) (property x y))))"},
		{"existential", "(assert (exists ((x A)(y B)) (property x y)))"},
		{"negation", "(assert (not (forall ((x A)(y B)) (property x y))))"},
		{"nested", "(assert (forall ((x A)) (exists ((y B)) (property x y))))"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, s := orderSolver(t, goalInstanceInput(tc.premise, "(not (property a b))"))
			proved, err := s.ProveUnsatByOrderAbstraction()
			if err != nil || proved {
				t.Fatalf("ground proof returned %v, %v", proved, err)
			}
			answer, err := s.Check()
			if err != nil || answer != lbool.True {
				t.Fatalf("source check returned %v, %v", answer, err)
			}
			model, err := s.Model()
			if err != nil || model == nil {
				t.Fatalf("missing concrete model: %v", err)
			}
			astTestMust(t, model.Close())
		})
	}
}

func TestGoalInstancesAmbiguousWitnessAndCancellation(t *testing.T) {
	ctx, s := orderSolver(t, goalInstanceInput("", "(and (not (p0 a b)) (p1 another b))"))
	m, err := ctx.Manager()
	astTestMust(t, err)
	values, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	plan, err := planGoalInstances(m, values)
	if err != nil || plan != nil {
		t.Fatalf("ambiguous witnesses yielded a plan: %v", err)
	}

	ctx, s = orderSolver(t, goalInstanceInput("", "(not (p0 a b))"))
	m, err = ctx.Manager()
	astTestMust(t, err)
	done := make(chan struct{})
	close(done)
	m.ResourceLimit().RunSignals = &control.RunSignals{Done: done}
	defer func() { m.ResourceLimit().RunSignals = nil }()
	proved, err := s.ProveUnsatByOrderAbstraction()
	if proved || err != nil || len(m.ResourceLimit().Children) != 0 {
		t.Fatalf("canceled attempt returned %v, %v with %d children", proved, err, len(m.ResourceLimit().Children))
	}
}

func TestGoalInstancesBatchObservesPopAndPreservesModel(t *testing.T) {
	c, s := orderSolver(t, goalInstanceInput("", "(not (property a b))"))
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	m, err := c.Manager()
	astTestMust(t, err)
	values, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	last := values[len(values)-1]
	astTestMust(t, s.Push())
	positive, err := m.MkNot(last)
	astTestMust(t, err)
	astTestMust(t, s.Assert(positive))
	astTestMust(t, s.Assert(last))
	if proved, err := batch.Prove(); !proved || err != nil {
		t.Fatalf("scoped contradiction returned %v, %v", proved, err)
	}
	astTestMust(t, s.Pop(1))
	if answer, err := s.Check(); answer != lbool.True || err != nil {
		t.Fatalf("after pop Check returned %v, %v", answer, err)
	}
	if proved, err := batch.Prove(); proved || err != nil {
		t.Fatalf("after pop private proof returned %v, %v", proved, err)
	}
	model, err := s.Model()
	astTestMust(t, err)
	if model == nil {
		t.Fatal("inconclusive proof discarded concrete model")
	}
	astTestMust(t, model.Close())
}

func TestGoalInstancesRetryAfterStepLimit(t *testing.T) {
	c, s := orderSolver(t, goalInstanceInput("", "(not (p0 a b))"))
	m, err := c.Manager()
	astTestMust(t, err)
	if proved, err := s.proveUnsatByOrderAbstraction(1); proved || err != nil {
		t.Fatalf("one-step proof returned %v, %v", proved, err)
	}
	if len(m.ResourceLimit().Children) != 0 {
		t.Fatal("exhausted proof retained child resource limit")
	}
	if proved, err := s.ProveUnsatByOrderAbstraction(); !proved || err != nil {
		t.Fatalf("retry returned %v, %v", proved, err)
	}
}

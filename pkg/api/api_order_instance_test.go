package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOrderProofGroundInstances(t *testing.T) {
	const declarations = "(define-sort A () (_ BitVec 3))(declare-const a A)(declare-const b A)" +
		"(declare-fun r (A A) Bool)(declare-fun q (A) Bool)(declare-fun h (A) A)"
	for _, tc := range []struct {
		name, premise, goal string
		refuted             bool
	}{
		{"repeated term", "(forall ((x A)) (r x x))", "(not (r (h a) (h a)))", true},
		{"unequal terms", "(forall ((x A)) (r x x))", "(not (r (h a) (h b)))", false},
		{"bindings after failed match", "(forall ((x A)) (r x x))", "(not (or (r a b) (r b b)))", true},
		{"implication instance", "(forall ((x A)) (=> (q x) (r x x)))", "(not (=> (q b) (r b b)))", true},
		{"universal from negative exists", "(not (exists ((x A)) (q x)))", "(q a)", true},
		{"negative forall", "(not (forall ((x A)) (q x)))", "(q a)", false},
		{"existential", "(exists ((x A)) (q x))", "(not (q a))", false},
		{"shadowed binder", "(forall ((x A)) (exists ((x A)) (q x)))", "(not (q a))", false},
		{"signed disjunction", "(forall ((x A)) (or (q x) (r x x)))", "(not (q a))", false},
		{"signed conjunction", "(forall ((x A)) (not (and (q x) (r x x))))", "(q a)", false},
		{"quantified target", "(and (forall ((x A)) (r x x)) (q a))", "(exists ((x A)) (not (q x)))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, declarations+"(assert "+tc.premise+")")
			m, err := c.Manager()
			astTestMust(t, err)
			batch := s.BeginOrderProofBatch()
			t.Cleanup(func() { astTestMust(t, batch.Close()) })
			astTestMust(t, s.Push())
			astTestMust(t, s.LoadSMT2String(declarations+"(assert "+tc.goal+")"))
			assertions, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			p, applicable, err := order.ForProof(m, assertions, orderProofSteps)
			if err != nil || !applicable || p.Refuted != tc.refuted {
				t.Fatal("instance match", p.Refuted, applicable, err)
			}
			if ok, e := batch.Prove(); e != nil || ok != tc.refuted {
				t.Fatal("public proof", ok, e)
			}
			if tc.refuted && (batch.state.bits != nil || batch.state.ctx != nil) {
				t.Fatal("allocated a solver for a ground instance")
			}
			expected := lbool.True
			if tc.refuted {
				expected = lbool.False
			}
			if got, e := s.Check(); e != nil || got != expected {
				t.Fatal("concrete check", got, e)
			}
			astTestMust(t, s.Pop(1))
			if ok, e := batch.Prove(); e != nil || ok {
				t.Fatal("proof survived pop", ok, e)
			}
		})
	}
}

func TestOrderGroundInstanceUsesStepBudget(t *testing.T) {
	c, s := orderSolver(t, booleanCoverage+"(declare-const a B)(assert (not (f a)))")
	m, err := c.Manager()
	astTestMust(t, err)
	assertions, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	// Find the first budget that permits the syntactic proof. Immediately
	// below that boundary neither the matcher nor a fallback gets a result.
	for steps := uint32(1); steps < 256; steps++ {
		p, applicable, err := order.ForProof(m, assertions, steps)
		astTestMust(t, err)
		if !p.Refuted {
			if applicable {
				t.Fatal("used extra search budget", steps)
			}
			continue
		}
		if steps == 1 || !applicable || p.Boolean != nil || p.Order.Text != "" {
			t.Fatal("direct proof", steps, applicable)
		}
		if ok, e := s.proveUnsatByOrderAbstraction(steps - 1); e != nil || ok {
			t.Fatal("exhausted public proof", steps, ok, e)
		}
		if ok, e := s.proveUnsatByOrderAbstraction(steps); e != nil || !ok {
			t.Fatal("budgeted public proof", steps, ok, e)
		}
		return
	}
	t.Fatal("no direct one-bit refutation")
}

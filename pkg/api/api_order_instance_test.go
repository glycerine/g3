package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOrderProofInstantiatesOriginalTransitionPremise(t *testing.T) {
	const base = `(define-sort A () (_ BitVec 2))
(declare-const a A)(declare-const b A)(declare-const c A)(declare-const d A)
(declare-fun mark (A A A A) Bool)
(declare-fun old (A A) Bool)(declare-fun updated (A A) Bool)
(declare-fun ready (A A) Bool)
(assert (forall ((x A)(y A)(z A)(w A))
  (=> (and (mark x y z w) (old x y)) (ready z w))))`
	for _, tc := range []struct {
		name, condition string
		wantInstance    bool
	}{
		{"corresponding predicates", "(updated a b)", true},
		{"inconsistent variable binding", "(updated b a)", false},
		{"quantified goal", "(exists ((v A)) (updated v b))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, base+"(assert (not (=> (and (mark a b c d) "+tc.condition+") (ready c d))))")
			m, err := c.Manager()
			astTestMust(t, err)
			assertions, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			original, ok, err := order.Relax(m, assertions)
			astTestMust(t, err)
			if !ok {
				t.Fatal("expected an order relaxation")
			}
			p, ok, err := order.ForProof(m, assertions, orderProofSteps)
			astTestMust(t, err)
			if !ok || p.Refuted || p.Order.Premises() != original.Premises() ||
				p.Order.Declarations() != original.Declarations() {
				t.Fatal("expected the original declarations and universal premises")
			}
			goal := original.Text[original.LastAssertionStart:]
			last := p.Order.Text[p.Order.LastAssertionStart:]
			if tc.wantInstance {
				body := strings.TrimSuffix(strings.TrimPrefix(goal, "(assert "), ")\n")
				rest, found := strings.CutPrefix(last, "(assert (and "+body+" ")
				lemma, closed := strings.CutSuffix(rest, "))\n")
				if !found || !closed {
					t.Fatalf("missing ground premise in %s", last)
				}
				_, counterexample := orderSolver(t, original.Declarations()+original.Premises()+"(assert (not "+lemma+"))")
				got, err := counterexample.Check()
				astTestMust(t, err)
				if got != lbool.False {
					t.Fatalf("ground instance does not follow from the original premises: %s", lemma)
				}
			} else if last != goal {
				t.Fatalf("unexpected ground instance: %s", last)
			}
			// For example, interpret old as false, updated and mark as true,
			// and ready as false. Renaming old in the lemma would lose this model.
			_, abstract := orderSolver(t, p.Order.Text)
			got, err := abstract.Check()
			astTestMust(t, err)
			if got != lbool.True {
				t.Fatalf("the updated predicate admits a counterexample, got %v", got)
			}
		})
	}
}

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

func TestOrderProofPointUpdateWitness(t *testing.T) {
	const decls = `(define-sort A () (_ BitVec 2))
(declare-const a A)(declare-const b A)(declare-const c A)(declare-const d A)
(declare-const p A)(declare-const q A)
(declare-fun old (A A) Bool)(declare-fun updated (A A) Bool)
(declare-fun edge (A A) Bool)(declare-fun tag (A A) A)
`
	const symmetric = `(=> (and (and (old x z) (old y z)) (edge z w)) (= (tag x w) (tag y w)))`
	const asymmetric = `(=> (and (and (old x z) (old y z)) (edge z w)) (edge x w))`
	const single = `(=> (and (old x z) (edge z w)) (edge y w))`
	const updated = `(ite (and (= i p)) true (old i j))`
	for _, tc := range []struct {
		name, invariant, rhs, extra string
		specialized                 bool
	}{
		{"two private symmetric witnesses", symmetric, updated, "", true},
		{"one witness", single, updated, "", true},
		{"one public witness", single, updated, "(assert (distinct a b))", true},
		{"public witness in pair", symmetric, updated, "(assert (distinct a p))", false},
		{"public witness under binder", symmetric, updated, "(assert (forall ((i A)) (edge b i)))", false},
		{"asymmetric conclusion", asymmetric, updated, "", false},
		{"update at either of two points", symmetric, "(ite (= i p) true (ite (= i q) true (old i j)))", "", false},
		{"outside reads a different cell", symmetric, "(ite (= i p) true (old j i))", "", false},
		{"unconditional update", symmetric, "true", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goal := strings.NewReplacer("old", "updated", " x", " a", " y", " b", " z", " c", " w", " d").Replace(tc.invariant)
			c, s := orderSolver(t, decls+
				"(assert (forall ((x A)(y A)(z A)(w A)) "+tc.invariant+"))"+
				"(assert (forall ((i A)(j A)) (= (updated i j) "+tc.rhs+")))"+
				tc.extra+"(assert (not "+goal+"))")
			m, err := c.Manager()
			astTestMust(t, err)
			assertions, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			before, ok, err := order.Relax(m, assertions)
			astTestMust(t, err)
			if !ok {
				t.Fatal("expected an order problem")
			}
			after, ok, err := order.ForProof(m, assertions, orderProofSteps)
			astTestMust(t, err)
			if !ok || after.Refuted || before.Premises() != after.Order.Premises() ||
				before.Declarations() != after.Order.Declarations() {
				t.Fatal("expected the same declarations and premises")
			}
			original := strings.TrimSuffix(strings.TrimPrefix(before.Text[before.LastAssertionStart:], "(assert "), ")\n")
			last := after.Order.Text[after.Order.LastAssertionStart:]
			if !strings.HasPrefix(last, "(assert (and ") || strings.Contains(last, original) == tc.specialized {
				t.Fatalf("specialized = %t, want %t\n%s", !strings.Contains(last, original), tc.specialized, last)
			}
			// Each example admits a counterexample to the postcondition.
			// An auxiliary proof must preserve that counterexample.
			proved, err := s.ProveUnsatByOrderAbstraction()
			astTestMust(t, err)
			if proved {
				t.Fatal("spuriously refuted a satisfiable transition")
			}
			result, err := s.Check()
			astTestMust(t, err)
			if result != lbool.True {
				t.Fatalf("expected a concrete counterexample, got %v", result)
			}
		})
	}
}

func TestOrderProofUnchangedAccessAtOtherWitness(t *testing.T) {
	const decls = `(define-sort A () (_ BitVec 2))
(declare-const a A)(declare-const b A)(declare-const c A)(declare-const d A)(declare-const p A)
(declare-fun gate (A A) Bool)(declare-fun edge (A A) Bool)
(declare-fun old (A A) A)(declare-fun updated (A A) A)
(assert (forall ((i A)(j A)) (= (updated i j) (ite (= i p) p (old i j)))))
`
	for _, tc := range []struct {
		name, conclusion string
		changed          bool
	}{
		{"reflexive conclusion", "(= (old x w) (old y w))", true},
		{"conclusion also requires a predicate", "(and (= (old x w) (old y w)) (edge w z))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			premise := "(=> (and (gate x z) (gate y z)) " + tc.conclusion + ")"
			goal := strings.NewReplacer("old", "updated", " x", " a", " y", " b", " z", " c", " w", " d").Replace(premise)
			c, s := orderSolver(t, decls+"(assert (forall ((x A)(y A)(z A)(w A)) "+premise+"))(assert (not "+goal+"))")
			manager, err := c.Manager()
			astTestMust(t, err)
			assertions, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			result, ok, err := order.ForProof(manager, assertions, orderProofSteps)
			astTestMust(t, err)
			if !ok || result.Refuted || result.Order.Text == "" {
				t.Fatal("expected an order problem")
			}
			last := result.Order.Text[result.Order.LastAssertionStart:]
			if strings.Contains(last, "(not (= ") != tc.changed {
				t.Fatalf("unexpected ground facts in %s", last)
			}
			proved, err := s.ProveUnsatByOrderAbstraction()
			astTestMust(t, err)
			if proved {
				t.Fatal("the update at the pivot can change the property")
			}
			got, err := s.Check()
			astTestMust(t, err)
			if got != lbool.True {
				t.Fatalf("expected a concrete counterexample, got %v", got)
			}
		})
	}
}

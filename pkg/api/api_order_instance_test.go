package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
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

func TestOrderProofConditionalPointUpdate(t *testing.T) {
	const decls = `
(declare-const a A)(declare-const b A)(declare-const c A)(declare-const d A)
(declare-const p A)(declare-const q A)
(declare-fun old (A A) Bool)(declare-fun updated (A A) Bool)
(declare-fun oldD (A) Bool)(declare-fun newD (A) Bool)
(declare-fun member (A A) Bool)(declare-fun end (A) Bool)
(assert (forall ((i A)(j A)) (= (updated i j) (ite (and (= i p) (= j q)) false (old i j)))))
(assert (forall ((z A)) (= (newD z) (exists ((u A)) (member u z)))))
(assert (forall ((x A)(y A)(z A)(w A)) (=> (and (old x y) (oldD z)) (end w))))
(assert (distinct a p))(assert (old a b))(assert (member q c))(assert (not (end d)))
`
	const final = "(assert (not (=> (and (updated a b) (newD c)) (end d))))"
	for _, domain := range []struct{ name, declaration string }{
		{"two-bit", "(define-sort A () (_ BitVec 2))"},
		{"uninterpreted", "(declare-sort A 0)"},
	} {
		for _, tc := range []struct {
			name, extra string
			wantProof   bool
		}{
			{"change away from the point", "(assert (not (oldD c)))", false},
			{"one possible changed atom", "(assert (forall ((z A)) (=> (newD z) (oldD z))))", true},
		} {
			t.Run(domain.name+"/"+tc.name, func(t *testing.T) {
				c, s := orderSolver(t, domain.declaration+decls+"(assert (distinct a b))(assert (distinct a c))(assert (distinct b c))"+tc.extra+final)
				manager, err := c.Manager()
				astTestMust(t, err)
				assertions, err := solver.Assertions(s.solver)
				astTestMust(t, err)
				proof, ok, err := order.ForProof(manager, assertions, orderProofSteps)
				astTestMust(t, err)
				if !ok || proof.Conditional == nil {
					t.Fatalf("expected a conditional point update (applicable %v, refuted %v)\n%s", ok, proof.Refuted, proof.Order.Text)
				}
				// The specialized counterexample is impossible at (p,q) even
				// when the actual counterexample away from (p,q) is satisfiable.
				trial := orderProofState{source: s}
				astTestMust(t, trial.start(manager))
				t.Cleanup(func() { astTestMust(t, trial.release()) })
				established, err := trial.check(manager, proof.Conditional.Condition, 100_000, false)
				astTestMust(t, err)
				if established != tc.wantProof {
					t.Fatalf("direction check = %v, want %v", established, tc.wantProof)
				}
				batch := s.BeginOrderProofBatch()
				t.Cleanup(func() { astTestMust(t, batch.Close()) })
				got, err := batch.Prove()
				astTestMust(t, err)
				if got != tc.wantProof {
					t.Fatalf("private proof = %v, want %v", got, tc.wantProof)
				}
				result, err := s.Check()
				astTestMust(t, err)
				if result == lbool.Undef || (result == lbool.False) != tc.wantProof {
					t.Fatalf("concrete solver result %v", result)
				}
			})
		}
	}
}

// Identical goals can follow different ordered premises after a source pop.
func TestOrderTransitionPremiseCache(t *testing.T) {
	const input = `(declare-sort A 0)(declare-const a A)(declare-const active Bool)
(declare-fun r (A A A A) Bool)
(assert (forall ((x A)(y A)(z A)(w A)) (r x y z w)))
(assert (forall ((x A)) (r x x x x)))
(assert active)(assert (not (r a a a a)))`
	var cache order.TransitionPremises
	for i := 0; i < 2; i++ {
		c, s := orderSolver(t, input)
		m, err := c.Manager()
		astTestMust(t, err)
		a, err := solver.Assertions(s.solver)
		astTestMust(t, err)
		if len(a) != 4 {
			t.Fatalf("got %d assertions", len(a))
		}
		with := []ast.ASTHandle{a[0], a[2], a[3]}
		without := []ast.ASTHandle{a[1], a[2], a[3]}
		if _, ok, err := order.Relax(m, with); err != nil || ok {
			t.Fatalf("uninterpreted source accepted by bit-vector relaxation: %v, %v", ok, err)
		}
		check := func(forms []ast.ASTHandle, want bool) {
			t.Helper()
			p, ok, err := cache.ForProof(m, forms, 1000)
			astTestMust(t, err)
			if ok != want || p.Refuted != want || p.Boolean != nil {
				t.Fatalf("match %v refuted %v Boolean problem %v, want direct match %v", ok, p.Refuted, p.Boolean != nil, want)
			}
		}
		m.ResourceLimit().IncCancel()
		check(with, false)
		m.ResourceLimit().DecCancel()
		check(with, true)
		check(with, true)
		check(without, false)
		check(with, true)
	}
}

const inclusionPrefix = `(define-sort A () (_ BitVec 2))
(declare-const a A)(declare-const b A)(declare-const c A)(declare-const d A)
(declare-const p A)(declare-const q A)
(declare-fun old (A A) Bool)(declare-fun updated (A A) Bool)
(declare-fun oldD (A) Bool)(declare-fun newD (A) Bool)
(declare-fun oldP (A A) Bool)(declare-fun newP (A A) Bool)
(declare-fun edge (A A) Bool)(declare-fun end (A) Bool)
(assert (forall ((x A)(y A)) (= (updated x y) (ite (and (= x p) (= y q)) true (old x y)))))
(assert (forall ((u A)(v A)) (= (newP u v) (or (oldP u v) (edge u v)))))
(assert (forall ((z A)) (= (oldD z) (exists ((u A)) (forall ((t A)) (=> (edge t u) (not (oldP t z))))))))
`

const inclusionGoal = `
(assert (forall ((x A)(y A)(z A)(w A))
 (=> (and (old x y) (oldD z) (not (oldP w z))) (end w))))
(assert (not (=> (and (updated a b) (newD c) (not (newP d c))) (end d))))`

func TestOrderProofLiftsImplicationThroughQuantifiedPredicates(t *testing.T) {
	for _, tc := range []struct {
		name, formula string
		lift          bool
	}{
		{"opposite polarity under two binders", "(=> (edge s r) (not (newP s k)))", true},
		{"changed sign", "(=> (edge s r) (newP s k))", false},
		{"changed bound variable", "(=> (edge s r) (not (newP r k)))", false},
		{"changed fixed argument", "(=> (edge s r) (not (newP s p)))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definition := "(assert (forall ((k A)) (= (newD k) (exists ((r A)) (forall ((s A)) " + tc.formula + ")))))"
			c, s := orderSolver(t, inclusionPrefix+definition+inclusionGoal)
			manager, err := c.Manager()
			astTestMust(t, err)
			assertions, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			proof, ok, err := order.ForProof(manager, assertions, orderProofSteps)
			astTestMust(t, err)
			if !ok || proof.Conditional == nil {
				t.Fatal("missing conditional point-update proof")
			}
			side := proof.Conditional.Condition
			goal := side.Text[side.LastAssertionStart:]
			if strings.HasPrefix(goal, "(assert (not (forall ") != tc.lift {
				t.Fatalf("lift = %v, got %s", tc.lift, goal)
			}
			if tc.lift {
				check := orderProofState{source: s}
				astTestMust(t, check.start(manager))
				t.Cleanup(func() { astTestMust(t, check.release()) })
				established, err := check.check(manager, side, 100_000, false)
				astTestMust(t, err)
				if !established {
					t.Fatal("failed to prove the pointwise inclusion")
				}
			}
		})
	}
}

func TestOrderProofCommonUpdateGuard(t *testing.T) {
	const decls = `(define-sort A () (_ BitVec 2))
(declare-const a A)(declare-const b A)(declare-const c A)(declare-const d A)(declare-const p A)
(declare-const action Bool)(declare-const second Bool)
(declare-fun gate (A A) Bool)(declare-fun old (A A) A)(declare-fun next (A A) A)
(declare-fun oldFlag (A A) Bool)(declare-fun nextFlag (A A) Bool)
(assert (forall ((i A)(j A)) (= (next i j) (ite action (ite (= i p) p (old i j)) (old i j)))))
`
	const invariant = `(=> (and (gate x z) (gate y z)) (and (= (old x w) (old y w)) (= (oldFlag x w) (oldFlag y w))))`
	goal := strings.NewReplacer("old", "next", " x", " a", " y", " b", " z", " c", " w", " d").Replace(invariant)
	for _, tc := range []struct {
		name, condition string
		shared          bool
	}{
		{"shared action", "action", true},
		{"independent actions", "second", false},
		{"opposite actions", "(not action)", false},
		{"guard varies by argument", "(gate i j)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, decls+
				"(assert (forall ((i A)(j A)) (= (nextFlag i j) (ite "+tc.condition+
				" (ite (= i p) true (oldFlag i j)) (oldFlag i j)))))"+
				"(assert (forall ((x A)(y A)(z A)(w A)) "+invariant+"))(assert (not "+goal+"))")
			m, err := c.Manager()
			astTestMust(t, err)
			assertions, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			before, ok, err := order.Relax(m, assertions)
			astTestMust(t, err)
			if !ok {
				t.Fatal("expected an order relaxation")
			}
			after, ok, err := order.ForProof(m, assertions, orderProofSteps)
			astTestMust(t, err)
			if !ok || after.Refuted || after.Order.Text == "" || (before.Premises() != after.Order.Premises()) != tc.shared {
				t.Fatalf("unexpected update specialization for %s", tc.condition)
			}
			proved, err := s.ProveUnsatByOrderAbstraction()
			astTestMust(t, err)
			got, err := s.Check()
			astTestMust(t, err)
			if proved || got != lbool.True {
				t.Fatalf("expected an active transition with a counterexample, got proved=%v, concrete=%v", proved, got)
			}
		})
	}
}

func TestOrderProofSpecializesBothSidesOfAConsequence(t *testing.T) {
	const wrapper = `(assert (forall ((k A)) (= (newD k) (exists ((r A)) (forall ((s A)) (=> (edge s r) (not (newP s k))))))))`
	for _, tc := range []struct {
		name, consequence string
		projected         bool
	}{
		{"both sides match the violated predicate", "(= (end u) (end v))", true},
		{"only one side matches", "(= (end u) (oldD v))", false},
		{"equality in a disjunction", "(or (= (end u) (end v)) (edge v u))", false},
		{"identical sides", "(= (end u) (end u))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule := "(assert (forall ((u A)(v A)) (=> (edge u v) " + tc.consequence + ")))"
			c, s := orderSolver(t, inclusionPrefix+wrapper+rule+inclusionGoal)
			m, err := c.Manager()
			astTestMust(t, err)
			assertions, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			proof, ok, err := order.ForProof(m, assertions, orderProofSteps)
			astTestMust(t, err)
			if !ok || proof.Conditional == nil {
				t.Fatal("expected a conditional transition")
			}
			primary := proof.Conditional.Primary.Premises()
			if (primary != proof.Order.Premises()) != tc.projected {
				t.Fatalf("unexpected primary premises for %s", tc.consequence)
			}
			if tc.projected && strings.Contains(primary, "(exists ") {
				t.Fatal("the projected primary query retained a quantified wrapper")
			}
			proved, err := s.ProveUnsatByOrderAbstraction()
			astTestMust(t, err)
			got, err := s.Check()
			astTestMust(t, err)
			if proved || got != lbool.True {
				t.Fatalf("expected a counterexample, got proved=%v, concrete=%v", proved, got)
			}
		})
	}
}

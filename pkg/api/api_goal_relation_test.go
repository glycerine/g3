package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGoalRelationSelectsSingleSortOfViolatedPostcondition(t *testing.T) {
	const declarations = `
(declare-sort A 0) (declare-sort B 0)
(declare-const a A) (declare-const b A) (declare-const c A)
(declare-const x B) (declare-const y B) (declare-const z B)
(declare-const saved Bool)
(declare-fun r (A A) Bool) (declare-fun s (B B) Bool) (declare-fun mix (A B) Bool)
(declare-fun f (A A) A)
(assert (forall ((v A)) (r v v)))
`
	for _, tc := range []struct {
		name, query string
		admitted    bool
	}{
		{"same domain", "(not (=> saved (and (r a b) (r b c))))", true},
		{"nested guards", "(and (not (=> saved (=> (r a b) (and (r b c) saved)))))", true},
		{"fewer constants", "(not (=> saved (and (r a b) (r b a))))", false},
		{"two domains", "(not (=> saved (and (r a b) (r b c) (s x y) (s y z))))", false},
		{"mixed arguments", "(not (=> saved (and (mix a x) (mix b y) (mix c z))))", false},
		{"non-Boolean result", "(not (=> saved (and (= (f a b) c) (= (f c a) b))))", false},
		{"positive implication", "(=> saved (and (r a b) (r b c)))", false},
		{"negated conjunction", "(not (and (r a b) (r b c)))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, declarations+"(assert "+tc.query+")")
			m, err := c.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			_, p, err := planGroundGoals(m, roots, false)
			astTestMust(t, err)
			if got := p != nil && p.selectedOnly; got != tc.admitted {
				t.Fatalf("selected relation plan = %t, want %t", got, tc.admitted)
			}
		})
	}
}

func TestGroundRelationSelectionRequiresSmallerQuantifiedBase(t *testing.T) {
	const declarations = `(declare-sort A 0)
(declare-const a A)(declare-const b A)(declare-const c A)
(declare-const saved Bool)(declare-const guard Bool)
(declare-fun r (A A) Bool)(declare-fun s (A A) Bool)
(declare-fun u (A) Bool)(declare-fun v (A) Bool)
(declare-const g0 Bool)(declare-const g1 Bool)(declare-const g2 Bool)
(assert g0)(assert g1)(assert g2)
(assert (forall ((x A)(y A)) (s x y)))
(assert (forall ((x A)) (u x)))`
	const r = "(assert (forall ((x A)(y A)) (r x y)))"
	const guarded = "(assert (forall ((x A)(y A)) (=> guard (r x y))))"
	const unrelated = "(assert (forall ((x A)) (v x)))"
	const goal = "(assert (not (=> saved (and (r a b) (s b c)))))"
	for _, tc := range []struct {
		name, premises string
		selected       bool
		outcome        lbool.LBool
	}{
		{"two of three", r, false, lbool.False},
		{"two of four", r + unrelated, true, lbool.False},
		{"guarded two of four", guarded + unrelated, true, lbool.True},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, source := orderSolver(t, declarations+tc.premises+goal)
			m, err := c.Manager()
			astTestMust(t, err)
			forms, err := solver.Assertions(source.solver)
			astTestMust(t, err)
			_, plan, err := planGroundGoals(m, forms, false)
			astTestMust(t, err)
			if plan == nil || !plan.selectedOnly {
				t.Fatal("expected a ground relation plan")
			}
			p := orderProofState{source: source}
			trial, err := p.selectWitnessPremises(m, plan)
			astTestMust(t, err)
			if (trial != nil) != tc.selected {
				t.Fatalf("selected = %t, want %t", trial != nil, tc.selected)
			}
			if trial != nil {
				yes, err := p.checkProofPlan(m, trial, orderProofSteps)
				astTestMust(t, err)
				if yes != (tc.outcome == lbool.False) {
					t.Fatalf("private refutation = %t, concrete outcome should be %v", yes, tc.outcome)
				}
			}
			got, err := source.Check()
			astTestMust(t, err)
			if got != tc.outcome {
				t.Fatalf("concrete outcome = %v, want %v", got, tc.outcome)
			}
		})
	}
}

package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/solver"
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
			_, p, err := planGroundGoals(m, roots)
			astTestMust(t, err)
			if got := p != nil && p.selectedOnly; got != tc.admitted {
				t.Fatalf("selected relation plan = %t, want %t", got, tc.admitted)
			}
		})
	}
}

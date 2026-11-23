package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestWitnessInstancesThreeTerms(t *testing.T) {
	const decl = witnessTestDeclarations + "(declare-const d A)(declare-fun edge (A A) Bool)"
	const trans = "(forall ((x A)(y A)(z A)) (=> (and (edge x y) (edge y z)) (edge x z)))"
	const query = "(assert (and (not (forall ((x A)) (bad x))) (not (exists ((x A)) (excluded x))) (edge a b) (edge b d) (not (edge a d))))"
	for _, tc := range []struct {
		name, rule string
		unsat      bool
	}{
		{"three distinct terms", trans, true},
		{"unselected transitivity", "(or switch " + trans + ")", false},
		{"three-binder relation without transitivity", "(forall ((x A)(y A)(z A)) (or (edge x y) (not (edge x y)) (edge y z)))", false},
		{"mixed binder sorts", "(forall ((x A)(y A)(z B)) (or (rel z x) (not (rel z x)) (edge y x)))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, decl+"(assert "+tc.rule+")"+query)
			fs, e := solver.Assertions(s.solver)
			astTestMust(t, e)
			m, e := c.Manager()
			astTestMust(t, e)
			plan, e := planWitnessInstances(m, fs)
			astTestMust(t, e)
			if plan == nil || plan.rounds != 1 {
				t.Fatal("expected a single-round plan for the two negated quantifiers")
			}
			p := orderProofState{source: s}
			yes, e := p.checkProofPlan(m, plan, orderProofSteps)
			astTestMust(t, e)
			if yes != tc.unsat {
				t.Fatalf("ground refutation=%v, expected %v", yes, tc.unsat)
			}
			want := lbool.True
			if tc.unsat {
				want = lbool.False
			}
			got, e := s.Check()
			astTestMust(t, e)
			if got != want {
				t.Fatalf("original check=%v, expected %v", got, want)
			}
		})
	}
}

func TestWitnessInstancesGroundDomains(t *testing.T) {
	const decl = `(declare-sort A 0)(declare-sort B 0)(declare-sort C 0)(declare-sort D 0)
(declare-const a A)(declare-const b B)(declare-const c C)(declare-const d D)
(declare-const a2 A)(declare-fun p (A B C) Bool)(declare-fun q (D) Bool)(declare-fun r (A) Bool)`
	for _, tc := range []struct {
		name, tail string
		eligible   bool
	}{
		{"one constant per sort", "", true},
		{"ambiguous A", "(r a2)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, decl+"(assert (forall ((x A)) (not (r x))))(assert (and (p a b c) (q d) (r a) "+tc.tail+"))")
			fs, e := solver.Assertions(s.solver)
			astTestMust(t, e)
			m, e := c.Manager()
			astTestMust(t, e)
			simple, ext, e := planGroundGoals(m, fs, false)
			astTestMust(t, e)
			if simple != nil || (ext != nil) != tc.eligible {
				t.Fatalf("simple=%v expanded=%v", simple, ext)
			}
			if ext != nil {
				p := orderProofState{source: s}
				_, narrow, e := planGroundGoals(m, fs, true)
				astTestMust(t, e)
				if ext.width != 10 || ext.rounds != 2 || narrow == nil || narrow.width != 1 || narrow.rounds != 1 {
					t.Fatal("unexpected witness search before or after the first push")
				}
				for _, plan := range []*witnessInstancePlan{ext, narrow} {
					yes, e := p.checkProofPlan(m, plan, orderProofSteps)
					astTestMust(t, e)
					if !yes {
						t.Fatalf("missed universal at a with width %d", plan.width)
					}
				}
			}
		})
	}
}

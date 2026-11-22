package api

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const groundPremiseDecls = `(declare-sort U 0)(declare-const a U)(declare-const b U)
(declare-fun p (U) Bool)(declare-fun q (U) Bool)(declare-fun f (U) U)`

func TestGroundPremisesSiblingConsequences(t *testing.T) {
	const rule = "(assert (forall ((x U)) (=> (p x) (q x))))"
	const reverse = "(assert (forall ((x U)) (=> (q x) (p x))))"
	cases := []struct {
		goal    string
		refuted bool
	}{
		{"(not (=> (p a) (q a)))", true},
		{"(p a)", false},
		{"(not (q a))", false},
		{"(not (or (not (p a)) (q a)))", true},
		{"(or (p a) (not (q a)))", false},
		{"(=> (p a) (not (q a)))", false},
		{"(and (p a) (not (q a)))", true},
		{"(and (p b) (not (q a)))", false},
		{"(not (and (p a) (q a)))", false},
	}
	var inputs string
	for _, tc := range cases {
		inputs += "(assert " + tc.goal + ")"
	}
	c, s := orderSolver(t, groundPremiseDecls+rule+reverse+inputs)
	m, e := c.Manager()
	astTestMust(t, e)
	roots, e := solver.Assertions(s.solver)
	astTestMust(t, e)
	var proof order.GroundPremises
	check := func(manager *ast.ASTManager, xs []ast.ASTHandle, steps uint32, want bool) {
		t.Helper()
		got, e := proof.Refutes(manager, xs, steps)
		if e != nil || got != want {
			t.Fatalf("ground proof %v, %v; want %v", got, e, want)
		}
	}
	for pass := 0; pass < 2; pass++ {
		for i, tc := range cases {
			check(m, []ast.ASTHandle{roots[0], roots[2+i]}, orderProofSteps, tc.refuted)
			if pass == 0 {
				_, concrete := orderSolver(t, groundPremiseDecls+rule+"(assert "+tc.goal+")")
				ans, e := concrete.Check()
				astTestMust(t, e)
				want := lbool.True
				if tc.refuted {
					want = lbool.False
				}
				if ans != want {
					t.Fatalf("%s: concrete answer %v, want %v", tc.goal, ans, want)
				}
			}
		}
	}
	xs := []ast.ASTHandle{roots[0], roots[2]}
	check(m, xs, 1, false)
	m.ResourceLimit().IncCancel()
	check(m, xs, orderProofSteps, false)
	m.ResourceLimit().DecCancel()
	check(m, xs, orderProofSteps, true)
	check(m, []ast.ASTHandle{roots[1], roots[2]}, orderProofSteps, false)
	check(m, xs, orderProofSteps, true)
	c2, s2 := orderSolver(t, groundPremiseDecls+reverse+"(assert "+cases[0].goal+")")
	m2, e := c2.Manager()
	astTestMust(t, e)
	other, e := solver.Assertions(s2.solver)
	astTestMust(t, e)
	check(m2, other, orderProofSteps, false)
	check(m, xs, orderProofSteps, true)
}

func TestGroundPremisesQuantifierPolarity(t *testing.T) {
	for _, tc := range []struct {
		premise, goal string
		want          bool
	}{
		{"(not (exists ((x U)) (p x)))", "(p a)", true},
		{"(exists ((x U)) (p x))", "(not (p a))", false},
		{"(not (forall ((x U)) (p x)))", "(p a)", false},
		{"(forall ((x U)) (p x))", "(not (p a))", true},
		{"(forall ((x U)) (= (f x) x))", "(not (= a (f a)))", true},
		{"(forall ((x U)) (= (f x) x))", "(not (= b a))", false},
		{"(forall ((x U)) (=> (p x) false))", "(not (p a))", false},
	} {
		c, s := orderSolver(t, groundPremiseDecls+"(assert "+tc.premise+")(assert "+tc.goal+")")
		m, e := c.Manager()
		astTestMust(t, e)
		roots, e := solver.Assertions(s.solver)
		astTestMust(t, e)
		var proof order.GroundPremises
		got, e := proof.Refutes(m, roots, orderProofSteps)
		astTestMust(t, e)
		concrete, e := s.Check()
		astTestMust(t, e)
		if got != tc.want || (concrete == lbool.False) != tc.want || concrete == lbool.Undef {
			t.Fatalf("%s, %s: proof %v and concrete %v, want refutation %v", tc.premise, tc.goal, got, concrete, tc.want)
		}
	}
}

func TestGroundPremisesLeavesArithmeticBindersToSolver(t *testing.T) {
	c, s := orderSolver(t, `(declare-fun p (Int) Bool)(declare-const a Int)
(assert (forall ((x Int)) (p x)))(assert (not (p a)))`)
	m, err := c.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	var proof order.GroundPremises
	got, err := proof.Refutes(m, roots, orderProofSteps)
	astTestMust(t, err)
	if got {
		t.Fatal("ground projection accepted an arithmetic binder")
	}
	answer, err := s.Check()
	astTestMust(t, err)
	if answer != lbool.False {
		t.Fatal("ordinary arithmetic solver", answer)
	}
}

// Time unit propagation through a chain with two sibling goals. One goal
// contradicts the last implication, the other leaves a satisfying assignment.
func BenchmarkGroundPremisesSiblings(b *testing.B) {
	var input strings.Builder
	input.WriteString("(declare-sort U 0)(declare-const a U)")
	for i := 0; i <= 32; i++ {
		fmt.Fprintf(&input, "(declare-fun p%d (U) Bool)", i)
	}
	for i := 0; i < 32; i++ {
		fmt.Fprintf(&input, "(assert (forall ((x U)) (=> (p%d x) (p%d x))))", i, i+1)
	}
	input.WriteString("(assert (and (p0 a) (not (p32 a))))(assert (and (p32 a) (not (p0 a))))")
	m, roots := benchmarkOrderAssertions(b, input.String(), (*SMTContext).NewSolver)
	base := roots[:len(roots)-2]
	assertions := append(slices.Clone(base), roots[len(base)])
	for _, reuse := range []bool{false, true} {
		name := "Rebuild"
		if reuse {
			name = "Reuse"
		}
		b.Run(name, func(b *testing.B) {
			var p order.GroundPremises
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if !reuse {
					p = order.GroundPremises{}
				}
				assertions[len(base)] = roots[len(base)+i%2]
				yes, err := p.Refutes(m, assertions, orderProofSteps)
				if err != nil || yes != (i%2 == 0) {
					b.Fatal("sibling", i%2, yes, err)
				}
			}
		})
	}
}

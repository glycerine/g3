package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGuardedPredicateAgreement(t *testing.T) {
	const decls = `(declare-sort U 0)(declare-const a U)(declare-const b U)
(declare-const g Bool)(declare-fun p (U U) Bool)(declare-fun q (U U) Bool)
(declare-fun f (U) U)(declare-fun h (U) U)
(assert (forall ((x U) (y U)) (=> g (= (p x y) (q x y)))))`
	for _, tc := range []struct {
		name, query string
		match       bool
		concrete    lbool.LBool
	}{
		{"guarded", "(not (=> g (= (p a b) (q a b))))", true, lbool.False},
		{"different guard", "(not (=> (not g) (= (p a b) (q a b))))", true, lbool.True},
		{"positive", "(=> g (= (p a b) (q a b)))", false, lbool.True},
		{"swapped arguments", "(not (=> g (= (p a b) (q b a))))", false, lbool.True},
		{"compound argument", "(not (=> g (= (p a (f b)) (q a (f b)))))", false, lbool.False},
		{"data equality", "(not (=> g (= (f a) (h a))))", false, lbool.True},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, s := orderSolver(t, decls+"(assert "+tc.query+")")
			m, err := ctx.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			if got := guardedPredicateAgreement(m, roots[len(roots)-1]); got != tc.match {
				t.Fatalf("matched %t, want %t", got, tc.match)
			}
			proved, err := provePredicateAgreement(s, m, roots, orderProofSteps)
			astTestMust(t, err)
			if proved != (tc.match && tc.concrete == lbool.False) {
				t.Fatalf("unexpected ground proof result %t", proved)
			}
			answer, err := s.Check()
			astTestMust(t, err)
			if answer != tc.concrete {
				t.Fatalf("concrete solver returned %v, want %v", answer, tc.concrete)
			}
		})
	}
}

func BenchmarkGuardedPredicateAgreement(b *testing.B) {
	ctx, s := orderSolver(b, verificationBenchmarkScript(b, "ivy_apple", "ticket_guarded_predicate_agreement"))
	m, err := ctx.Manager()
	astTestMust(b, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(b, err)
	for _, mode := range []string{"Ground", "Ordinary"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				target, err := ctx.NewSolver()
				astTestMust(b, err)
				for _, h := range roots[:len(roots)-1] {
					astTestMust(b, target.Assert(h))
				}
				astTestMust(b, target.Push())
				astTestMust(b, target.Assert(roots[len(roots)-1]))
				b.StartTimer()
				if mode == "Ground" {
					proved, err := provePredicateAgreement(target, m, roots, orderProofSteps)
					if err != nil || !proved {
						b.Fatal(proved, err)
					}
				} else {
					answer, err := target.Check()
					if err != nil || answer != lbool.False {
						b.Fatal(answer, err)
					}
				}
				b.StopTimer()
				astTestMust(b, target.Close())
			}
		})
	}
}

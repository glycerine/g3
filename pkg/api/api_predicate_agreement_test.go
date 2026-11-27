package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
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
			proved, err := (&orderProofState{source: s}).proveGuardedGoal(m, roots, orderProofSteps)
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
					proved, err := (&orderProofState{source: target}).proveGuardedGoal(m, roots, orderProofSteps)
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

func TestGuardedEqualityCaseSiblings(t *testing.T) {
	const script = `(declare-sort U 0)(declare-const a U)(declare-const b U)
(declare-fun r (U U) Bool)(declare-const g Bool)(declare-const h Bool)
(assert (forall ((x U)(y U)) (=> (r x y) (= x y))))
(assert (not (=> (and g (r a b)) (or (and g (= a b)) (and h g)))))
(assert (not (=> (and g h) (or (and (not g) (= a b)) (and (not h) g)))))
(assert (not (=> (and g (r b a)) (or (or (and h g) (and g (= b a))) (and h (not g))))))
(assert (not (=> (and g h) (or (and g (= a b)) h))))`
	ctx, s := orderSolver(t, script)
	m, err := ctx.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	batch := s.BeginOrderProofBatchForGoals(64)
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	for i, want := range []bool{true, false, true, false} {
		goal := roots[i+1]
		if got := guardedEqualityCases(m, goal); got != (i < 3) {
			t.Fatalf("goal %d matched %t", i, got)
		}
		ok, err := batch.state.proveGuardedGoal(m, []ast.ASTHandle{roots[0], goal}, orderProofSteps)
		if err != nil || ok != want {
			t.Fatalf("goal %d proved %t, want %t, error %v", i, ok, want, err)
		}
	}
	// Replacing the universal premise removes the reason that a and b agree.
	ok, err := batch.state.proveGuardedGoal(m, []ast.ASTHandle{roots[2], roots[1]}, orderProofSteps)
	if err != nil || ok {
		t.Fatal("proof survived removal of its premise", ok, err)
	}
}

func BenchmarkGuardedEqualityCaseSiblings(b *testing.B) {
	ctx, donor := orderSolver(b, verificationBenchmarkScript(b, "ivy_apple", "ticket_guarded_case_siblings"))
	m, err := ctx.Manager()
	astTestMust(b, err)
	roots, err := solver.Assertions(donor.solver)
	astTestMust(b, err)
	if len(roots) != 3 {
		b.Fatal("expected a premise and two goals")
	}
	for _, mode := range []string{"Shared", "Fresh", "Ordinary"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				s, err := ctx.NewSolver()
				astTestMust(b, err)
				astTestMust(b, s.Assert(roots[0]))
				batch := s.BeginOrderProofBatchForGoals(64)
				for _, goal := range roots[1:] {
					astTestMust(b, s.Push())
					astTestMust(b, s.Assert(goal))
					if mode == "Ordinary" {
						got, err := s.Check()
						if err != nil || got != lbool.False {
							b.Fatal(got, err)
						}
					} else {
						ok, err := batch.state.proveGuardedGoal(m, []ast.ASTHandle{roots[0], goal}, orderProofSteps)
						if err != nil || !ok {
							b.Fatal(ok, err)
						}
						if mode == "Fresh" {
							astTestMust(b, batch.Close())
							batch = s.BeginOrderProofBatchForGoals(64)
						}
					}
					astTestMust(b, s.Pop(1))
				}
				astTestMust(b, batch.Close())
				astTestMust(b, s.Close())
			}
		})
	}
}

func TestGuardedCaseUsesWitnessesAbsentFromGoal(t *testing.T) {
	for _, tc := range []struct{ fact, premise string }{{"(r c d)", "(r x y)"}, {"(not (r c d))", "(not (r x y))"}} {
		fact, premise := tc.fact, tc.premise
		ctx, s := orderSolver(t, `(declare-sort U 0)(declare-const a U)(declare-const b U)
(declare-const c U)(declare-const d U)(declare-fun r (U U) Bool)
(declare-const g Bool)(declare-const h Bool)(declare-const q Bool)
(assert (forall ((x U)(y U)) (=> `+premise+` q)))
(assert `+fact+`)
(assert (not (=> (and g h) (or (and q g) (and h (= a b))))))`)
		m, err := ctx.Manager()
		astTestMust(t, err)
		roots, err := solver.Assertions(s.solver)
		astTestMust(t, err)
		var ordinary groundInstanceSolver
		ok, err := ordinary.prove(s, m, roots, orderProofSteps, 1)
		astTestMust(t, err)
		astTestMust(t, ordinary.close())
		if ok {
			t.Fatal("goal constants alone unexpectedly sufficed", fact)
		}
		batch := s.BeginOrderProofBatchForGoals(64)
		ok, err = batch.state.proveGuardedGoal(m, roots, orderProofSteps)
		astTestMust(t, err)
		astTestMust(t, batch.Close())
		if !ok {
			t.Fatal("missed witnesses in premise", fact)
		}
	}
}

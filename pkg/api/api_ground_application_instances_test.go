package api

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

func TestGroundApplicationInstancesAcrossGoals(t *testing.T) {
	const script = `(declare-sort U 0)
(declare-sort V 0)
(declare-const a U)
(declare-const b U)
(declare-const c V)
(declare-fun f (U) U)
(declare-fun r (U V) Bool)
(declare-fun p (U) Bool)
(declare-fun q (V U) Bool)
(assert (forall ((x U) (y V)) (=> (r x y) (q y (f x)))))
(assert (forall ((j V) (i U)) (=> (q j i) (p i))))
(assert (forall ((i U)) (=> (p i) (p i))))
(assert (r a c))
(assert (not (p (f a))))
(assert (not (p (f b))))
(assert (q c (f b)))
(assert (not (q c (f b))))`
	ctx, s := orderSolver(t, script)
	m, err := ctx.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	var private groundInstanceSolver
	t.Cleanup(func() { astTestMust(t, private.close()) })
	check := func(base []ast.ASTHandle, goal ast.ASTHandle, want bool) {
		t.Helper()
		query := append(append([]ast.ASTHandle(nil), base...), goal)
		got, err := private.proveObserved(s, m, query, 350_000)
		if err != nil || got != want {
			t.Fatalf("base size %d, goal %v: proved %t, want %t, error %v", len(base), goal, got, want, err)
		}
		if private.sat.solver != nil {
			t.Fatal("application matching allocated a residual SAT solver")
		}
	}
	base := roots[:4:4]
	notPa, notPb, qb, notQb := roots[4], roots[5], roots[6], roots[7]
	check(base, notPb, false)
	// Seeing q(c,f(b)) in a goal must not turn it into a fact for its sibling.
	check(base, qb, false)
	check(base, notQb, false)
	withQb := append(base, qb)
	check(withQb, qb, false)
	check(withQb, notPb, true)
	check(base, notPb, false)
	m.ResourceLimit().IncCancel()
	check(base, notPa, false)
	m.ResourceLimit().DecCancel()
	// The r(a,c) goal supplies arguments of sorts U,V. Its rule creates a q
	// application with sorts V,U, which supplies the second rule's arguments.
	check(base, roots[3], false)
	check(base, notPa, true)
}

func TestGroundApplicationVerificationBatch(t *testing.T) {
	input := verificationBenchmarkScript(t, "ivy_apple", "hermes_application_batch_5")
	ctx, donor := orderSolver(t, input)
	m, err := ctx.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(donor.solver)
	astTestMust(t, err)
	for _, count := range []int{23, len(roots) - 1} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			target, err := ctx.NewSolver()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, target.Close()) })
			astTestMust(t, target.Assert(roots[0]))
			batch := target.BeginOrderProofBatchForGoals(count)
			t.Cleanup(func() { astTestMust(t, batch.Close()) })
			for _, goal := range roots[1:] {
				_, err := batch.ProveWithGoal(goal)
				astTestMust(t, err)
			}
			want := count >= 24 && batch.state.plans.reusable(m) && !ast.ASTValidateHandles
			used := batch.state.instances != nil && batch.state.instances.pool != nil
			if used != want {
				t.Fatalf("%d announced goals: application pool used %t, want %t", count, used, want)
			}
			left, err := solver.Assertions(target.solver)
			astTestMust(t, err)
			level, err := target.ScopeLevel()
			astTestMust(t, err)
			if len(left) != 1 || left[0] != roots[0] || level != 0 {
				t.Fatal("a projected goal modified the source solver", left, level)
			}
		})
	}
}

// Each fixture starts with the permanent base. The remaining assertions are
// separate sibling goals from a single Hermes batch, in verifier order.
func BenchmarkGroundApplicationWorkerMemory(b *testing.B) {
	for _, batch := range []string{"2", "5", "18"} {
		for _, workers := range []int{1, 8, 32} {
			b.Run("batch-"+batch+"/solvers-"+strconv.Itoa(workers), func(b *testing.B) {
				input := verificationBenchmarkScript(b, "ivy_apple", "hermes_application_batch_"+batch)
				benchmarkGroundWorkerMemory(b, input, workers, true)
			})
		}
	}
}

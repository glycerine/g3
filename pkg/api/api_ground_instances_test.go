package api

import (
	"github.com/glycerine/g3/pkg/util/control"
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Assuming r(x) leaves four binary clauses whose conjunction is false.
// Dropping any one rule admits a model. The goal is deliberately absent
// from the permanent assertions so a second goal can ask for its opposite.
const branchingGroundInstances = `(declare-sort U 0)
(declare-const a U)
(declare-const b U)
(declare-fun p (U) Bool)
(declare-fun q (U) Bool)
(declare-fun r (U) Bool)
(assert (forall ((x U)) (=> (r x) (or (p x) (q x)))))
(assert (forall ((x U)) (=> (r x) (or (not (p x)) (q x)))))
(assert (forall ((x U)) (=> (r x) (or (p x) (not (q x))))))
(assert (forall ((x U)) (=> (r x) (or (not (p x)) (not (q x))))))
(assert (r a))
(assert (not (r a)))
(assert (r b))`

func TestGroundInstanceSolverChangesPremisesAndAssumptions(t *testing.T) {
	ctx, s := orderSolver(t, branchingGroundInstances)
	m, err := ctx.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	var private groundInstanceSolver
	t.Cleanup(func() { astTestMust(t, private.close()) })
	check := func(base []ast.ASTHandle, goal ast.ASTHandle, steps uint32, want bool) {
		t.Helper()
		xs := append(append([]ast.ASTHandle(nil), base...), goal)
		yes, err := private.prove(s, m, xs, steps, 1)
		if err != nil || yes != want {
			t.Fatalf("base size %d, goal %v, budget %d: proved %t, want %t, error %v", len(base), goal, steps, yes, want, err)
		}
		if len(m.ResourceLimit().Children) != 0 || private.sat.limit.RunSignals != nil || len(private.sat.limit.Limits) != 0 {
			t.Fatal("a query retained its private resource scope")
		}
	}
	base := roots[:4]
	check(base, roots[4], 2_000_000, true)
	if private.sat.solver == nil || private.installed == 0 {
		t.Fatal("expected a residual SAT search")
	}
	check(base, roots[5], 2_000_000, false)
	check(base, roots[6], 2_000_000, true)
	check(roots[:3], roots[4], 2_000_000, false)
	check(base, roots[4], 2_000_000, true)
	m.ResourceLimit().IncCancel()
	check(base, roots[4], 2_000_000, false)
	m.ResourceLimit().DecCancel()
	free, err := m.GetQuantifierBody(base[0])
	astTestMust(t, err)
	check(append(append([]ast.ASTHandle(nil), base...), free), roots[4], 2_000_000, false)
	check(base, roots[4], 2_000_000, true)
	done := make(chan struct{})
	close(done)
	parent := m.ResourceLimit()
	oldSignals := parent.RunSignals
	parent.RunSignals = &control.RunSignals{Done: done}
	check(base, roots[4], 2_000_000, false)
	parent.RunSignals = oldSignals

	// Force an interrupted import of the already compiled problem, then
	// rebuild its residual solver from row zero under a fresh resource scope.
	q, err := private.pool.Project(roots[4], 2_000_000, 1, private.sat.limit)
	astTestMust(t, err)
	astTestMust(t, private.resetSAT())
	astTestMust(t, private.sat.limit.Push(1))
	yes, err := private.search(s, m, q)
	private.sat.limit.Pop()
	if yes || err != nil || private.sat.solver != nil || private.installed != 0 || private.units != 0 {
		t.Fatalf("interrupted installation: proved %t, solver %p, rows %d, units %d, error %v", yes, private.sat.solver, private.installed, private.units, err)
	}
	check(base, roots[4], 2_000_000, true)
	check(base, roots[5], 2_000_000, false)
}

func TestGroundInstanceVerificationBatch(t *testing.T) {
	ctx, donor := orderSolver(t, appleBenchmarkInput(t, "tlb_ground_instance_siblings"))
	m, err := ctx.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(donor.solver)
	astTestMust(t, err)
	target, err := ctx.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, target.Close()) })
	astTestMust(t, target.Assert(roots[0]))
	short := target.BeginOrderProofBatchForGoals(16)
	if short.state.instances != nil {
		t.Fatal("short batch allocated a ground instance solver")
	}
	astTestMust(t, short.Close())
	b := target.BeginOrderProofBatchForGoals(64)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	live := b.state.plans.reusable(m) && !ast.ASTValidateHandles
	var proofs int
	for _, goal := range roots[1:] {
		yes, err := b.ProveWithGoal(goal)
		astTestMust(t, err)
		if yes {
			proofs++
			continue
		}
		astTestMust(t, target.Push())
		astTestMust(t, target.Assert(goal))
		yes, err = b.Prove()
		astTestMust(t, err)
		if yes {
			proofs++
		} else {
			r, err := target.Check()
			astTestMust(t, err)
			if r != lbool.False {
				t.Fatalf("concrete solver returned %v for an UNSAT sibling", r)
			}
		}
		astTestMust(t, target.Pop(1))
	}
	if live && (b.state.instances == nil || b.state.instances.pool == nil || proofs == 0) {
		t.Fatal("eligible production batch did not reach the ground instance proof", proofs)
	}
	left, err := solver.Assertions(target.solver)
	astTestMust(t, err)
	level, err := target.ScopeLevel()
	astTestMust(t, err)
	if len(left) != 1 || left[0] != roots[0] || level != 0 {
		t.Fatal("verification changed the base", left, level)
	}
}

func BenchmarkVerificationGroundInstanceSiblings(b *testing.B) {
	ctx, source := orderSolver(b, appleBenchmarkInput(b, "tlb_ground_instance_siblings"))
	m, err := ctx.Manager()
	astTestMust(b, err)
	roots, err := solver.Assertions(source.solver)
	astTestMust(b, err)
	b.ReportAllocs()
	var proved int
	var pair [2]ast.ASTHandle
	pair[0] = roots[0]
	for b.Loop() {
		var private groundInstanceSolver
		for _, goal := range roots[1:] {
			pair[1] = goal
			yes, err := private.prove(source, m, pair[:], 2_000_000, 192)
			astTestMust(b, err)
			if yes {
				proved++
			}
		}
		astTestMust(b, private.close())
	}
	b.ReportMetric(float64(len(roots)-1), "goals/op")
	b.ReportMetric(float64(proved)/float64(b.N), "refutations/op")
}

// Measure the private projection above already loaded, independent source
// solvers. Run with -cpu=1 and -benchtime=1x for a resident-heap observation.
func BenchmarkGroundInstanceWorkerMemory(b *testing.B) {
	const workers = 8
	input := appleBenchmarkInput(b, "tlb_ground_instance_siblings")
	type worker struct {
		manager *ast.ASTManager
		source  *APISimpleSolver
		roots   []ast.ASTHandle
		pool    groundInstanceSolver
	}
	owners := make([]worker, workers)
	for i := range owners {
		ctx, source := orderSolver(b, input)
		m, err := ctx.Manager()
		astTestMust(b, err)
		roots, err := solver.Assertions(source.solver)
		astTestMust(b, err)
		owners[i] = worker{manager: m, source: source, roots: roots}
	}
	var active, idle float64
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		base := raftLiveHeap()
		b.StartTimer()
		for i := range owners {
			w := &owners[i]
			pair := [2]ast.ASTHandle{w.roots[0]}
			for _, goal := range w.roots[1:] {
				pair[1] = goal
				_, err := w.pool.prove(w.source, w.manager, pair[:], 2_000_000, 192)
				astTestMust(b, err)
			}
		}
		b.StopTimer()
		active += raftLiveHeap() - base
		for i := range owners {
			astTestMust(b, owners[i].pool.close())
		}
		idle += raftLiveHeap() - base
	}
	runtime.KeepAlive(owners)
	b.ReportMetric(active/float64(workers*b.N), "active-B/solver")
	b.ReportMetric(idle/float64(workers*b.N), "released-B/solver")
}

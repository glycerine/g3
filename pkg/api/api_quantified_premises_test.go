package api

import (
	"runtime"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

func TestQuantifiedPremisesAcrossSiblingGoals(t *testing.T) {
	var cached order.QuantifiedPremises
	for owner := 0; owner < 2; owner++ {
		c, s := orderSolver(t, quantifiedGoalDeclarations+
			"(assert (p a))(assert (not (r a b)))(assert (forall ((x A)) (p x)))(assert (exists ((x B)) (r (f a) x)))")
		m, err := c.Manager()
		astTestMust(t, err)
		roots, err := solver.Assertions(s.solver)
		astTestMust(t, err)
		check := func(rows []ast.ASTHandle, budget uint32, admitted bool) {
			t.Helper()
			want, err := order.ProjectQuantifiedGoal(m, rows, budget)
			astTestMust(t, err)
			got, err := cached.Project(m, rows, budget)
			astTestMust(t, err)
			if (got != nil) != admitted || (want != nil) != admitted {
				t.Fatalf("projection admission at budget %d: cached %v, fresh %v, want %v", budget, got != nil, want != nil, admitted)
			}
			if got != nil && (got.Vars != want.Vars || got.Work != want.Work || !slices.Equal(got.Clauses, want.Clauses)) {
				t.Fatalf("changing the goal or premises changed the projected clauses or work: cached %+v, fresh %+v", got, want)
			}
		}
		first := roots[:3]
		otherGoal := []ast.ASTHandle{roots[0], roots[1], roots[3]}
		reordered := []ast.ASTHandle{roots[1], roots[0], roots[3]}
		for _, rows := range [][]ast.ASTHandle{first, otherGoal, reordered, roots[1:], first} {
			check(rows, orderProofSteps, true)
		}
		check(first, 2, false)
		check(otherGoal, orderProofSteps, true)
		m.ResourceLimit().IncCancel()
		check(first, orderProofSteps, false)
		m.ResourceLimit().DecCancel()
		check(first, orderProofSteps, true)
	}
}

func appleQuantifiedPremisesFixture(b *testing.B) (*ast.ASTManager, []ast.ASTHandle) {
	b.Helper()
	input := appleBenchmarkInput(b, "dramc_nb2-71")
	c, err := NewSMTContext()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { astTestMust(b, c.Close()) })
	s, err := c.NewSolver()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { astTestMust(b, s.Close()) })
	if err = s.LoadSMT2String(input); err != nil {
		b.Fatal(err)
	}
	m, err := c.Manager()
	if err != nil {
		b.Fatal(err)
	}
	roots, err := solver.Assertions(s.solver)
	if err != nil {
		b.Fatal(err)
	}
	return m, roots
}

func BenchmarkAppleQuantifiedPremises(b *testing.B) {
	m, roots := appleQuantifiedPremisesFixture(b)
	for _, mode := range []string{"Fresh", "Reuse"} {
		b.Run(mode, func(b *testing.B) {
			var premises *order.QuantifiedPremises
			if mode == "Reuse" {
				premises = &order.QuantifiedPremises{}
			}
			ref, err := premises.Project(m, roots, orderProofSteps)
			if err != nil || ref == nil {
				b.Fatalf("initial projection: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got, err := premises.Project(m, roots, orderProofSteps)
				if err != nil || got == nil || got.Work != ref.Work || len(got.Clauses) != len(ref.Clauses) {
					b.Fatalf("repeated projection: %v", err)
				}
			}
		})
	}
}

// Measure what remains reachable while consumers hold the emitted problems.
// All projections share the source AST so its storage is outside the delta.
func BenchmarkAppleQuantifiedProblemLiveHeap(b *testing.B) {
	m, roots := appleQuantifiedPremisesFixture(b)
	const batch = 16
	var total int64
	for i := 0; i < b.N; i++ {
		var before, after runtime.MemStats
		var problems [batch]*order.BooleanProblem
		runtime.GC()
		runtime.ReadMemStats(&before)
		for j := range problems {
			problem, err := order.ProjectQuantifiedGoal(m, roots, orderProofSteps)
			if err != nil || problem == nil || len(problem.Clauses) == 0 {
				b.Fatalf("empty quantified problem: %v", err)
			}
			problems[j] = problem
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		runtime.KeepAlive(problems)
		total += int64(after.HeapAlloc) - int64(before.HeapAlloc)
	}
	b.ReportMetric(float64(total)/float64(b.N)/batch, "live-B/problem")
}

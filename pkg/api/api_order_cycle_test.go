package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestVerificationGoalBitVectorOrderCycles(t *testing.T) {
	const decls = "(declare-const x (_ BitVec 64))(declare-const y (_ BitVec 64))(declare-const z (_ BitVec 64))"
	for _, tc := range []struct {
		name, formula string
		proved        bool
	}{
		{"transitivity", "(not (=> (and (bvult x y) (bvule y z)) (bvult x z)))", true},
		{"trichotomy", "(not (or (bvult x y) (= x y) (bvult y x)))", true},
		{"asymmetry", "(and (bvugt x y) (bvugt y x))", true},
		{"weak cycle", "(and (bvuge x y) (bvule x y))", false},
		{"strict conclusion", "(not (=> (and (bvule x y) (bvule y z)) (bvult x z)))", false},
		{"reverse implication", "(=> (bvult x y) (bvult y x))", false},
		{"positive disjunction", "(or (bvult x y) (bvult y x))", false},
		{"mixed signedness", "(and (bvult x y) (bvslt y x))", false},
		{"opaque compound operands", "(and (bvult (bvadd x z) y) (bvule y (bvadd x z)))", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, donor := orderSolver(t, decls+"(assert "+tc.formula+")")
			rows, err := solver.Assertions(donor.solver)
			astTestMust(t, err)
			source, err := ctx.NewSolverForIncrementalChecks()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, source.Close()) })
			m, err := ctx.Manager()
			astTestMust(t, err)
			base, err := m.MkTrue()
			astTestMust(t, err)
			astTestMust(t, source.Assert(base))
			b := source.BeginOrderProofBatch()
			t.Cleanup(func() { astTestMust(t, b.Close()) })
			result, err := donor.Check()
			astTestMust(t, err)
			if result == lbool.Undef || (result == lbool.False) != tc.proved {
				t.Fatalf("concrete result %v", result)
			}
			got, err := b.ProveWithGoal(rows[0])
			astTestMust(t, err)
			if got != tc.proved {
				t.Fatalf("private proof %t, want %t", got, tc.proved)
			}
		})
	}
}

func TestOrderCycleWidthCutoff(t *testing.T) {
	for _, width := range []int{8, 16, 31, 32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			input := fmt.Sprintf("(define-sort W () (_ BitVec %d))", width) +
				"(declare-const x W)(declare-const y W)(assert (and (bvult x y) (bvule y x)))"
			ctx, donor := orderSolver(t, input)
			m, err := ctx.Manager()
			astTestMust(t, err)
			rows, err := solver.Assertions(donor.solver)
			astTestMust(t, err)
			if got := order.RefutesComparisonCycle(m, rows[0], 32); got != (width >= 32) {
				t.Fatalf("width %d refuted %t", width, got)
			}
			result, err := donor.Check()
			astTestMust(t, err)
			if result != lbool.False {
				t.Fatalf("concrete result %v", result)
			}
		})
	}
}

// Two-bit vectors realize every relative ordering of three terms. Enumerate
// those concrete models independently of the graph algorithm.
func TestOrderCyclesAgainstTwoBitModels(t *testing.T) {
	names := []string{"x", "y", "z"}
	operators := []string{"bvult", "bvule", "bvugt", "bvuge", "=", "distinct"}
	var script strings.Builder
	for _, name := range names {
		fmt.Fprintf(&script, "(declare-const %s (_ BitVec 2))", name)
	}
	for i, name := range names {
		for _, op := range operators {
			if op == "distinct" {
				fmt.Fprintf(&script, "(assert (not (= %s %s)))", name, names[(i+1)%3])
			} else {
				fmt.Fprintf(&script, "(assert (%s %s %s))", op, name, names[(i+1)%3])
			}
		}
	}
	ctx, donor := orderSolver(t, script.String())
	m, err := ctx.Manager()
	astTestMust(t, err)
	terms, err := solver.Assertions(donor.solver)
	astTestMust(t, err)
	if len(terms) != 3*len(operators) {
		t.Fatalf("got %d comparison terms", len(terms))
	}
	var models [3][6]uint64
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			for z := 0; z < 4; z++ {
				values := [3]int{x, y, z}
				bit := uint64(1) << uint(16*x+4*y+z)
				for i, a := range values {
					b := values[(i+1)%3]
					for op, ok := range [...]bool{a < b, a <= b, a > b, a >= b, a == b, a != b} {
						if ok {
							models[i][op] |= bit
						}
					}
				}
			}
		}
	}
	for i := range operators {
		for j := range operators {
			pair, err := m.MkAnd(terms[i], terms[6+j])
			astTestMust(t, err)
			astTestMust(t, m.IncRef(pair))
			for k := range operators {
				if i >= 4 && j >= 4 && k >= 4 {
					continue
				}
				goal, err := m.MkAnd(pair, terms[12+k])
				astTestMust(t, err)
				astTestMust(t, m.IncRef(goal))
				got := order.RefutesComparisonCycle(m, goal, 0)
				astTestMust(t, m.DecRef(goal))
				want := models[0][i]&models[1][j]&models[2][k] == 0
				if got != want {
					t.Fatalf("%s(x,y), %s(y,z), %s(z,x): refuted %t, want %t", operators[i], operators[j], operators[k], got, want)
				}
			}
			astTestMust(t, m.DecRef(pair))
		}
	}
}

// BenchmarkVerificationOrderCycle measures the word-level refutation without
// reusing a previous answer for the same goal.
func BenchmarkVerificationOrderCycle(b *testing.B) {
	for _, width := range []int{4, 64, 256} {
		b.Run(fmt.Sprintf("%d-bit", width), func(b *testing.B) {
			input := fmt.Sprintf("(define-sort B () (_ BitVec %d))", width) +
				"(declare-const x B)(declare-const y B)(declare-const z B)" +
				"(assert (and (bvult x y) (bvule y z) (bvule z x)))"
			ctx, s := orderSolver(b, input)
			rows, err := solver.Assertions(s.solver)
			astTestMust(b, err)
			m, err := ctx.Manager()
			astTestMust(b, err)
			goal := rows[0]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if !order.RefutesComparisonCycle(m, goal, 0) {
					b.Fatal("missed strict cycle")
				}
			}
		})
	}
}

package tactic

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestIndependentPolynomialsRetainOriginalConstraints(t *testing.T) {
	if ast.ASTSourceDebug || diagnostic.TraceCompiled() {
		t.Skip("polynomial preparation runs with tracing and source assertions disabled")
	}
	for _, models := range []bool{false, true} {
		t.Run(fmt.Sprintf("models=%v", models), func(t *testing.T) {
			f := newGoalCompilerFixture(t)
			m := f.manager
			g, err := basetactic.NewGoal(m, false, models, true)
			compilerMust(t, err)
			defer func() { compilerMust(t, g.Close()) }()
			compilerMust(t, g.SetDepth(5))
			two, err := m.ArithmeticRealNumeralText("2")
			compilerMust(t, err)
			three, err := m.ArithmeticRealNumeralText("3")
			compilerMust(t, err)
			zero, err := m.ArithmeticRealNumeralText("0")
			compilerMust(t, err)
			sort, err := m.GetSort(two)
			compilerMust(t, err)

			var original, kept []ast.ASTHandle
			var deps []ast.ASTDependency
			add := func(form ast.ASTHandle, keep bool) {
				t.Helper()
				dep, err := m.DependencyLeaf(form)
				compilerMust(t, err)
				compilerMust(t, g.AssertWithProof(form, ast.ASTHandle{}, dep))
				original = append(original, form)
				if keep {
					kept = append(kept, form)
					deps = append(deps, dep)
				}
			}
			xs := make([]ast.ASTHandle, 24)
			for i := range xs {
				x := compilerConst(t, m, fmt.Sprintf("component%d", i), sort)
				xs[i] = x
				sq, err := m.MkArithmeticMul(x, x)
				compilerMust(t, err)
				rhs := two
				if i == 3 {
					rhs = three
				}
				eq, err := m.MkEq(sq, rhs)
				compilerMust(t, err)
				a, b := x, zero
				if i == 4 {
					a, b = b, a
				}
				sign, err := m.MkArithmeticGT(a, b)
				compilerMust(t, err)
				// The first three variables will be coupled below, and the next
				// two have different constraints. Variable five represents the rest.
				if i%2 == 0 {
					add(sign, i <= 5)
					add(eq, i <= 5)
				} else {
					add(eq, i <= 5)
					add(sign, i <= 5)
				}
				if i == 6 {
					add(sign, false)
				}
			}
			sum, err := m.MkArithmeticAdd(xs[0], xs[1])
			compilerMust(t, err)
			lower, err := m.MkArithmeticGT(sum, zero)
			compilerMust(t, err)
			add(lower, true)
			// sum is already cached as multivariate. Its parent also disqualifies xs[2].
			link, err := m.MkArithmeticGT(sum, xs[2])
			compilerMust(t, err)
			add(link, true)

			tac, err := NewPolynomialPreparationTactic(m, f.context, f.globals, nil, f.services)
			compilerMust(t, err)
			defer func() { compilerMust(t, tac.Close()) }()
			var result basetactic.TacticResult
			defer func() { compilerMust(t, result.Close()) }()
			compilerMust(t, tac.Apply(g, &result))
			if len(result.Goals) != 1 || result.Goals[0] == g || (result.ModelConverter != nil) != models {
				t.Fatal("expected a reduced subgoal and the requested model ownership")
			}
			check := func(goal *basetactic.Goal, want []ast.ASTHandle, depth uint32) {
				t.Helper()
				actual, err := goal.AppendFormulas(nil)
				compilerMust(t, err)
				level, err := goal.Depth()
				compilerMust(t, err)
				if len(actual) != len(want) || depth != level {
					t.Fatalf("goal has %d assertions at depth %d, want %d at depth %d", len(actual), level, len(want), depth)
				}
				for i := range want {
					if actual[i] != want[i] {
						t.Fatalf("assertion %d changed", i)
					}
				}
			}
			check(g, original, 5)
			check(result.Goals[0], kept, 6)
			for i, want := range deps {
				got, err := result.Goals[0].Dependency(uint32(i))
				compilerMust(t, err)
				if got != want {
					t.Fatalf("dependency %d changed", i)
				}
			}
			var integerOnly basetactic.TacticResult
			defer func() { compilerMust(t, integerOnly.Close()) }()
			compilerMust(t, candidateTactic(t, f).Apply(g, &integerOnly))
			if len(integerOnly.Goals) != 1 || integerOnly.Goals[0] != g || integerOnly.ModelConverter != nil {
				t.Fatal("integer witness search should pass on the irrational goal")
			}
			// Adding an uninterpreted Boolean prevents analysis of this entire goal.
			bs, err := m.GetSort(link)
			compilerMust(t, err)
			add(compilerConst(t, m, "flag", bs), true)
			var fallback basetactic.TacticResult
			defer func() { compilerMust(t, fallback.Close()) }()
			compilerMust(t, tac.Apply(g, &fallback))
			if len(fallback.Goals) != 1 || fallback.Goals[0] != g || fallback.ModelConverter != nil {
				t.Fatal("unsupported goal did not pass through intact")
			}
			check(g, original, 5)
		})
	}
}

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

func TestIndependentPolynomialsKeepSharedParametersSymbolic(t *testing.T) {
	if ast.ASTSourceDebug || diagnostic.TraceCompiled() {
		t.Skip("polynomial preparation runs with tracing and source assertions disabled")
	}
	f := newGoalCompilerFixture(t)
	m := f.manager
	g, err := basetactic.NewGoal(m, false, false, false)
	compilerMust(t, err)
	defer func() { compilerMust(t, g.Close()) }()
	two, err := m.ArithmeticRealNumeralText("2")
	compilerMust(t, err)
	three, err := m.ArithmeticRealNumeralText("3")
	compilerMust(t, err)
	sort, err := m.GetSort(two)
	compilerMust(t, err)
	a := compilerConst(t, m, "parameter-a", sort)
	b := compilerConst(t, m, "parameter-b", sort)
	expected := make([]ast.ASTHandle, 0, 9)
	add := func(h ast.ASTHandle, keep bool) {
		t.Helper()
		compilerMust(t, g.Assert(h))
		if keep {
			expected = append(expected, h)
		}
	}
	squareEquals := func(x, rhs ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		sq, err := m.MkArithmeticMul(x, x)
		compilerMust(t, err)
		eq, err := m.MkEq(sq, rhs)
		compilerMust(t, err)
		return eq
	}
	add(squareEquals(a, two), true)
	add(squareEquals(b, three), true)
	var xs []ast.ASTHandle
	for i := 0; i < 45; i++ {
		x := compilerConst(t, m, fmt.Sprintf("local-%d", i), sort)
		xs = append(xs, x)
		// x0 and x1 are also linked below. The first independent owners
		// are x2 for the fixed coefficient, x3 for a, and x4 for b.
		rhs := []ast.ASTHandle{a, b, two}[i%3]
		add(squareEquals(x, rhs), i < 5)
	}
	link, err := m.MkArithmeticLT(xs[0], xs[1])
	compilerMust(t, err)
	add(link, true)
	tac, err := NewPolynomialPreparationTactic(m, f.context, f.globals, nil, f.services)
	compilerMust(t, err)
	defer func() { compilerMust(t, tac.Close()) }()
	var result basetactic.TacticResult
	defer func() { compilerMust(t, result.Close()) }()
	compilerMust(t, tac.Apply(g, &result))
	if len(result.Goals) != 1 || result.Goals[0] == g {
		t.Fatal("expected reduction around two symbolic parameters")
	}
	got, err := result.Goals[0].AppendFormulas(nil)
	compilerMust(t, err)
	if len(got) != len(expected) {
		t.Fatalf("retained %d assertions, want %d", len(got), len(expected))
	}
	for i, want := range expected {
		if got[i] != want {
			t.Fatalf("retained assertion %d has a different variable or parameter", i)
		}
	}
}

func TestIndependentPolynomialsRecognizeParametersInsideSharedTerms(t *testing.T) {
	f := newGoalCompilerFixture(t)
	m := f.manager
	g, err := basetactic.NewGoal(m, false, false, false)
	compilerMust(t, err)
	defer func() { compilerMust(t, g.Close()) }()
	two, err := m.ArithmeticRealNumeralText("2")
	compilerMust(t, err)
	sort, err := m.GetSort(two)
	compilerMust(t, err)
	a, b := compilerConst(t, m, "a", sort), compilerConst(t, m, "b", sort)
	sum, err := m.MkArithmeticAdd(a, b)
	compilerMust(t, err)
	diff, err := m.MkArithmeticSub(b, a)
	compilerMust(t, err)
	for i := range 32 {
		x := compilerConst(t, m, fmt.Sprintf("shared-term-%d", i), sort)
		sq, err := m.MkArithmeticMul(x, x)
		compilerMust(t, err)
		rhs := sum
		if i%2 != 0 {
			rhs = diff
		}
		eq, err := m.MkEq(sq, rhs)
		compilerMust(t, err)
		compilerMust(t, g.Assert(eq))
	}
	forms, err := g.AppendFormulas(nil)
	compilerMust(t, err)
	family, err := m.ArithmeticFamilyID()
	compilerMust(t, err)
	first, err := scanPolynomialComponents(m, family, forms, nil)
	compilerMust(t, err)
	parameters, err := first.sharedParameters()
	compilerMust(t, err)
	if len(parameters) != 2 || !parameters[a] || !parameters[b] {
		t.Fatalf("expected only the two parameters in the shared sum and difference, got %v", parameters)
	}
	final, err := scanPolynomialComponents(m, family, forms, parameters)
	compilerMust(t, err)
	if aliases := final.group(); aliases != 30 {
		t.Fatalf("expected one representative per expression, got %d aliases", aliases)
	}
}

func TestIndependentPolynomialsFoldReflexiveComparisons(t *testing.T) {
	f := newGoalCompilerFixture(t)
	m := f.manager
	g, err := basetactic.NewGoal(m, false, true, true)
	compilerMust(t, err)
	defer func() { compilerMust(t, g.Close()) }()
	arith, err := m.ArithmeticFamilyID()
	compilerMust(t, err)
	two, err := m.ArithmeticRealNumeralText("2")
	compilerMust(t, err)
	sort, err := m.GetSort(two)
	compilerMust(t, err)
	var squares, variables, forms []ast.ASTHandle
	add := func(h ast.ASTHandle) {
		t.Helper()
		dep, err := m.DependencyLeaf(h)
		compilerMust(t, err)
		compilerMust(t, g.AssertWithProof(h, ast.ASTHandle{}, dep))
		forms = append(forms, h)
	}
	for i := range 18 {
		x := compilerConst(t, m, fmt.Sprintf("compared-%d", i), sort)
		sq, err := m.MkArithmeticMul(x, x)
		compilerMust(t, err)
		eq, err := m.MkEq(sq, two)
		compilerMust(t, err)
		add(eq)
		squares, variables = append(squares, sq), append(variables, x)
	}
	for i := 1; i < len(squares); i++ {
		order, err := m.MkArithmeticLE(squares[i], squares[i-1])
		compilerMust(t, err)
		add(order)
	}
	prepare := func() *polynomialComponents {
		t.Helper()
		p, err := scanPolynomialComponents(m, arith, forms, nil)
		compilerMust(t, err)
		if p == nil || p.group() != 0 {
			t.Fatal("expected linked variables before testing reflexivity")
		}
		next, err := p.groupReflexiveComparisons(forms)
		compilerMust(t, err)
		return next
	}
	p := prepare()
	if p == nil || len(p.omitted) != len(squares)-1 {
		t.Fatal("expected every non-strict comparison to hold for the reconstructed model")
	}
	for i, x := range variables {
		if got := p.comparisonRepresentative(x); got != p.terms[variables[0]].variable {
			t.Fatalf("variable %d copied a different representative", i)
		}
	}
	// A strict order between otherwise identical variables can require
	// different signs. It must survive even if all square comparisons agree.
	strict, err := m.MkArithmeticLT(variables[0], variables[1])
	compilerMust(t, err)
	add(strict)
	if prepare() != nil {
		t.Fatal("strict inequality lost its independent operands")
	}
}

package transforms

// Source: context::get_sort_size_estimate returns uint64 maximum for an
// infinite sort; join_planner converts it to float cost before arithmetic.

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type nonfiniteJoinCostFixture struct {
	t       *testing.T
	manager *ast.ASTManager
	rules   *base.RuleManager
	plugin  *SimpleJoinsPlugin
	boolean ast.ASTHandle
	roots   []ast.ASTRef
}

func newNonfiniteJoinCostFixture(t *testing.T) *nonfiniteJoinCostFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 28)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	simpleJoinNonfiniteMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	simpleJoinNonfiniteMust(t, err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	simpleJoinNonfiniteMust(t, err)
	rules, err := base.NewRuleManager(manager)
	simpleJoinNonfiniteMust(t, err)
	f := &nonfiniteJoinCostFixture{t: t, manager: manager, rules: rules, plugin: NewSimpleJoinsPlugin(manager, rules, 0)}
	t.Cleanup(func() {
		simpleJoinNonfiniteMust(t, f.plugin.Close())
		for i := len(f.roots) - 1; i >= 0; i-- {
			simpleJoinNonfiniteMust(t, manager.DelRef(&f.roots[i]))
		}
		simpleJoinNonfiniteMust(t, rules.Close())
		simpleJoinNonfiniteMust(t, manager.Close())
		simpleJoinNonfiniteMust(t, numerals.Close())
		simpleJoinNonfiniteMust(t, symbols.Close())
	})
	f.boolean = f.hold(manager.MkBoolSort())
	return f
}

func simpleJoinNonfiniteMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *nonfiniteJoinCostFixture) hold(value ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	simpleJoinNonfiniteMust(f.t, err)
	root, err := f.manager.NewRef(value)
	simpleJoinNonfiniteMust(f.t, err)
	f.roots = append(f.roots, root)
	return value
}

func (f *nonfiniteJoinCostFixture) application(name string, arguments ...ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
	f.t.Helper()
	domain := make([]ast.ASTHandle, len(arguments))
	for i, argument := range arguments {
		var err error
		domain[i], err = f.manager.GetSort(argument)
		simpleJoinNonfiniteMust(f.t, err)
	}
	symbol, err := f.manager.Symbols.Intern(name)
	simpleJoinNonfiniteMust(f.t, err)
	declaration := f.hold(f.manager.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{}))
	simpleJoinNonfiniteMust(f.t, f.rules.RegisterPredicate(declaration))
	return declaration, f.hold(f.manager.MkApp(declaration, arguments))
}

func TestSimpleJoinsNonfiniteDomainAndFloatingCosts(t *testing.T) {
	for _, sortKind := range []int32{1, 0} {
		name := "Int"
		if sortKind == 0 {
			name = "Real"
		}
		t.Run(name, func(t *testing.T) {
			f := newNonfiniteJoinCostFixture(t)
			family, err := f.manager.RegisterArithmeticPlugin()
			simpleJoinNonfiniteMust(t, err)
			sort := f.hold(f.manager.MkFamilySort(family, sortKind, nil))
			if got := f.plugin.domainCost(sort); got != float32(math.Ldexp(1, 64)) {
				t.Fatalf("nonfinite domain cost %g, want source uint64 maximum rounded to 2^64", got)
			}
			if got := f.plugin.domainCost(f.boolean); got != 2 {
				t.Fatal("finite Boolean domain cost changed", got)
			}
			x, y := f.hold(f.manager.MkVar(0, sort)), f.hold(f.manager.MkVar(1, sort))
			b := f.hold(f.manager.MkVar(2, f.boolean))
			_, left := f.application("nonfinite-cost-left", x)
			_, right := f.application("nonfinite-cost-right", y)
			_, finite := f.application("nonfinite-cost-finite", b)
			_, head := f.application("nonfinite-cost-head", x, y, b)
			cost, _, err := f.plugin.tailCost(left)
			simpleJoinNonfiniteMust(t, err)
			if cost != float32(math.Ldexp(1, 64)) {
				t.Fatal("unary arithmetic tail cost", cost)
			}
			tails := []ast.ASTHandle{left, right, finite}
			cost, err = f.plugin.estimatedPairCost(tails, []ast.ASTHandle{head}, 0, 2)
			simpleJoinNonfiniteMust(t, err)
			if cost != float32(math.Ldexp(1, 65)) {
				t.Fatal("arithmetic/Boolean pair must remain finite at 2^65", cost)
			}
			cost, err = f.plugin.estimatedPairCost(tails, []ast.ASTHandle{head}, 0, 1)
			simpleJoinNonfiniteMust(t, err)
			if !math.IsInf(float64(cost), 1) {
				t.Fatal("two nonfinite domains must preserve source float overflow", cost)
			}
			cost, err = f.plugin.estimatedPairCost([]ast.ASTHandle{left, right}, nil, 0, 1)
			simpleJoinNonfiniteMust(t, err)
			if !math.IsNaN(float64(cost)) {
				t.Fatal("projecting both local domains must preserve source infinity/infinity", cost)
			}
		})
	}
}

func TestSimpleJoinsNonfiniteCostsChooseFiniteArithmeticBooleanPair(t *testing.T) {
	f := newNonfiniteJoinCostFixture(t)
	family, err := f.manager.RegisterArithmeticPlugin()
	simpleJoinNonfiniteMust(t, err)
	integer := f.hold(f.manager.MkFamilySort(family, 1, nil))
	x, y := f.hold(f.manager.MkVar(0, integer)), f.hold(f.manager.MkVar(1, integer))
	b := f.hold(f.manager.MkVar(2, f.boolean))
	leftDecl, left := f.application("nonfinite-planner-left", x)
	rightDecl, right := f.application("nonfinite-planner-right", y)
	finiteDecl, finite := f.application("nonfinite-planner-finite", b)
	_, head := f.application("nonfinite-planner-head", x, y, b)
	set, err := base.NewRuleSet(f.rules)
	simpleJoinNonfiniteMust(t, err)
	t.Cleanup(func() { simpleJoinNonfiniteMust(t, set.Close()) })
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{left, right, finite}, nil, "nonfinite-planner")
	simpleJoinNonfiniteMust(t, err)
	simpleJoinNonfiniteMust(t, set.Add(rule))
	simpleJoinNonfiniteMust(t, rule.Close())
	transformed, err := f.plugin.Transform(set)
	simpleJoinNonfiniteMust(t, err)
	if transformed == nil {
		t.Fatal("three positive tails were not split")
	}
	t.Cleanup(func() { simpleJoinNonfiniteMust(t, transformed.Close()) })
	fresh := f.plugin.FreshPredicates()
	if len(fresh) != 1 {
		t.Fatal("expected one binary join helper", len(fresh))
	}
	definitions, err := transformed.PredicateRules(fresh[0])
	simpleJoinNonfiniteMust(t, err)
	defer func() {
		for _, definition := range definitions {
			simpleJoinNonfiniteMust(t, definition.Close())
		}
	}()
	if len(definitions) != 1 {
		t.Fatal("expected one helper definition", len(definitions))
	}
	first, err := definitions[0].TailDeclaration(0)
	simpleJoinNonfiniteMust(t, err)
	second, err := definitions[0].TailDeclaration(1)
	simpleJoinNonfiniteMust(t, err)
	// The first source pair (left,right) overflows float. The next pair
	// (left,finite) costs 2^65, so it wins before stratification discount.
	if first == rightDecl || second == rightDecl || first != leftDecl && second != leftDecl || first != finiteDecl && second != finiteDecl {
		t.Fatal("planner failed to choose the finite-cost arithmetic/Boolean pair", first, second)
	}
}

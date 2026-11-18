package tactic

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/nlsat"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type goalCompilerFixture struct {
	manager  *ast.ASTManager
	solver   *nlsat.Solver
	compiler *GoalCompiler
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	services model.ModelEvaluatorConfig
}

func compilerMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newGoalCompilerFixture(t *testing.T) *goalCompilerFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 30)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	compilerMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{
		Integers: unumeral.IntegerConfig{
			Cells: arena.Config{Budget: budget},
			Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
		},
	})
	compilerMust(t, err)
	t.Cleanup(func() {
		compilerMust(t, numerals.Close())
		compilerMust(t, symbols.Close())
		if used := budget.Used(); used != 0 {
			t.Errorf("owner graph retained %d bytes", used)
		}
	})
	context, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, context.Close()) })
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, catalog.Close()) })
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, globals.Close()) })
	compilerMust(t, globals.InitializeSource())
	previous, err := uparams.BindProcessGlobalParamsService(globals)
	compilerMust(t, err)
	t.Cleanup(func() {
		_, restoreErr := uparams.BindProcessGlobalParamsService(previous)
		compilerMust(t, restoreErr)
	})
	cooperation, err := cooperate.NewCooperation(budget)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, cooperation.Close()) })
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, manager.Close()) })
	_, err = manager.RegisterArithmeticPlugin()
	compilerMust(t, err)
	algebraic, err := manager.ArithmeticAlgebraicManager()
	compilerMust(t, err)
	rationals, err := algebraic.RationalManager()
	compilerMust(t, err)
	limit, err := control.NewReslimit(budget)
	compilerMust(t, err)
	t.Cleanup(limit.Close)
	polynomials, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core: polynomial.MultivariatePolynomialConfig{
			Integer: rationals.Integers(), Rationals: rationals,
			Checkpoint: limit.Inc, Cooperation: func() error { return nil },
		},
	})
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, polynomials.CloseChecked()) })
	solver, err := nlsat.NewSolver(nlsat.SolverConfig{
		Algebraic: algebraic, Polynomial: polynomials, Limit: limit, SimplifyCore: true,
		MaxConflicts: ^uint32(0),
	})
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, solver.Close()) })
	compiler, err := NewGoalCompiler(manager, solver)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, compiler.Close()) })
	return &goalCompilerFixture{
		manager: manager, solver: solver, compiler: compiler, context: context, globals: globals,
		services: model.ModelEvaluatorConfig{Context: context, Globals: globals, Memory: numerals.RuntimeOwner(), Cooperation: cooperation},
	}
}

func compilerConst(t *testing.T, manager *ast.ASTManager, name string, sort ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	symbol, err := manager.Symbols.Intern(name)
	compilerMust(t, err)
	result, err := manager.MkConst(symbol, sort)
	compilerMust(t, err)
	return result
}

func compilerGoal(t *testing.T, manager *ast.ASTManager, formula ast.ASTHandle) *basetactic.Goal {
	t.Helper()
	goal, err := basetactic.NewGoal(manager, false, true, false)
	compilerMust(t, err)
	compilerMust(t, goal.Assert(formula))
	t.Cleanup(func() { compilerMust(t, goal.Close()) })
	return goal
}

func TestGoalCompilerRealQuadraticSat(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	two, err := fixture.manager.ArithmeticRealNumeralText("2")
	compilerMust(t, err)
	sort, err := fixture.manager.GetSort(two)
	compilerMust(t, err)
	x := compilerConst(t, fixture.manager, "x", sort)
	square, err := fixture.manager.MkArithmeticMul(x, x)
	compilerMust(t, err)
	equality, err := fixture.manager.MkEq(square, two)
	compilerMust(t, err)
	compilerMust(t, fixture.compiler.Compile(compilerGoal(t, fixture.manager, equality)))
	result, err := fixture.solver.Check()
	compilerMust(t, err)
	if result != lbool.True {
		t.Fatalf("x*x = 2: got %v", result)
	}
	entries, err := fixture.compiler.TermMapping().Entries()
	compilerMust(t, err)
	if len(entries) != 1 || entries[0].Expr != x {
		t.Fatalf("term mapping = %#v", entries)
	}
}

func TestGoalCompilerKeepsOutOfRangePowerAsOneAtomicTerm(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	zero, err := fixture.manager.ArithmeticRealNumeralText("0")
	compilerMust(t, err)
	integerExponent, err := fixture.manager.ArithmeticIntegerNumeralText("4294967296")
	compilerMust(t, err)
	sort, err := fixture.manager.GetSort(zero)
	compilerMust(t, err)
	x := compilerConst(t, fixture.manager, "large-power-x", sort)
	power, err := fixture.manager.MkArithmeticPower(x, integerExponent)
	compilerMust(t, err)
	formula, err := fixture.manager.MkEq(power, zero)
	compilerMust(t, err)
	compilerMust(t, fixture.compiler.Compile(compilerGoal(t, fixture.manager, formula)))
	entries, err := fixture.compiler.TermMapping().Entries()
	compilerMust(t, err)
	if len(entries) != 1 || entries[0].Expr != power {
		t.Fatalf("NLSAT atom mapping for out-of-range power = %#v", entries)
	}
}

func TestGoalCompilerIntegerLinearUnsat(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	one, err := fixture.manager.ArithmeticIntegerNumeralText("1")
	compilerMust(t, err)
	two, err := fixture.manager.ArithmeticIntegerNumeralText("2")
	compilerMust(t, err)
	sort, err := fixture.manager.GetSort(one)
	compilerMust(t, err)
	x := compilerConst(t, fixture.manager, "x", sort)
	twice, err := fixture.manager.MkArithmeticMul(two, x)
	compilerMust(t, err)
	equality, err := fixture.manager.MkEq(twice, one)
	compilerMust(t, err)
	compilerMust(t, fixture.compiler.Compile(compilerGoal(t, fixture.manager, equality)))
	result, err := fixture.solver.Check()
	compilerMust(t, err)
	if result != lbool.False {
		t.Fatalf("2*x = 1 over Int: got %v", result)
	}
}

func TestGoalCompilerRationalDenominatorClearing(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	half, err := fixture.manager.ArithmeticRealNumeralText("1/2")
	compilerMust(t, err)
	one, err := fixture.manager.ArithmeticRealNumeralText("1")
	compilerMust(t, err)
	sort, err := fixture.manager.GetSort(one)
	compilerMust(t, err)
	x := compilerConst(t, fixture.manager, "x", sort)
	sum, err := fixture.manager.MkArithmeticAdd(x, half)
	compilerMust(t, err)
	equality, err := fixture.manager.MkEq(sum, one)
	compilerMust(t, err)
	compilerMust(t, fixture.compiler.Compile(compilerGoal(t, fixture.manager, equality)))
	result, err := fixture.solver.Check()
	compilerMust(t, err)
	if result != lbool.True {
		t.Fatalf("x + 1/2 = 1: got %v", result)
	}
	modelValue, supported, err := fixture.compiler.Model()
	compilerMust(t, err)
	if !supported || modelValue == nil {
		t.Fatal("SAT model was not published")
	}
	defer func() { compilerMust(t, modelValue.Close()) }()
	declaration, err := fixture.manager.GetDecl(x)
	compilerMust(t, err)
	interpretation, err := modelValue.Core().ConstantInterpretation(declaration)
	compilerMust(t, err)
	text, integer, matched, err := fixture.manager.ArithmeticNumeralText(interpretation)
	compilerMust(t, err)
	if !matched || integer || text != "1/2" {
		t.Fatalf("x model = %q, integer=%v, matched=%v", text, integer, matched)
	}
}

func TestGoalCompilerBooleanAbstractionModel(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	booleanSort, err := fixture.manager.MkBoolSort()
	compilerMust(t, err)
	p := compilerConst(t, fixture.manager, "p", booleanSort)
	compilerMust(t, fixture.compiler.Compile(compilerGoal(t, fixture.manager, p)))
	result, err := fixture.solver.Check()
	compilerMust(t, err)
	if result != lbool.True {
		t.Fatalf("p: got %v", result)
	}
	modelValue, supported, err := fixture.compiler.Model()
	compilerMust(t, err)
	if !supported || modelValue == nil {
		t.Fatal("Boolean SAT model was not published")
	}
	defer func() { compilerMust(t, modelValue.Close()) }()
	declaration, err := fixture.manager.GetDecl(p)
	compilerMust(t, err)
	interpretation, err := modelValue.Core().ConstantInterpretation(declaration)
	compilerMust(t, err)
	if kind, kindErr := fixture.manager.GetDeclKind(interpretation); kindErr != nil || kind != ast.ASTOpTrue {
		t.Fatalf("p model kind = %d, err=%v", kind, kindErr)
	}
}

func TestGoalCompilerUnsatCoreDependencies(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	zero, err := fixture.manager.ArithmeticRealNumeralText("0")
	compilerMust(t, err)
	one, err := fixture.manager.ArithmeticRealNumeralText("1")
	compilerMust(t, err)
	sort, err := fixture.manager.GetSort(zero)
	compilerMust(t, err)
	x := compilerConst(t, fixture.manager, "x", sort)
	upper, err := fixture.manager.MkArithmeticLE(x, zero)
	compilerMust(t, err)
	lower, err := fixture.manager.MkArithmeticGE(x, one)
	compilerMust(t, err)
	booleanSort, err := fixture.manager.MkBoolSort()
	compilerMust(t, err)
	labels := []ast.ASTHandle{
		compilerConst(t, fixture.manager, "upper", booleanSort),
		compilerConst(t, fixture.manager, "lower", booleanSort),
	}
	goal, err := basetactic.NewGoal(fixture.manager, false, false, true)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, goal.Close()) })
	for i, formula := range []ast.ASTHandle{upper, lower} {
		dependency, cause := fixture.manager.DependencyLeaf(labels[i])
		compilerMust(t, cause)
		compilerMust(t, goal.AssertWithProof(formula, ast.ASTHandle{}, dependency))
	}
	compilerMust(t, fixture.compiler.Compile(goal))
	result, err := fixture.solver.Check()
	compilerMust(t, err)
	if result != lbool.False {
		t.Fatalf("bounded contradiction: got %v", result)
	}
	var core basetactic.DependencyRef
	compilerMust(t, fixture.compiler.Core(&core))
	defer func() { compilerMust(t, core.Close()) }()
	leaves, err := fixture.manager.DependencyLinearize(core.Get(), nil)
	compilerMust(t, err)
	if len(leaves) != 2 {
		t.Fatalf("core leaves = %#v", leaves)
	}
	seen := make(map[ast.ASTHandle]bool, len(leaves))
	for _, leaf := range leaves {
		seen[leaf] = true
	}
	for _, label := range labels {
		if !seen[label] {
			t.Fatalf("core omitted label %#v", label)
		}
	}
}

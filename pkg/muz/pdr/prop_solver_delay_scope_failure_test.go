package pdr

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// A non-Boolean temporary formula reaches the solver after the delayed push
// and is rejected by the native assertion. The public check must still pop.
func TestPropSolverCheckFailureRollsBackDelayedPush(t *testing.T) {
	manager, context, _ := classifierContextFixture(t)
	truth, err := manager.MkTrue()
	closureMust(t, err)
	solver, err := NewPropSolver(manager, context.owner.contexts, truth, "delay-assert-error")
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, solver.Close()) })
	integer, err := manager.ArithmeticIntegerNumeralText("1")
	closureMust(t, err)
	for attempt := 0; attempt < 2; attempt++ {
		_, err = solver.CheckAssumptionsAndFormula(nil, integer, false, false)
		if !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("expected assertion domain failure", err)
		}
		if solver.context.delayed || solver.context.pushed || solver.delayScope != nil {
			t.Fatal("failed check retained its delayed scope")
		}
		result, err := solver.CheckAssumptions(nil, false, false)
		closureMust(t, err)
		if result.Status != lbool.True {
			t.Fatal("failed check corrupted the next check", result.Status)
		}
	}
}

// A deferred scope pop can fail after its native cleanup has begun. The
// propositional solver must retain ownership until Close can resume it.
func TestPropSolverDelayedScopeRecoversPartialPop(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 29)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	contexts, err := NewSMTContextManager(manager, nil, 0, smtcore.SMTSolverConfig{Params: params, Globals: globals})
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	solver, err := NewPropSolver(manager, contexts, truth, "partial-delay")
	must(err)

	scoped, err := numeral.NewScopedInt(numbers.Manager.Z)
	must(err)
	integer, err := scoped.Get()
	must(err)
	words := make([]uint32, 262144)
	words[0], words[len(words)-1] = 17, 1
	must(numbers.Manager.Z.SetLimbs(integer, words))
	value, err := numbers.NewFromScopedInt(scoped)
	must(err)
	parameter, err := manager.NewParameterRational(&value)
	must(err)
	info, err := manager.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	label, err := symbols.Intern("prop-scope-pop-assertion")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	scope, err := solver.context.BeginDelay()
	must(err)
	must(solver.context.Assert(formula))
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())

	a, err := runtime.Allocate(8)
	must(err)
	b, err := runtime.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first boundary", err)
	}
	must(runtime.Deallocate(&a))
	delta = math.MinInt64 + 262144 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("second boundary", err)
	}
	must(runtime.Deallocate(&b))
	blocks := runtime.Stats().LiveBlocks
	failure := solver.closeDelay(scope)
	if !errors.Is(failure, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("scope pop did not fail after native cleanup", failure)
	}
	if err = solver.open(); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("solver accepted work while cleanup was pending", err)
	}
	recovery, err := runtime.Allocate(4 << 20)
	must(err)
	must(solver.Close())
	must(contexts.Close())
	must(manager.Close())
	must(globals.Close())
	must(catalog.Close())
	must(params.Close())
	must(numbers.Close())
	must(symbols.Close())
	if err = runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
		t.Fatal(err)
	}
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("leak", budget.Used())
	}
}

package duality

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// A queryRun owns each copied source background axiom through teardown.
func TestQueryRunCloseAfterConsumedBackgroundRelease(t *testing.T) {
	queryRunConsumedRelease(t)
}

func queryRunConsumedRelease(t *testing.T) {
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
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	config, err := graph.NewConfig(params, nil)
	must(err)
	context, err := graph.NewContext(manager, config)
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
	label, err := symbols.Intern("prepared-background-consumed")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	expression, err := graph.NewExpr(context, formula)
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	previous, err := graph.NewExpr(context, truth)
	must(err)
	run := &queryRun{background: []*graph.Expr{previous, expression}}
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
	failure := run.close()
	if !errors.Is(failure, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("background cleanup did not fail after native consumption", failure)
	}
	if len(run.background) != 1 || run.background[0] != previous || !expression.Closed() || previous.Closed() {
		t.Fatal("cleanup did not retain exactly the unconsumed background prefix")
	}

	recovery, err := runtime.Allocate(4 << 20)
	must(err)
	must(run.close())
	must(run.close())
	must(context.Close())
	must(config.Close())
	must(params.Close())
	must(manager.Close())
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

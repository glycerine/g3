package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// smt_params fp; smt::kernel k(m, fp): a construction failure, including
// one inside the smt_params constructor, releases what was made.
func TestDefaultParamsKernelAllocationFrontiers(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	parameters, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, parameters, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget, Params: parameters, Globals: globals})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	warm, err := NewDefaultParamsKernel(manager)
	must(err)
	must(warm.Close())
	succeeded := false
	for stop := int64(0); stop < 1<<16 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			before := budget.Used()
			budget.FailAfter(stop)
			kernel, err := NewDefaultParamsKernel(manager)
			budget.FailAfter(-1)
			if err == nil {
				succeeded = true
				must(kernel.Close())
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				var failure memory.Failure
				if !errors.As(err, &failure) || failure.Kind != memory.FailureMemout {
					t.Fatal(err)
				}
			}
			if after := budget.Used(); after != before {
				t.Fatal("construction frontier leaked", stop, after-before, err)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded {
		t.Fatal("no successful construction")
	}
}

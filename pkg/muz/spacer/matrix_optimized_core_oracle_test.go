package spacer

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// The source finalize path encounters x, y, z in that order and starts with
// rows [2,2,0] and [1,2,1]. spacer_matrix.cpp divides the first row by 2,
// subtracts it from the second, and leaves row zero unchanged on pivot two.
// Thus the core combinations are [1,1,0] and [0,1,1].
func TestOptimizedFarkasMultirowCoreSourceOracle(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 26)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64})
	must(err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	must(err)
	params, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	must(err)
	arithmetic, err := manager.RegisterArithmeticPlugin()
	must(err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	must(err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	must(err)
	literals := make([]ast.ASTHandle, 3)
	refs := make([]ast.ASTRef, 3)
	for i, name := range []string{"matrix-oracle-x", "matrix-oracle-y", "matrix-oracle-z"} {
		sym, err := manager.Symbols.Intern(name)
		must(err)
		variable, err := manager.MkConst(sym, integer)
		must(err)
		literals[i], err = manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{variable, zero})
		must(err)
		refs[i], err = manager.NewRef(literals[i])
		must(err)
	}
	learner, err := NewUnsatCoreLearner(manager)
	must(err)
	plugin, err := NewUnsatCoreOptimizedFarkasPlugin(learner, globals.Context, globals)
	must(err)
	must(learner.RegisterPlugin(plugin))
	plugin.combinations = [][]farkasTerm{
		{{literals[0], big.NewRat(2, 1)}, {literals[1], big.NewRat(2, 1)}},
		{{literals[0], big.NewRat(1, 1)}, {literals[1], big.NewRat(2, 1)}, {literals[2], big.NewRat(1, 1)}},
	}
	must(plugin.Finalize())
	if len(learner.core) != 2 {
		t.Fatalf("got %d core lemmas, want 2", len(learner.core))
	}
	want := []string{
		// ast_util mk_not strips the negation farkas_util::get returns.
		"(<= (+ matrix-oracle-x matrix-oracle-y) 0)",
		"(<= (+ matrix-oracle-y matrix-oracle-z) 0)",
	}
	for i, ref := range learner.core {
		h, err := manager.RefGet(ref)
		must(err)
		var out bytes.Buffer
		must(ast.DisplayLegacySMT(&out, manager, h, 0, nil))
		if out.String() != want[i] {
			t.Errorf("core %d = %s; want %s", i, out.String(), want[i])
		}
	}
	must(learner.Close())
	for i := range refs {
		must(manager.DelRef(&refs[i]))
	}
	must(manager.Close())
	must(globals.Close())
	must(catalog.Close())
	must(params.Close())
	must(numerals.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("fixture leaked bytes", budget.Used())
	}
}

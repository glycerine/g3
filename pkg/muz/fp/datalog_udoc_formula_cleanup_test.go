package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type udocFormulaCleanupFixture struct {
	runtime                *memory.Runtime
	manager                *ast.ASTManager
	engine                 *PropositionalEngine
	predicate, wide, value ast.ASTHandle
}

func newUDocFormulaCleanupFixture(t *testing.T) *udocFormulaCleanupFixture {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
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
	engine, err := NewPropositionalEngineWithServices(manager, params, globals)
	must(err)
	var valueRef ast.ASTRef
	t.Cleanup(func() {
		for _, step := range []func() error{
			func() error { return runtime.SetMaxCount(0) },
			func() error { return manager.DelRef(&valueRef) },
			engine.Close, manager.Close, globals.Close, catalog.Close, params.Close,
			numbers.Close, symbols.Close, runtime.Close,
		} {
			if cause := step(); cause != nil {
				t.Error(cause)
			}
		}
		if used := budget.Used(); used != 0 {
			t.Fatal("UDoc formula fixture leaked bytes", used)
		}
	})
	must(globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(engine.SelectEngine("datalog"))
	boolean, err := manager.MkBoolSort()
	must(err)
	wide, err := manager.BitVectorSort(160)
	must(err)
	name, err := symbols.Intern("udoc-formula-wide")
	must(err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{wide}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(predicate))
	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "1361129467683753853853498429727072845829"))
	value, err := manager.BitVectorNumeral(&number, 160)
	must(err)
	must(numbers.Del(&number))
	valueRef, err = manager.NewRef(value)
	must(err)
	must(engine.AddFact(predicate, []ast.ASTHandle{value}))
	status, err := engine.QueryPredicate(predicate)
	must(err)
	if status != lbool.True || engine.udocDatalog == nil {
		t.Fatal("wide UDoc fixture was not selected", status)
	}
	warm, err := engine.Answer(predicate)
	must(err)
	must(manager.DelRef(&warm))
	return &udocFormulaCleanupFixture{runtime: runtime, manager: manager, engine: engine, predicate: predicate, wide: wide, value: value}
}

func TestDatalogUDocAnswerAllocationFailurePreservesOwners(t *testing.T) {
	failures, receipts, successes := 0, 0, 0
	for allowance := uint64(1); allowance <= 48; allowance++ {
		// Source src/util/mpz.cpp:274-279 swaps the result before replacing scratch;
		// allocation refusal can leave a guarded hole, so each body gets
		// an independent numerical context which is fully disposed below.
		t.Run(fmt.Sprintf("native-count-%d", allowance), func(t *testing.T) {
			f := newUDocFormulaCleanupFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			baseline := f.manager.Stats().LiveReferences
			baselineNodes := f.manager.Stats().LiveNodes
			must(f.runtime.SetMaxCount(uint64(f.runtime.State().GlobalCount) + allowance))
			answer, cause := f.engine.Answer(f.predicate)
			must(f.runtime.SetMaxCount(0))
			var receipt memory.ConstructionFailure
			if errors.As(cause, &receipt) {
				receipts++
				if receipt.RuntimeOwner() != f.runtime {
					t.Fatal("UDoc cleanup lost native runtime")
				}
				shown, exposed := receipt.Error(), receipt.Unwrap()
				must(receipt.Cleanup())
				must(receipt.Cleanup())
				if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil {
					t.Fatal("UDoc cleanup retry changed receipt")
				}
			}
			if cause != nil {
				failures++
				if answer != (ast.ASTRef{}) {
					t.Fatal("failed public Answer exposed a partial formula", answer, cause)
				}
				if got := f.manager.Stats().LiveNodes; got != baselineNodes {
					t.Fatal("failed UDoc Answer leaked partial AST nodes", baselineNodes, got, cause)
				}
			} else {
				successes++
				must(f.manager.DelRef(&answer))
			}
			if got := f.manager.Stats().LiveReferences; got != baseline {
				t.Fatal("UDoc Answer leaked temporary roots", baseline, got, cause)
			}
		})
	}
	if failures == 0 || successes == 0 {
		t.Fatal("campaign missed public Answer failure or success", failures, successes, receipts)
	}
	t.Logf("observed %d failures, %d successes and %d cleanup receipts across 48 public Answer attempts", failures, successes, receipts)
	t.Run("owning-answer", func(t *testing.T) {
		f := newUDocFormulaCleanupFixture(t)
		must := func(err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		}
		answer, err := f.engine.Answer(f.predicate)
		must(err)
		defer func() { must(f.manager.DelRef(&answer)) }()
		formula, err := f.manager.RefGet(answer)
		must(err)
		variable, err := f.manager.MkVar(0, f.wide)
		must(err)
		expected, err := f.manager.MkEq(variable, f.value)
		must(err)
		if formula != expected {
			t.Fatal("UDoc wide fact formula changed", formula, expected)
		}
		status, err := f.engine.QueryPredicate(f.predicate)
		must(err)
		if status != lbool.True {
			t.Fatal("UDoc query changed after Answer", status)
		}
		if got, err := f.manager.RefGet(answer); err != nil || got != formula {
			t.Fatal("owning UDoc answer lost replaced query", got, err)
		}
	})
}

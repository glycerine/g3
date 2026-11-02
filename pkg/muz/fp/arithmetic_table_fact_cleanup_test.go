package fp

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestArithmeticTableFactInsertionAndCleanupRefusalKeepsRoots(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 29)
	var engine *PropositionalEngine
	var attempt *arithmeticTableFactAttempt
	var declaration ast.ASTHandle
	var failure, switchFailure error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			// This is the production insertion/cleanup stage reached by
			// AddTableFact after constructing its owning numeral roots.
			failure = attempt.addFact(engine, declaration)
			switchFailure = engine.selectBackend(EngineSpacer)
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	engine, err = NewPropositionalEngine(manager)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if attempt != nil {
			must(attempt.Close())
		}
		must(engine.Close())
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("arithmetic fact roots leaked", budget.Used())
		}
	}()
	must(engine.SelectEngine("datalog"))
	family, err := manager.ArithmeticFamilyID()
	must(err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("native-fact-owner")
	must(err)
	declaration, err = manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(declaration))
	must(engine.AddTableFact(declaration, []uint32{1}))
	status, err := engine.QueryPredicate(declaration)
	must(err)
	if status != lbool.Undef {
		t.Fatal(status)
	}
	two, err := manager.MkDatalogNumeral(2, integer)
	must(err)
	root, err := manager.NewRef(two)
	must(err)
	attempt = &arithmeticTableFactAttempt{manager: manager, roots: []ast.ASTRef{root}, handles: []ast.ASTHandle{two}}
	owner := engine.boundDatalog
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !callback || !errors.Is(failure, ast.ErrASTPending) || !errors.Is(switchFailure, ast.ErrASTPending) {
		t.Fatal("expected actual native insertion/cleanup refusal", body, callback, failure, switchFailure)
	}
	if len(attempt.roots) != 1 || attempt.roots[0] != root || len(engine.arithmeticFactCleanup) != 1 || engine.arithmeticFactCleanup[0] != attempt {
		t.Fatal("refusal lost owning numeral roots")
	}
	if engine.queryReady || engine.boundDatalog != owner {
		t.Fatal("refused backend switch exposed stale result or discarded bound owner")
	}
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || receipt.RuntimeOwner() != runtime || receipt.OriginalCause() == nil || receipt.CleanupCause() == nil {
		t.Fatal("cleanup refusal lacks native retry receipt", failure)
	}
	must(receipt.Cleanup())
	must(engine.clearArithmeticTableFactCleanup())
	if len(attempt.roots) != 0 || len(engine.arithmeticFactCleanup) != 0 || receipt.CleanupCause() != nil {
		t.Fatal("native retry left root owners")
	}
	status, err = engine.QueryPredicate(declaration)
	must(err)
	if status != lbool.Undef {
		t.Fatal("failed insertion corrupted prior bound fact", status)
	}
}

package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestStoredFactFormulaAttemptDrainsNativeCleanupFrontiers(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var manager *ast.ASTManager
	var nestedRoots []ast.ASTRef
	var attempt *storedFactFormulaAttempt
	var pending memory.ConstructionFailure
	active, called := false, false
	original := errors.New("query copy failed")
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !called {
			called = true
			nested := closeASTRefs(manager, nestedRoots)
			nestedRoots = nil
			failure := attempt.fail(errors.Join(original, nested))
			if !errors.Is(failure, original) || !errors.Is(failure, ast.ErrASTPending) || !errors.As(failure, &pending) {
				t.Fatalf("stored fact finalizer owner lost: %v", failure)
			}
			if len(attempt.current) != 1 || len(attempt.outputs) != 2 || attempt.nested == nil {
				t.Fatal("finalizer lost an ownership frontier")
			}
			for i := 0; i < 2; i++ {
				if err := pending.Cleanup(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatalf("native refusal changed on retry: %v", err)
				}
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("stored fact roots leaked bytes", budget.Used())
		}
	}()
	fragmentMust(t, manager.RegisterDeclarationPlugins())
	engine, err := NewPropositionalEngine(manager)
	must(err)
	defer func() { must(engine.Close()) }()
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("stored-attempt-p")
	must(err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(declaration))
	must(engine.AddTableFact(declaration, []uint32{0}))
	must(engine.AddTableFact(declaration, []uint32{1}))
	baseline := manager.Stats().LiveReferences
	outputs, err := engine.StoredFactFormulas()
	must(err)
	current, err := engine.relations.DecodeFact([]ast.ASTHandle{boolean}, rel.Fact{1})
	must(err)
	extra, err := manager.CopyRef(current[0])
	must(err)
	nestedRoots = []ast.ASTRef{extra}
	attempt = &storedFactFormulaAttempt{manager: manager, current: current, outputs: outputs}
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
	if body == nil || !called || pending == nil {
		t.Fatal("native refusal missed stored fact finalizer cleanup", body, called)
	}
	if pending.RuntimeOwner() != runtime || pending.CleanupCause() == nil {
		t.Fatal("cleanup owner lost runtime/cause")
	}
	origin, exposed, message := pending.OriginalCause(), errors.Unwrap(pending), pending.Error()
	must(pending.Cleanup())
	must(pending.Cleanup())
	if pending.CleanupCause() != nil || pending.RuntimeOwner() != runtime {
		t.Fatal("cleanup completion lost stable ownership")
	}
	if pending.OriginalCause() != origin || errors.Unwrap(pending) != exposed || pending.Error() != message {
		t.Fatal("cleanup completion changed the stored-fact receipt")
	}
	if manager.Stats().LiveReferences != baseline {
		t.Fatal("stored fact finalizer retry leaked roots", baseline, manager.Stats().LiveReferences)
	}
}

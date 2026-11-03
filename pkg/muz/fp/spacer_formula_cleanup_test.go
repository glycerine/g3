package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSlicedFormulaCopiedOwnersSurviveNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var m *ast.ASTManager
	var declaration ast.ASTHandle
	var source ast.ASTRef
	var completed *slicedFormulaAttempt
	var failures []memory.ConstructionFailure
	active, called := false, false
	originalCause := errors.New("conversion failed after result copy")
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !called {
			called = true
			result, cause := expandSlicedFormula(m, map[ast.ASTHandle][]bool{declaration: {true, false}}, declaration, source)
			var cleanup memory.ConstructionFailure
			if result != (ast.ASTRef{}) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &cleanup) {
				t.Fatalf("source conversion dropped refused owner: %v", cause)
			}
			failures = append(failures, cleanup)
			result, cause = completed.finish(originalCause)
			if result != (ast.ASTRef{}) || !errors.Is(cause, originalCause) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &cleanup) {
				t.Fatalf("copied result dropped refused owners: %v", cause)
			}
			failures = append(failures, cleanup)
			for _, pending := range failures {
				for i := 0; i < 2; i++ {
					if cause := pending.Cleanup(); !errors.Is(cause, ast.ErrASTPending) {
						t.Fatalf("retry escaped native guard: %v", cause)
					}
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
	m, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("conversion cleanup leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := symbols.Intern("sliced-cleanup-p")
	must(err)
	declaration, err = m.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	pin, err := m.NewRef(declaration)
	must(err)
	defer func() { must(m.DelRef(&pin)) }()
	x, err := m.MkVar(0, boolean)
	must(err)
	y, err := m.MkVar(1, boolean)
	must(err)
	xpin, err := m.NewRef(x)
	must(err)
	defer func() { must(m.DelRef(&xpin)) }()
	ypin, err := m.NewRef(y)
	must(err)
	defer func() { must(m.DelRef(&ypin)) }()
	baseline := m.Stats().LiveReferences
	source, err = m.NewRef(x)
	must(err)
	completed = &slicedFormulaAttempt{manager: m}
	completed.source, err = m.NewRef(x)
	must(err)
	completed.result, err = m.NewRef(y)
	must(err)
	binding, err := m.NewRef(y)
	must(err)
	completed.bindings = []ast.ASTRef{binding}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !called || len(failures) != 2 {
		t.Fatal("native refusal did not reach both conversion continuations", body, called)
	}
	got, err := m.RefGet(source)
	must(err)
	if got != x {
		t.Fatal("source formula changed during refusal")
	}
	got, err = m.RefGet(completed.result)
	must(err)
	if got != y {
		t.Fatal("copied result changed during refusal")
	}
	if len(completed.bindings) != 1 {
		t.Fatal("refused cleanup lost binding owners")
	}
	got, err = m.RefGet(completed.bindings[0])
	must(err)
	if got != y {
		t.Fatal("binding changed during cleanup refusal")
	}
	for _, failure := range failures {
		must(failure.Cleanup())
		must(failure.Cleanup())
	}
	if got := m.Stats().LiveReferences; got != baseline {
		t.Fatal("cleanup did not release all conversion owners", baseline, got)
	}
	// A fresh conversion must still perform the original variable expansion.
	fresh, err := m.NewRef(x)
	must(err)
	result, err := expandSlicedFormula(m, map[ast.ASTHandle][]bool{declaration: {true, false}}, declaration, fresh)
	must(err)
	got, err = m.RefGet(result)
	must(err)
	if got != y {
		t.Fatal("retry lost sliced variable expansion", got, y)
	}
	must(m.DelRef(&result))
}

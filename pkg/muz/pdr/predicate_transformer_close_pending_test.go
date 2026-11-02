package pdr

// Source: pred_transformer ownership in pdr_context.cpp. A refused destructor
// leaves its AST roots reachable but must reject use until cleanup resumes.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPredicateTransformerRejectsUseDuringRefusedCleanup(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var transformer *PredicateTransformer
	active, callback := false, false
	var closeErr error
	var runtime *memory.Runtime
	var err error
	runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			root := transformer.references[0]
			closeErr = transformer.Close()
			if !errors.Is(closeErr, ast.ErrASTPending) || !transformer.closing || transformer.closed || transformer.references[0] != root {
				t.Fatalf("refused transformer cleanup lost its AST owner: %v", closeErr)
			}
			if _, err := transformer.Head(); !errors.Is(err, ast.ErrASTPending) {
				t.Fatalf("closing transformer accepted use: %v", err)
			}
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
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if transformer != nil {
			must(transformer.Close())
		}
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("pending predicate transformer fixture leaked bytes", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("pending-predicate-transformer")
	must(err)
	head, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	root, err := manager.NewRef(head)
	must(err)
	transformer = &PredicateTransformer{manager: manager, head: head, references: []ast.ASTRef{root}}
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
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) {
		t.Fatal("numerical callback did not refuse transformer cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(transformer.Close())
	must(transformer.Close())
	if !transformer.closed || transformer.closing || transformer.manager != nil || len(transformer.references) != 0 {
		t.Fatal("transformer cleanup retry did not finish")
	}
	if _, err := transformer.Head(); !errors.Is(err, ErrPredicateTransformerOwner) {
		t.Fatal("closed transformer did not report its terminal owner error", err)
	}
}

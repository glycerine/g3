package fp

import (
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRModelPresentationPreservesRefusedCleanupOwner(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var m *ast.ASTManager
	var owner *pdrModelPresentation
	var pending memory.ConstructionFailure
	active, called := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !called {
			called = true
			cause := owner.finish(io.ErrClosedPipe)
			if !errors.Is(cause, io.ErrClosedPipe) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) || owner.model == nil {
				t.Fatalf("presentation dropped model or print failure: %v", cause)
			}
			for i := 0; i < 2; i++ {
				if cause := pending.Cleanup(); !errors.Is(cause, ast.ErrASTPending) || owner.model == nil {
					t.Fatalf("guarded cleanup lost model: %v", cause)
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
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			must(m.DelRef(&pins[i]))
		}
	}()
	declare := func(name string) ast.ASTHandle {
		s, err := symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(s, nil, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		pin, err := m.NewRef(d)
		must(err)
		pins = append(pins, pin)
		return d
	}
	fact, query := declare("presentation-fact"), declare("presentation-query")
	truth, err := m.MkTrue()
	must(err)
	falsity, err := m.MkFalse()
	must(err)
	baseline := m.Stats().LiveReferences
	md, err := model.NewModel(m)
	must(err)
	must(md.Core().RegisterConstant(fact, truth))
	must(md.Core().RegisterConstant(query, falsity))
	owner = &pdrModelPresentation{manager: m, model: md}
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
	if body == nil || !called || pending == nil {
		t.Fatal("native refusal did not reach presentation cleanup", body, called)
	}
	got, err := owner.model.Core().ConstantInterpretation(fact)
	must(err)
	if got != truth {
		t.Fatal("refused cleanup lost original fact interpretation")
	}
	got, err = owner.model.Core().ConstantInterpretation(query)
	must(err)
	if got != falsity {
		t.Fatal("refused cleanup lost query refutation")
	}
	if pending.RuntimeOwner() != runtime || pending.CleanupCause() == nil {
		t.Fatal("cleanup continuation lost runtime or refusal")
	}
	must(pending.Cleanup())
	must(pending.Cleanup())
	if owner.model != nil || pending.CleanupCause() != nil {
		t.Fatal("successful cleanup did not retire model")
	}
	if got := m.Stats().LiveReferences; got != baseline {
		t.Fatal("presentation model leaked references", baseline, got)
	}
}

package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerLiteralExpansionPreservesRootsDuringPendingCleanup(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 24)
	var expansion literalExpansion
	active, callbacks, intact := false, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				cause := expansion.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && len(expansion.refs) == 1 && len(expansion.literals) == 1
			}
		}
		return source
	}})
	must(err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		must(expansion.Close())
		must(manager.Close())
		must(numerals.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("literal expansion pending fixture leaked bytes", budget.Used())
		}
	}()
	must(manager.RegisterDeclarationPlugins())
	truth, err := manager.MkTrue()
	must(err)
	expansion.manager = manager
	expansion.literals = []ast.ASTHandle{truth}
	_, err = expansion.keep(truth, nil)
	must(err)
	value, err := numerals.NewValue()
	must(err)
	defer func() { must(numerals.Del(&value)) }()
	must(numerals.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("pending cleanup did not preserve expansion roots", body, callbacks, intact)
	}
	if expansion.refs[0] != truth || expansion.literals[0] != truth {
		t.Fatal("pending cleanup changed expansion identity")
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(expansion.Close())
	must(expansion.Close())
	if len(expansion.refs) != 0 || expansion.literals != nil {
		t.Fatal("cleanup retry did not retire expansion owners")
	}
}

package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerRuleReplacementPreservesNativeRefusedSnapshot(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var m *ast.ASTManager
	var engine *PropositionalEngine
	var snapshot *base.RuleSet
	active, called, preserved := false, false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !called {
			called = true
			first := engine.closeSpacerForReplacement()
			second := engine.closeSpacerForReplacement()
			preserved = errors.Is(first, ast.ErrASTPending) && errors.Is(second, ast.ErrASTPending) && engine.spacerRules == snapshot
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
	name, err := symbols.Intern("replacement-snapshot")
	must(err)
	declaration, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	snapshot, err = base.NewRuleSet(rules)
	must(err)
	defer func() { must(snapshot.Close()) }()
	must(rules.RegisterPredicate(declaration))
	must(snapshot.SetOutputPredicate(declaration))
	engine = &PropositionalEngine{manager: m, spacerRules: snapshot}
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
	if body == nil || !called || !preserved {
		t.Fatal("replacement lost refused source snapshot", body, called, preserved)
	}
	must(engine.closeSpacerForReplacement())
	must(engine.closeSpacerForReplacement())
	if engine.spacerRules != nil {
		t.Fatal("replacement retry did not retire snapshot")
	}
}

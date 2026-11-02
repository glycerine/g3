package transforms

// Source: dl_mk_bit_blast.cpp impl simplifier/bit-blaster ownership.
// Its Go child rewriter must remain reachable when native cleanup is refused.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	bitblaster "github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBitBlastClosePreservesPendingChildren(t *testing.T) {
	for _, completed := range []bool{false, true} {
		name := "initial"
		if completed {
			name = "after-simplifier-disposal"
		}
		t.Run(name, func(t *testing.T) { testBitBlastPendingChildren(t, completed) })
	}
}
func testBitBlastPendingChildren(t *testing.T, completed bool) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var plugin *BitBlastPlugin
	active, callbacks, intact := false, 0, true
	var original *bitblaster.BitBlasterRewriter
	var predicate, expanded ast.ASTHandle
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				cause := plugin.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !plugin.closed && plugin.blaster == original && plugin.expansion.oldToNew[predicate] == expanded
				if completed {
					intact = intact && plugin.simplifier == nil
				} else {
					intact = intact && plugin.simplifier != nil && !plugin.simplifier.closed
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
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	set, err := base.NewRuleSet(rules)
	must(err)
	defer func() { must(set.Close()) }()
	parameters, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, parameters, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	defer func() { must(globals.Close()); must(catalog.Close()); must(parameters.Close()) }()
	_, err = m.RegisterBitVectorPlugin()
	must(err)
	bv, err := m.BitVectorSort(3)
	must(err)
	name, err := symbols.Intern("pending-bit-vector-predicate")
	must(err)
	predicate, err = m.MkFuncDecl(name, []ast.ASTHandle{bv}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(predicate))
	x, err := m.MkVar(0, bv)
	must(err)
	head, err := m.MkApp(predicate, []ast.ASTHandle{x})
	must(err)
	five, err := m.BitVectorNumeralUnsigned(5, 3)
	must(err)
	guard, err := m.MkEq(x, five)
	must(err)
	rule, err := rules.NewRule(head, []ast.ASTHandle{guard}, nil, "pending-bit-vector-rule")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	must(set.SetOutputPredicate(predicate))

	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("invalid fixture")
	}
	plugin, err = NewBitBlastPlugin(m, rules, parameters, globals, 1)
	must(err)
	defer func() { must(plugin.Close()) }()
	transformed, err := plugin.Transform(set)
	must(err)
	if transformed == nil {
		t.Fatal("fixture did not expand bit-vector predicate")
	}
	expanded = plugin.FreshPredicates()[predicate]
	if expanded.IsNull() {
		t.Fatal("missing expanded predicate")
	}
	arity, err := m.GetArity(expanded)
	must(err)
	if arity != 3 {
		t.Fatal("predicate did not expand to three bits", arity)
	}
	rewritten, err := transformed.Rule(0)
	must(err)
	declaration, err := rewritten.HeadDeclaration()
	must(err)
	if declaration != expanded {
		t.Fatal("expanded rule head mismatch")
	}
	must(rewritten.Close())
	must(transformed.Close())
	original = plugin.blaster

	if completed {
		// Stage a legitimate continuation state by successfully disposing the
		// earlier child, then refuse the later child at its real native boundary.
		// This does not claim an originating post-admission failure was injected.
		must(plugin.simplifier.Close())
	}

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
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("refused close lost rewriter ownership", body, callbacks, intact)
	}

	if next, cause := plugin.Transform(set); !errors.Is(cause, base.ErrRuleTransformerOwner) || next != nil {
		t.Fatal("closing plugin accepted transformation", cause)
	}

	must(plugin.Close())
	must(plugin.Close())
	if !plugin.closed || plugin.blaster != nil || plugin.simplifier != nil || plugin.expansion.oldToNew != nil || plugin.expansion.source != nil || plugin.expansion.target != nil {
		t.Fatal("retry did not release child rewriter")
	}
}

package transforms

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

func TestSliceProofClosePreservesRootsWhileManagerExecutes(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var converter *SliceProofConverter
	active, callbacks, intact := false, 0, true
	var closeErr error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			count := len(converter.formulas)
			if len(converter.originals) == 0 {
				intact = false
				return f
			}
			original := converter.originals[0]
			closeErr = converter.Close()
			intact = intact && errors.Is(closeErr, ast.ErrASTPending) && converter.manager != nil && len(converter.formulas) == count && len(converter.originals) > 0 && converter.originals[0] == original
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
			t.Fatal("slice proof fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	name, err := symbols.Intern("pending-original")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(predicate))
	name, err = symbols.Intern("pending-sliced")
	must(err)
	slicedPredicate, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(slicedPredicate))
	variable, err := m.MkVar(0, integer)
	must(err)
	head, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	must(err)
	original, err := rules.NewRule(head, nil, nil, "original")
	must(err)
	defer func() { must(original.Close()) }()
	must(rules.MkRuleAssertedProof(original))
	head, err = m.MkApp(slicedPredicate, nil)
	must(err)
	sliced, err := rules.NewRule(head, nil, nil, "sliced")
	must(err)
	defer func() { must(sliced.Close()) }()
	must(rules.MkRuleAssertedProof(sliced))
	converter = newSliceProofConverter(m, rules, map[ast.ASTHandle]ast.ASTHandle{predicate: slicedPredicate}, map[ast.ASTHandle][]bool{predicate: {true}})
	must(converter.insert(original, sliced, nil))
	proof, found, err := sliced.Proof()
	must(err)
	if !found {
		t.Fatal("missing sliced proof")
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
	if body == nil || callbacks == 0 || !intact || !errors.Is(closeErr, ast.ErrASTPending) {
		t.Fatal("pending close lost its actual proof roots", body, callbacks, intact, closeErr)
	}
	converted, err := converter.Convert(proof)
	must(err)
	node, err := m.RefGet(converted)
	must(err)
	expected, found, err := original.Proof()
	must(err)
	if !found || node != expected {
		t.Fatal("rejected close changed original-proof mapping")
	}
	must(m.DelRef(&converted))
	must(converter.Close())
	must(converter.Close())
}

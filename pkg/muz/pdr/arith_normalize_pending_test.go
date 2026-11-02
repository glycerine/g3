package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestArithmeticNormalizationCleanupKeepsUnconsumedRootsForRetry(t *testing.T) {
	budget := arena.NewBudget(1 << 28)
	var n arithmeticNormalization
	active, callbacks := false, 0
	var cleanupErr error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			before := len(n.refs)
			cleanupErr = n.Cleanup()
			if !errors.Is(cleanupErr, ast.ErrASTPending) || len(n.refs) != before || before == 0 {
				t.Fatal("executing manager consumed normalization roots", cleanupErr, before, len(n.refs))
			}
		}
		return f
	}})
	closureMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	closureMust(t, err)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	closureMust(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	closureMust(t, err)
	n.manager = m
	defer func() {
		active = false
		closureMust(t, runtime.SetMaxCount(0))
		closureMust(t, n.Cleanup())
		closureMust(t, m.Close())
		closureMust(t, numerals.Close())
		closureMust(t, symbols.Close())
		closureMust(t, runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("normalization pending fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	closureMust(t, err)
	two, err := m.ArithmeticRealNumeralText("2")
	closureMust(t, err)
	bound, err := m.ArithmeticRealNumeralText("106")
	closureMust(t, err)
	sort, err := m.GetSort(two)
	closureMust(t, err)
	x, err := m.MkFreshConst("pending-normalization", sort)
	closureMust(t, err)
	product, err := m.MkArithmeticMul(two, x)
	closureMust(t, err)
	input, err := m.MkArithmeticLE(product, bound)
	closureMust(t, err)
	inputRef, err := m.NewRef(input)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&inputRef)) }()
	value, err := numerals.NewValue()
	closureMust(t, err)
	defer func() { closureMust(t, numerals.Del(&value)) }()
	closureMust(t, numerals.SetString(&value, "18446744073709551617/18446744073709551619"))
	before := m.Stats()
	output, err := n.tree(input, make(map[ast.ASTHandle]ast.ASTHandle))
	closureMust(t, err)
	if output == input || len(n.refs) == 0 {
		t.Fatal("fixture did not own normalized temporaries")
	}
	roots := append([]ast.ASTHandle(nil), n.refs...)
	closureMust(t, runtime.SetMaxCount(uint64(runtime.State().GlobalCount)+1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	closureMust(t, runtime.SetMaxCount(0))
	if body == nil || callbacks == 0 || !errors.Is(cleanupErr, ast.ErrASTPending) {
		t.Fatal("numerical failure did not exercise pending cleanup", body, callbacks, cleanupErr)
	}
	for index, root := range roots {
		if n.refs[index] != root {
			t.Fatal("pending cleanup reordered ownership roots")
		}
		_, err := m.GetKind(root)
		closureMust(t, err)
	}
	closureMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
	closureMust(t, n.Cleanup())
	closureMust(t, n.Cleanup())
	if len(n.refs) != 0 || n.CleanupCause() != nil {
		t.Fatal("cleanup continuation did not finish")
	}
	after := m.Stats()
	if before.LiveNodes != after.LiveNodes || before.LiveReferences != after.LiveReferences {
		t.Fatal("cleanup continuation leaked normalized nodes or references", before, after)
	}
	current, err := m.RefGet(inputRef)
	closureMust(t, err)
	if current != input {
		t.Fatal("interrupted cleanup changed the caller's formula")
	}
}

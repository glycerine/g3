package solver

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func checkSatResultRefusalFixture(t *testing.T, cleanup func() error) (*ast.ASTManager, func() error) {
	t.Helper()
	budget := arena.NewBudget(1 << 28)
	active := false
	var failure error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && failure == nil {
			failure = cleanup()
		}
		return source
	}})
	solverMust(t, err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	solverMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	solverMust(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	solverMust(t, err)
	t.Cleanup(func() {
		solverMust(t, m.Close())
		solverMust(t, numbers.Close())
		solverMust(t, symbols.Close())
		solverMust(t, runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("result cleanup leaked bytes", budget.Used())
		}
	})
	_, err = m.RegisterArithmeticPlugin()
	solverMust(t, err)
	return m, func() error {
		t.Helper()
		value, err := numbers.NewValue()
		solverMust(t, err)
		solverMust(t, numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
		solverMust(t, runtime.SetMaxCount(uint64(runtime.State().GlobalCount)+1))
		var invocation ast.ASTArithmeticNumeralInvocation
		active = true
		_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
		active = false
		solverMust(t, runtime.SetMaxCount(0))
		solverMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
		solverMust(t, numbers.Del(&value))
		if body == nil || !errors.Is(failure, ast.ErrASTPending) {
			t.Fatal("native cleanup did not refuse while the AST invocation was active", body, failure)
		}
		return failure
	}
}

func TestSimpleCheckSatResultCloseResumesConsumedCorePrefix(t *testing.T) {
	var result *SimpleCheckSatResult
	m, refuse := checkSatResultRefusalFixture(t, func() error { return result.Close() })
	result, err := NewSimpleCheckSatResult(m)
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, result.Close()) })
	atom := namedOwnerConst(t, m, "result-core-frontier")
	solverMust(t, result.SetUnsatCore([]ast.ASTHandle{atom, atom}))
	// Model the exact state after DelRef consumed the first token before
	// returning an error. The second token and the proof remain owned.
	solverMust(t, m.DelRef(&result.core[0]))
	live, proof := result.core[1], result.proof
	refuse()
	if result.closed || len(result.core) != 1 || result.core[0] != live || result.proof != proof {
		t.Fatal("refused cleanup lost its remaining core or proof", result.core, result.proof, result.closed)
	}
	count, err := m.GetRefCount(atom)
	solverMust(t, err)
	if count != 2 {
		t.Fatal("refused cleanup changed the live core reference", count)
	}
	solverMust(t, result.Close())
	solverMust(t, result.Close())
	count, err = m.GetRefCount(atom)
	solverMust(t, err)
	if !result.closed || len(result.core) != 0 || result.proof != (ast.ASTRef{}) || count != 1 {
		t.Fatal("cleanup retry did not retire each result owner once", result.closed, result.core, result.proof, count)
	}
}

func TestSimpleCheckSatResultCloseResumesProofTail(t *testing.T) {
	for _, consumed := range []bool{false, true} {
		t.Run(map[bool]string{false: "refused", true: "consumed"}[consumed], func(t *testing.T) {
			var result *SimpleCheckSatResult
			m, refuse := checkSatResultRefusalFixture(t, func() error { return result.Close() })
			solverMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			result, err := NewSimpleCheckSatResult(m)
			solverMust(t, err)
			t.Cleanup(func() { solverMust(t, result.Close()) })
			atom := namedOwnerConst(t, m, "result-proof-frontier")
			proof, err := m.MkProofAsserted(atom)
			solverMust(t, err)
			solverMust(t, result.SetProof(proof))
			token := result.proof
			refuse()
			count, err := m.GetRefCount(proof)
			solverMust(t, err)
			if result.closed || result.proof != token || count != 1 {
				t.Fatal("refused proof cleanup consumed or lost its owner", result.closed, result.proof, count)
			}
			if consumed {
				// A later release may consume its ASTRef before reporting an
				// error; Close must not attempt to release that zero token.
				solverMust(t, m.DelRef(&result.proof))
			}
			solverMust(t, result.Close())
			solverMust(t, result.Close())
			count, err = m.GetRefCount(atom)
			solverMust(t, err)
			if !result.closed || result.proof != (ast.ASTRef{}) || count != 1 {
				t.Fatal("proof cleanup retry did not release exactly once", result.closed, result.proof, count)
			}
		})
	}
}

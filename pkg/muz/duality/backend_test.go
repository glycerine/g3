package duality_test

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	graph "github.com/glycerine/g3/pkg/duality"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityOwnedBackendIntegerQueriesAndBorrowedServiceLifetime(t *testing.T) {
	for _, mbqi := range []bool{false, true} {
		name := "without-mbqi"
		if mbqi {
			name = "with-mbqi"
		}
		t.Run(name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			owner, err := cmd_context.NewStandaloneContext()
			must(err)
			defer owner.Close()
			m, err := owner.Manager()
			must(err)
			config, err := owner.SolverTranslationConfig()
			must(err)
			initial, err := m.ProofMode()
			must(err)
			backend, err := duality.NewBackend(m, config, mbqi)
			must(err)
			defer backend.Close()
			copied := *backend
			if err := copied.Close(); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("copied backend accepted", err)
			}
			var refs []ast.ASTRef
			defer func() {
				for i := range refs {
					must(m.DelRef(&refs[i]))
				}
			}()
			pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				must(err)
				ref, err := m.NewRef(h)
				must(err)
				refs = append(refs, ref)
				return h
			}
			zero := pin(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
			one := pin(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
			integer, err := m.GetSort(zero)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			symbol, err := m.Symbols.Intern("backend-integer-P")
			must(err)
			predicate := pin(m.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
			fact := pin(m.MkApp(predicate, []ast.ASTHandle{zero}))
			queryOne := pin(m.MkApp(predicate, []ast.ASTHandle{one}))
			var previous *graph.Counterexample
			for round, query := range []ast.ASTHandle{queryOne, fact, queryOne} {
				options := duality.DefaultQueryOptions()
				if round == 1 {
					options.StratifiedInlining = true
				}
				must(backend.SetOptions(options))
				prepared, err := duality.PrepareQuery(m, []ast.ASTHandle{fact}, []uint32{math.MaxUint32}, query, []ast.ASTHandle{predicate}, math.MaxUint32)
				must(err)
				status, err := backend.Query(prepared)
				must(err)
				must(prepared.Close())
				expected := lbool.False
				if round == 1 {
					expected = lbool.True
				}
				if status != expected {
					t.Fatalf("integer query %d: %v want %v", round, status, expected)
				}
				if previous != nil && previous.Root() != nil {
					t.Fatal("old backend counterexample still owned")
				}
				previous = backend.Counterexample()
				if round == 1 && (previous == nil || previous.Root() == nil) {
					t.Fatal("missing integer counterexample")
				}
				if backend.IsResultRecursionBounded() {
					t.Fatal("unbounded integer query marked bounded")
				}
				mode, err := m.ProofMode()
				must(err)
				if mode != initial {
					t.Fatal("backend leaked proof mode")
				}
			}
			must(backend.Close())
			must(backend.Close())
			if _, err := backend.Query(nil); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("closed backend accepted query", err)
			}
			// Closing the backend must leave the command owner's services usable.
			_, err = config.ModelEvaluator.Cooperation.Enabled()
			must(err)
			_, err = config.ModelEvaluator.Memory.AllocationSize()
			must(err)
			p := config.Params.EmptyValue()
			must(config.Params.Del(&p))
			m.Budget().FailAfter(0)
			failed, allocationErr := duality.NewBackend(m, config, mbqi)
			m.Budget().FailAfter(-1)
			if failed != nil || allocationErr == nil {
				t.Fatal("refused backend allocation unexpectedly succeeded", failed, allocationErr)
			}
			mode, err := m.ProofMode()
			must(err)
			if mode != initial {
				t.Fatal("failed backend construction leaked proof mode")
			}
			retry, err := duality.NewBackend(m, config, mbqi)
			must(err)
			must(retry.Close())
			broken := config
			broken.ModelEvaluator.Memory = nil
			if failed, err := duality.NewBackend(m, broken, mbqi); err == nil || failed != nil {
				t.Fatal("missing borrowed service accepted", failed, err)
			}
		})
	}
}

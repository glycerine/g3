package pdr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestNormalizeArithmeticColdFailureOwnershipAndRetry(t *testing.T) {
	succeeded, failures := false, 0
	stops := make([]int64, 32)
	for i := range stops {
		stops[i] = int64(i)
	}
	for stop := int64(32); stop <= 32768; stop *= 2 {
		stops = append(stops, stop)
	}
	for _, stop := range stops {
		if succeeded {
			break
		}
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, owner, transformer, _, closeBase := closureFixture(t)
			defer func() {
				closureMust(t, transformer.Close())
				closureMust(t, owner.Close())
				closeBase()
			}()
			two, err := m.ArithmeticRealNumeralText("2")
			closureMust(t, err)
			sort, err := m.GetSort(two)
			closureMust(t, err)
			name, err := m.Symbols.Intern("cold-normalization-x")
			closureMust(t, err)
			x, err := m.MkConst(name, sort)
			closureMust(t, err)
			product, err := m.MkArithmeticMul(two, x)
			closureMust(t, err)
			// Exceed the warm node pages so cold reservation failures are
			// reached while earlier sibling rewrites already own new terms.
			clauses := make([]ast.ASTHandle, 600)
			for i := range clauses {
				bound, err := m.ArithmeticRealNumeralText(fmt.Sprint(2*i + 1001))
				closureMust(t, err)
				relation, err := m.MkArithmeticLT(product, bound)
				closureMust(t, err)
				clauses[i], err = m.MkNot(relation)
				closureMust(t, err)
			}
			input, err := m.MkAndArray(clauses)
			closureMust(t, err)
			source, err := m.NewRef(input)
			closureMust(t, err)
			defer func() { closureMust(t, m.DelRef(&source)) }()
			ref, err := m.NewRef(input)
			closureMust(t, err)
			defer func() { closureMust(t, m.DelRef(&ref)) }()
			closureMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			err = NormalizeArithmeticRef(m, ref)
			m.Budget().FailAfter(-1)
			if err != nil {
				failures++
				current, getErr := m.RefGet(ref)
				closureMust(t, getErr)
				if current != input {
					t.Fatal("interrupted rewrite replaced the original formula")
				}
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					closureMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected failure", err)
				}
			} else {
				succeeded = true
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatalf("live references = %d, want %d", got, before)
			}
			if mode, err := m.ProofMode(); err != nil || mode != ast.ASTProofEnabled {
				t.Fatal("proof mode", mode, err)
			}
			closureMust(t, m.SetRef(ref, input))
			closureMust(t, NormalizeArithmeticRef(m, ref))
			output, err := m.RefGet(ref)
			closureMust(t, err)
			if output == input {
				t.Fatal("retry did not normalize nested strict comparison")
			}
			// Release the result while preserving the input, then compare node counts
			// with another complete cycle to catch escaped temporary coefficients.
			closureMust(t, m.SetRef(ref, input))
			nodes := m.Stats().LiveNodes
			closureMust(t, NormalizeArithmeticRef(m, ref))
			closureMust(t, m.SetRef(ref, input))
			if got := m.Stats().LiveNodes; got != nodes {
				t.Fatalf("retry leaked nodes: %d, want %d", got, nodes)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing failure/success frontier", failures, succeeded)
	}
}

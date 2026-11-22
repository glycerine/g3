package smtcore

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
)

func newSwapBatch(t testing.TB, m *ast.ASTManager) *macros.FormulaBatch {
	t.Helper()
	batch, err := macros.NewFormulaBatch(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	return batch
}

func BenchmarkSMTReplacePendingAssertions(b *testing.B) {
	for _, committed := range []int{0, 128, 4096} {
		for _, pending := range []int{1, 32} {
			b.Run(fmt.Sprintf("committed=%d/pending=%d", committed, pending), func(b *testing.B) {
				m := astTestManager(b)
				k := &SMTKernel{manager: m}
				b.Cleanup(func() { astTestMust(b, k.restoreAssertions(0)) })
				truth, err := m.MkTrue()
				astTestMust(b, err)
				falsity, err := m.MkFalse()
				astTestMust(b, err)
				for range committed {
					astTestMust(b, k.appendAssertion(truth, ast.ASTHandle{}))
				}
				k.assertedHead = committed
				batches := [2]*macros.FormulaBatch{newSwapBatch(b, m), newSwapBatch(b, m)}
				for i, f := range []ast.ASTHandle{truth, falsity} {
					for range pending {
						astTestMust(b, batches[i].Insert(macros.Formula{Expression: f}))
					}
				}
				astTestMust(b, k.swapPendingFormulaBatch(batches[1]))
				i := 0
				b.ReportAllocs()
				for b.Loop() {
					if err := k.swapPendingFormulaBatch(batches[i]); err != nil {
						b.Fatal(err)
					}
					i ^= 1
				}
			})
		}
	}
}

func TestSMTPendingAssertionSwapRollback(t *testing.T) {
	// Moving five pending rows two positions forward overlaps the source.
	// Exercise both spare capacity and growth before publishing a replacement.
	for _, capacity := range []int{6, 16} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			m, s := smtAssertedNNFFixture(t, true)
			k := s.kernel
			k.assertions = make([]SMTKernelAssertion, 0, capacity)
			sort, err := m.MkBoolSort()
			astTestMust(t, err)
			row := func(name string) macros.Formula {
				h, err := m.MkConst(astTestSymbol(t, m, name), sort)
				astTestMust(t, err)
				proof, err := m.MkProofAsserted(h)
				astTestMust(t, err)
				return macros.Formula{Expression: h, Proof: proof}
			}
			for _, name := range []string{"committed", "one", "two", "three", "four", "five"} {
				f := row(name)
				astTestMust(t, k.appendAssertion(f.Expression, f.Proof))
			}
			k.assertedHead = 1
			original := slices.Clone(k.assertions)
			batch := newSwapBatch(t, m)
			first := row("replacement")
			stale := row("stale")
			astTestMust(t, batch.Insert(first))
			astTestMust(t, batch.Insert(stale))
			// Deliberately invalidate the second proof while its expression is
			// still owned. The swap must unwind both the complete first row and
			// the second row's partially acquired reference.
			astTestMust(t, m.DecRef(stale.Proof))
			roots := []ast.ASTHandle{first.Expression, first.Proof, stale.Expression}
			for _, r := range original {
				roots = append(roots, r.Formula, r.Proof)
			}
			counts := make([]uint32, len(roots))
			for i, h := range roots {
				counts[i], err = m.GetRefCount(h)
				astTestMust(t, err)
			}
			if err := k.swapPendingFormulaBatch(batch); !errors.Is(err, ast.ErrASTHandle) {
				t.Fatalf("stale proof returned %v", err)
			}
			if k.assertedHead != 1 || !slices.Equal(k.assertions, original) {
				t.Fatal("partially acquired replacement changed the original assertions")
			}
			for i, h := range roots {
				got, err := m.GetRefCount(h)
				astTestMust(t, err)
				if got != counts[i] {
					t.Fatalf("reference count for root %d changed from %d to %d", i, counts[i], got)
				}
			}
			// Reset releases the remaining good roots and reports the bad proof.
			if err := batch.Reset(); !errors.Is(err, ast.ErrASTHandle) {
				t.Fatalf("reset of invalidated batch returned %v", err)
			}
			astTestMust(t, k.swapPendingFormulaBatch(batch))
			if !slices.Equal(k.assertions, original[:1]) {
				t.Fatal("an empty replacement must retire only the uncommitted rows")
			}
		})
	}
}

package rel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func cleanupUDocProductFailure(t *testing.T, cause error) {
	t.Helper()
	var receipt memory.ConstructionFailure
	if errors.As(cause, &receipt) {
		shown, exposed := receipt.Error(), receipt.Unwrap()
		astTestMust(t, receipt.Cleanup())
		astTestMust(t, receipt.Cleanup())
		if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil {
			t.Fatal("DOC product cleanup changed stable receipt")
		}
	}
}

func TestUDocTableProductJoinAllocationKeepsBorrowedSources(t *testing.T) {
	failures, completed := 0, 0
	for _, allowance := range []int64{0, 1, 2, 4, 8, 16, 32, 64} {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f := newTableRelationFixture(t)
			factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
			astTestMust(t, err)
			t.Cleanup(factory.Close)
			doc, err := factory.NewFull([]ast.ASTHandle{f.bits})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, doc.Close()) })
			sorts, domains, fact := make([]ast.ASTHandle, 512), make([]uint64, 512), make(Fact, 512)
			for i := range sorts {
				sorts[i], domains[i] = f.boolean, 2
			}
			signature, err := NewSignature(domains)
			astTestMust(t, err)
			finite := NewHashTable(signature)
			t.Cleanup(func() { astTestMust(t, finite.Close()) })
			_, err = finite.Add(fact)
			astTestMust(t, err)
			before := f.ast.Stats()
			f.ast.Budget().FailAfter(allowance)
			product, cause := JoinUDocFinite(f.relations, doc, sorts, finite, nil, nil)
			f.ast.Budget().FailAfter(-1)
			cleanupUDocProductFailure(t, cause)
			if cause != nil {
				failures++
				if !errors.Is(cause, arena.ErrInjectedFailure) || product != nil {
					t.Fatal("failed mixed join published owner or changed cause", product, cause)
				}
			} else {
				completed++
				astTestMust(t, product.Close())
			}
			after := f.ast.Stats()
			if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
				t.Fatal("mixed join leaked native AST ownership", before, after, cause)
			}
			if empty, err := doc.Empty(); err != nil || empty {
				t.Fatal("mixed join consumed borrowed DOC", empty, err)
			}
			if found, err := finite.Contains(fact); err != nil || !found {
				t.Fatal("mixed join consumed borrowed table", found, err)
			}
		})
	}
	if failures == 0 || completed == 0 {
		t.Fatal("mixed join allocation campaign missed failure or completion", failures, completed)
	}
}

func TestUDocTableProductFormulaAllocationReleasesUnpublishedNodes(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(factory.Close)
	sort, err := f.ast.BitVectorSort(9)
	astTestMust(t, err)
	astTestPin(t, f.ast, sort)
	doc, err := factory.NewFull([]ast.ASTHandle{sort})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, doc.Close()) })
	finite := udocTableProductFinite(t, []uint64{2}, Fact{1})
	product, err := JoinUDocFinite(f.relations, doc, []ast.ASTHandle{f.boolean}, finite, nil, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, product.Close()) })
	if count, err := product.Table().Len(); err != nil || count != 512 {
		t.Fatal("formula campaign missed source full finite component", count, err)
	}
	// Populate persistent declaration caches before comparing exact live AST
	// counts. A failed constructor can leak nodes without leaking ASTRef slots.
	warm, err := product.ToFormula()
	astTestMust(t, err)
	astTestMust(t, f.ast.DelRef(&warm))
	before := f.ast.Stats()
	failures, completed := 0, 0
	for _, allowance := range []int64{0, 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 4096} {
		f.ast.Budget().FailAfter(allowance)
		formula, cause := product.ToFormula()
		f.ast.Budget().FailAfter(-1)
		cleanupUDocProductFailure(t, cause)
		if cause != nil {
			failures++
			if !errors.Is(cause, arena.ErrInjectedFailure) || formula != (ast.ASTRef{}) {
				t.Fatal("failed product formula published reference or changed cause", allowance, formula, cause)
			}
		} else {
			completed++
			astTestMust(t, f.ast.DelRef(&formula))
		}
		after := f.ast.Stats()
		if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
			t.Fatal("product formula leaked native AST ownership", allowance, before, after, cause)
		}
	}
	if failures == 0 || completed == 0 {
		t.Fatal("product formula campaign missed failure or completion", failures, completed)
	}
}

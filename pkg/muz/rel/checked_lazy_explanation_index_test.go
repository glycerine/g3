package rel

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCheckedLazyExplanationIndexAndDerivedProducts(t *testing.T) {
	pairs := []struct{ primary, checker string }{
		{"lazy_sparse", "hashtable"},
		{"hashtable", "lazy_sparse"},
		{"lazy_sparse", "sparse"},
		{"sparse", "lazy_sparse"},
		{"lazy_sparse", "lazy_sparse"},
	}
	for _, pair := range pairs {
		t.Run(pair.primary+"-checked-by-"+pair.checker, func(t *testing.T) {
			lazy := NewLazySparseFiniteFactory()
			t.Cleanup(func() {
				if err := lazy.Close(); err != nil {
					t.Fatal(err)
				}
			})
			factory, err := NewFiniteCheckedFactoryWithLazy(pair.primary, pair.checker, lazy)
			if err != nil {
				t.Fatal(err)
			}
			signature, err := NewSignature([]uint64{2, math.MaxInt32})
			if err != nil {
				t.Fatal(err)
			}
			index, err := factory.NewExplanationIndex(signature, 1)
			if err != nil {
				t.Fatal(err)
			}
			assertLazyEngine := func(label string, index *CheckedExplanationIndex) {
				t.Helper()
				for side, table := range map[string]FiniteTable{"primary": index.primary, "checker": index.checker} {
					if lazyTable, ok := table.(*LazySparseTable); ok && lazyTable.engine != lazy.engine {
						t.Fatalf("%s %s lazy side used a different engine", label, side)
					}
				}
			}
			assertLazyEngine("index", index)
			if err = index.EnsureFact(Fact{1, 3}); err != nil {
				t.Fatal(err)
			}
			probe := Fact{1, 0}
			if found, err := index.FetchFact(probe); err != nil || !found || probe[1] != 3 {
				t.Fatalf("initial fetch (%v, found=%t, %v), want suffix 3", probe, found, err)
			}
			if err = index.EnsureFact(Fact{1, 5}); err != nil {
				t.Fatal(err)
			}
			if err = index.EnsureFact(Fact{1, Element(math.MaxInt32)}); err == nil {
				t.Fatal("out-of-domain functional suffix accepted")
			}
			probe[1] = 0
			if found, err := index.FetchFact(probe); err != nil || !found || probe[1] != 5 {
				t.Fatalf("failed Ensure changed prior row: %v, found=%t, %v", probe, found, err)
			}
			if removed, err := index.Remove(Fact{1, 4}); err != nil || removed {
				t.Fatalf("wrong-suffix removal (%t, %v), want false", removed, err)
			}
			probe[1] = 0
			if found, err := index.FetchFact(probe); err != nil || !found || probe[1] != 5 {
				t.Fatalf("wrong-suffix removal changed row: %v, found=%t, %v", probe, found, err)
			}
			if removed, err := index.Remove(Fact{1, 5}); err != nil || !removed {
				t.Fatalf("exact removal (%t, %v), want true", removed, err)
			}
			if err = index.Close(); err != nil {
				t.Fatal(err)
			}

			mismatch, err := factory.NewExplanationIndex(signature, 1)
			if err != nil {
				t.Fatal(err)
			}
			assertLazyEngine("mismatch index", mismatch)
			if err = mismatch.EnsureFact(Fact{0, 7}); err != nil {
				t.Fatal(err)
			}
			if err = explanationIndexEnsure(mismatch.checker, Fact{0, 9}, 1); err != nil {
				t.Fatal(err)
			}
			if _, err = mismatch.Facts(); !errors.Is(err, ErrCheckTable) {
				t.Fatalf("suffix-only mismatch error %v, want ErrCheckTable", err)
			}
			if err = mismatch.Close(); err != nil {
				t.Fatal(err)
			}

			f := newTableRelationFixture(t)
			if err = f.relations.SetLazySparseFiniteFactory(lazy); err != nil {
				t.Fatal(err)
			}
			if err = f.relations.SetDefaultFiniteTable("lazy_sparse"); err != nil {
				t.Fatal(err)
			}
			if err = f.relations.SetCheckedFiniteFactory(factory); err != nil {
				t.Fatal(err)
			}
			nullarySignature, err := NewSignature(nil)
			if err != nil {
				t.Fatal(err)
			}
			if factory.CanHandleSignature(nullarySignature) {
				t.Fatal("lazy checked ordinary table claimed nullary signature")
			}
			ordinaryNullary, err := f.relations.NewDefaultFiniteTable(nullarySignature)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := ordinaryNullary.(*HashTable); !ok {
				t.Fatalf("ordinary nullary fallback type %T, want *HashTable", ordinaryNullary)
			}
			if err = ordinaryNullary.Close(); err != nil {
				t.Fatal(err)
			}

			ruleSort, err := f.ast.MkDatalogRuleSort()
			astTestMust(t, err)
			symbol, err := f.ast.Symbols.Intern("checked-lazy-proof")
			astTestMust(t, err)
			proof, err := f.ast.MkConst(symbol, ruleSort)
			astTestMust(t, err)
			product, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
			astTestMust(t, err)
			astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth, f.falsehood}, proof))
			nullaryFact, err := NewFiniteExplanationProduct(f.relations, nil)
			astTestMust(t, err)
			astTestMust(t, nullaryFact.AddFact(nil, proof))
			assertProductKinds := func(label string, product *FiniteExplanationProduct) {
				t.Helper()
				primary, checker, checked, e := product.CheckedConcreteKinds()
				if e != nil || !checked || primary != pair.primary || checker != pair.checker {
					t.Fatalf("%s pair (%q, %q, checked=%t, %v), want (%q, %q)", label, primary, checker, checked, e, pair.primary, pair.checker)
				}
				checkedIndex := product.table.(*CheckedExplanationIndex)
				assertLazyEngine(label, checkedIndex)
			}
			assertProductKinds("source", product)
			assertProductKinds("nullary fact", nullaryFact)
			if err = f.relations.SetCheckedFiniteFactory(nil); err != nil {
				t.Fatal(err)
			}
			projected, err := product.Project([]int{1})
			astTestMust(t, err)
			assertProductKinds("derived projection", projected)
			if got, found, err := projected.Explanation([]ast.ASTHandle{f.truth}); err != nil || !found || got != proof {
				t.Fatalf("derived proof (%v, found=%t, %v), want %v", got, found, err, proof)
			}
			// Owners must release their LazySparse handles before the shared
			// query engine is closed.
			astTestMust(t, projected.Close())
			astTestMust(t, nullaryFact.Close())
			astTestMust(t, product.Close())
			astTestMust(t, f.relations.SetLazySparseFiniteFactory(nil))
			astTestMust(t, lazy.Close())
			if err = lazy.open(); !errors.Is(err, ErrLazySparse) {
				t.Fatalf("closed shared lazy factory error %v, want ErrLazySparse", err)
			}
		})
	}
}

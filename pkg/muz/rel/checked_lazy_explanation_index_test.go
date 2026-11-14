package rel

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCheckedLazyExplanationIndexAndDerivedProducts(t *testing.T) {
	pairs := []struct{ primary, checker string }{
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

func TestCheckedLazyExplanationHashPairsUseOrdinarySparseFallback(t *testing.T) {
	for _, pair := range []struct{ primary, checker string }{
		{"lazy_sparse", "hashtable"},
		{"hashtable", "lazy_sparse"},
	} {
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
			if factory.CanHandleExplanationIndex(signature, 1) {
				t.Fatal("checked Hash pair claimed a functional explanation index")
			}
			// Package-local adoption keeps otherwise unreachable Hash checker
			// corruption coverage without weakening source plugin selection.
			direct, err := directCheckedExplanationIndex(pair.primary, pair.checker, lazy, signature)
			if err != nil {
				t.Fatal(err)
			}
			gotPrimary, gotChecker, err := direct.ConcreteKinds()
			if err != nil || gotPrimary != pair.primary || gotChecker != pair.checker {
				t.Fatalf("direct pair (%q, %q, %v), want (%q, %q)", gotPrimary, gotChecker, err, pair.primary, pair.checker)
			}
			if err = direct.EnsureFact(Fact{0, 7}); err != nil {
				t.Fatal(err)
			}
			if err = explanationIndexEnsure(direct.checker, Fact{0, 9}, 1); err != nil {
				t.Fatal(err)
			}
			if _, err = direct.Facts(); !errors.Is(err, ErrCheckTable) {
				t.Fatalf("direct suffix mismatch error %v, want ErrCheckTable", err)
			}
			if err = direct.Close(); err != nil {
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
			index, err := f.relations.NewExplanationIndexTable(signature, 1)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := index.(*SparseTable); !ok {
				t.Fatalf("manager functional fallback type %T, want *SparseTable", index)
			}
			if err = index.EnsureFact(Fact{1, 5}); err != nil {
				t.Fatal(err)
			}
			probe := Fact{1, 0}
			if found, err := index.FetchFact(probe); err != nil || !found || probe[1] != 5 {
				t.Fatalf("fallback fetch (%v, found=%t, %v), want suffix 5", probe, found, err)
			}
			if err = index.Close(); err != nil {
				t.Fatal(err)
			}

			product, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := product.table.(*SparseTable); !ok {
				t.Fatalf("fact-product fallback type %T, want *SparseTable", product.table)
			}
			if primary, checker, checked, err := product.CheckedConcreteKinds(); err != nil || checked {
				t.Fatalf("fallback reported checked pair (%q, %q, checked=%t, %v)", primary, checker, checked, err)
			}
			ruleSort, err := f.ast.MkDatalogRuleSort()
			astTestMust(t, err)
			symbol, err := f.ast.Symbols.Intern("hash-fallback-proof")
			astTestMust(t, err)
			proof, err := f.ast.MkConst(symbol, ruleSort)
			astTestMust(t, err)
			proofRef, err := f.ast.NewRef(proof)
			astTestMust(t, err)
			astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth}, proof))
			astTestMust(t, f.ast.DelRef(&proofRef))
			projected, err := product.Project([]int{0})
			astTestMust(t, err)
			if _, ok := projected.table.(*SparseTable); !ok {
				t.Fatalf("derived fallback type %T, want *SparseTable", projected.table)
			}
			if primary, checker, checked, err := projected.CheckedConcreteKinds(); err != nil || checked {
				t.Fatalf("derived fallback reported checked pair (%q, %q, checked=%t, %v)", primary, checker, checked, err)
			}
			if err = product.Close(); err != nil {
				t.Fatal(err)
			}
			if got, found, err := projected.Explanation(nil); err != nil || !found || got != proof {
				t.Fatalf("derived fallback proof (%v, found=%t, %v), want %v", got, found, err, proof)
			}
			if _, err = f.ast.GetKind(proof); err != nil {
				t.Fatalf("derived fallback lost proof after source close: %v", err)
			}
			if err = projected.Close(); err != nil {
				t.Fatal(err)
			}
			if f.ast.IsLiveHandle(proof) {
				t.Fatal("closed derived fallback kept proof alive")
			}
			if err = f.relations.SetLazySparseFiniteFactory(nil); err != nil {
				t.Fatal(err)
			}
			if err = lazy.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

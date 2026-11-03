package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteExplanationFavouriteDerivationsAndOwnership(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f := newTableRelationFixture(t)
			lazy := NewLazySparseFiniteFactory()
			defer func() { astTestMust(t, lazy.Close()) }()
			astTestMust(t, f.relations.SetLazySparseFiniteFactory(lazy))
			astTestMust(t, f.relations.SetDefaultFiniteTable(kind))
			ruleSort, err := f.ast.MkDatalogRuleSort()
			astTestMust(t, err)
			makeProof := func(name string) ast.ASTHandle {
				symbol, e := f.ast.Symbols.Intern(name)
				astTestMust(t, e)
				proof, e := f.ast.MkConst(symbol, ruleSort)
				astTestMust(t, e)
				return proof
			}
			first, later := makeProof("favourite-first"), makeProof("favourite-later")
			source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
			astTestMust(t, err)
			defer func() { astTestMust(t, source.Close()) }()
			checkIndex := func(product *FiniteExplanationProduct) {
				t.Helper()
				switch kind {
				case "hashtable", "sparse":
					// Source Hash rejects functional columns, so its favourite
					// selection falls through to the registered Sparse plugin.
					if _, ok := product.table.(*SparseTable); !ok {
						t.Fatalf("index type %T, want Sparse", product.table)
					}
				case "lazy_sparse":
					index, ok := product.table.(*LazySparseTable)
					if !ok || !lazy.Owns(index) {
						t.Fatalf("index type %T does not belong to original lazy engine", product.table)
					}
				}
			}
			checkIndex(source)
			for _, row := range []struct {
				values []ast.ASTHandle
				proof  ast.ASTHandle
			}{{[]ast.ASTHandle{f.falsehood, f.falsehood}, first}, {[]ast.ASTHandle{f.falsehood, f.truth}, later}, {[]ast.ASTHandle{f.truth, f.truth}, later}} {
				astTestMust(t, source.AddFact(row.values, row.proof))
			}
			nullary, err := NewFiniteExplanationProduct(f.relations, nil)
			astTestMust(t, err)
			defer func() { astTestMust(t, nullary.Close()) }()
			checkIndex(nullary)
			astTestMust(t, nullary.AddFact(nil, first))
			indexSignature, err := nullary.IndexSignature()
			astTestMust(t, err)
			if indexSignature.Len() != 1 {
				t.Fatalf("nullary functional index width %d, want 1", indexSignature.Len())
			}
			nullaryProofFree, err := nullary.ProjectExplanation([]int{0})
			astTestMust(t, err)
			defer func() { astTestMust(t, nullaryProofFree.Close()) }()
			nullaryFinite, err := nullaryProofFree.ToFiniteTable()
			astTestMust(t, err)
			defer func() { astTestMust(t, nullaryFinite.Close()) }()
			if found, e := nullaryFinite.Contains(Fact{}); e != nil || !found {
				t.Fatal("nullary finite conversion", found, e)
			}
			// A later favourite selection must not move existing source owners.
			nextKind := "hashtable"
			if kind == nextKind {
				nextKind = "sparse"
			}
			astTestMust(t, f.relations.SetDefaultFiniteTable(nextKind))
			clone, err := source.Clone()
			astTestMust(t, err)
			defer func() { astTestMust(t, clone.Close()) }()
			projected, err := source.Project([]int{1})
			astTestMust(t, err)
			defer func() { astTestMust(t, projected.Close()) }()
			renamed, err := source.RenameFinite([]int{0, 1})
			astTestMust(t, err)
			defer func() { astTestMust(t, renamed.Close()) }()
			joined, err := projected.Join(projected, []int{0}, []int{0})
			astTestMust(t, err)
			defer func() { astTestMust(t, joined.Close()) }()
			for _, product := range []*FiniteExplanationProduct{clone, projected, renamed, joined} {
				checkIndex(product)
			}
			proof, found, err := projected.Explanation([]ast.ASTHandle{f.falsehood})
			astTestMust(t, err)
			if !found || proof != first {
				t.Fatal("projection collision replaced the first proof")
			}
			proof, found, err = renamed.Explanation([]ast.ASTHandle{f.truth, f.falsehood})
			astTestMust(t, err)
			if !found || proof != later {
				t.Fatal("finite rename lost its matching proof")
			}
			row, found, err := joined.ExplanationRow([]ast.ASTHandle{f.falsehood, f.falsehood})
			astTestMust(t, err)
			if !found || len(row) != 2 || row[0] != first || row[1] != first {
				t.Fatal("join did not preserve ordered child proofs", row)
			}
			proofFree, err := projected.ProjectExplanation([]int{0})
			astTestMust(t, err)
			defer func() { astTestMust(t, proofFree.Close()) }()
			checkIndex(proofFree)
			finite, err := proofFree.ToFiniteTable()
			astTestMust(t, err)
			defer func() { astTestMust(t, finite.Close()) }()
			if count, e := finite.Len(); e != nil || count != 2 {
				t.Fatal("converted finite rows", count, e)
			}
			// A Hash-involving checked favourite rejects the functional index,
			// so manager registration falls through to an unchecked Sparse table.
			incapable, err := NewFiniteCheckedFactory("hashtable", "sparse")
			astTestMust(t, err)
			astTestMust(t, f.relations.SetCheckedFiniteFactory(incapable))
			fallback, err := source.Clone()
			astTestMust(t, err)
			defer func() { astTestMust(t, fallback.Close()) }()
			if _, ok := fallback.table.(*SparseTable); !ok {
				t.Fatalf("functional fallback index %T, want ordinary Sparse", fallback.table)
			}
			// A capable checked favourite lifts a plain index without moving source.
			factory, err := NewFiniteCheckedFactoryWithLazy("sparse", "lazy_sparse", lazy)
			astTestMust(t, err)
			astTestMust(t, f.relations.SetCheckedFiniteFactory(factory))
			checked, err := source.Clone()
			astTestMust(t, err)
			defer func() { astTestMust(t, checked.Close()) }()
			primary, checker, isChecked, err := checked.CheckedConcreteKinds()
			astTestMust(t, err)
			if !isChecked || primary != "sparse" || checker != "lazy_sparse" {
				t.Fatal("plain source did not enter active checked query", primary, checker, isChecked)
			}
			checkIndex(source)
			astTestMust(t, f.relations.SetCheckedFiniteFactory(nil))
			astTestMust(t, source.Close())
			astTestMust(t, projected.Close())
			astTestMust(t, renamed.Close())
			astTestMust(t, joined.Close())
			astTestMust(t, checked.Close())
			proof, found, err = clone.Explanation([]ast.ASTHandle{f.falsehood, f.falsehood})
			astTestMust(t, err)
			if !found || proof != first {
				t.Fatal("clone proof did not survive the source owners")
			}
			astTestMust(t, clone.FilterFiniteEqual(0, f.truth))
			if count, e := clone.Len(); e != nil || count != 1 {
				t.Fatal("functional-index filter", count, e)
			}
			if count, e := finite.Len(); e != nil || count != 2 {
				t.Fatal("independent finite conversion changed", count, e)
			}
		})
	}
}

package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func fillRelationExplanationProduct(t *testing.T, product *RelationExplanationProduct, fact Fact, proof ast.ASTHandle) {
	t.Helper()
	_, err := product.table.Add(fact)
	must(t, err)
	must(t, product.explanation.AddFact([]ast.ASTHandle{proof}))
}

func TestRelationExplanationProductPreservesSparseSelectedBacking(t *testing.T) {
	f := newTableRelationFixture(t)
	signature, err := NewSignature([]uint64{2})
	must(t, err)
	source, err := NewSparseTable(signature, 0)
	must(t, err)
	_, err = source.Add(Fact{1})
	must(t, err)
	proof := relationExplanationProof(t, f.ast, "relation-sparse-selected-proof")
	product, err := newRelationExplanationProductFromFinite(f.relations, []ast.ASTHandle{f.boolean}, source, proof)
	must(t, err)
	defer func() { must(t, product.Close()); must(t, source.Close()) }()
	if _, ok := product.table.(*SparseTable); !ok {
		t.Fatalf("sparse source became %T", product.table)
	}
	must(t, source.Reset())
	if rows, err := product.Len(); err != nil || rows != 1 {
		t.Fatal("sparse snapshot aliases source", rows, err)
	}
	clone, err := product.Clone()
	must(t, err)
	defer func() { must(t, clone.Close()) }()
	if _, ok := clone.table.(*SparseTable); !ok {
		t.Fatalf("sparse clone became %T", clone.table)
	}
	projected, err := product.Project(nil)
	must(t, err)
	defer func() { must(t, projected.Close()) }()
	if _, ok := projected.table.(*SparseTable); !ok {
		t.Fatalf("sparse projection became %T", projected.table)
	}
	renamed, err := product.RenameFinite(nil)
	must(t, err)
	defer func() { must(t, renamed.Close()) }()
	if _, ok := renamed.table.(*SparseTable); !ok {
		t.Fatalf("sparse rename became %T", renamed.table)
	}
	emptyLike, err := product.EmptyLike()
	must(t, err)
	defer func() { must(t, emptyLike.Close()) }()
	if _, ok := emptyLike.table.(*SparseTable); !ok {
		t.Fatalf("sparse empty product became %T", emptyLike.table)
	}
}

func TestRelationExplanationProductCheckedFiniteDispatch(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewFiniteCheckedFactory("sparse", "hashtable")
	must(t, err)
	must(t, f.relations.SetCheckedFiniteFactory(factory))
	t.Cleanup(func() { must(t, f.relations.SetCheckedFiniteFactory(nil)) })
	a := relationExplanationProof(t, f.ast, "relation-checked-a")
	b := relationExplanationProof(t, f.ast, "relation-checked-b")
	d := relationExplanationProof(t, f.ast, "relation-checked-d")
	makeProduct := func(fact Fact, proof ast.ASTHandle) *RelationExplanationProduct {
		product, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
		must(t, err)
		fillRelationExplanationProduct(t, product, fact, proof)
		return product
	}
	target, source, delta := makeProduct(Fact{0}, a), makeProduct(Fact{1}, b), makeProduct(Fact{0}, d)
	defer func() { must(t, delta.Close()); must(t, source.Close()); must(t, target.Close()) }()
	primary, checker, checked, err := source.CheckedConcreteKinds()
	if err != nil || !checked || primary != "sparse" || checker != "hashtable" {
		t.Fatal("checked concrete kinds", primary, checker, checked, err)
	}
	joined, err := source.Join(source, []int{0}, []int{0})
	must(t, err)
	defer func() { must(t, joined.Close()) }()
	if _, _, checked, err = joined.CheckedConcreteKinds(); err != nil || !checked {
		t.Fatal("checked join lost checker", checked, err)
	}
	projected, err := source.Project(nil)
	must(t, err)
	defer func() { must(t, projected.Close()) }()
	if _, _, checked, err = projected.CheckedConcreteKinds(); err != nil || !checked {
		t.Fatal("checked projection lost checker", checked, err)
	}
	zeroProof, err := source.ProjectColumns([]int{1})
	must(t, err)
	defer func() { must(t, zeroProof.Close()) }()
	if _, _, checked, err = zeroProof.CheckedConcreteKinds(); err != nil || !checked {
		t.Fatal("checked zero-proof projection lost checker", checked, err)
	}
	proofRow, err := zeroProof.explanation.Data()
	if err != nil || len(proofRow) != 0 {
		t.Fatal("checked zero-proof identity", proofRow, err)
	}
	renamed, err := source.RenameFinite(nil)
	must(t, err)
	defer func() { must(t, renamed.Close()) }()
	if _, _, checked, err = renamed.CheckedConcreteKinds(); err != nil || !checked {
		t.Fatal("checked rename lost checker", checked, err)
	}

	variable, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, variable)
	guard, err := f.ast.MkEq(variable, f.truth)
	astTestMust(t, err)
	astTestPin(t, f.ast, guard)
	must(t, source.FilterInterpreted(tableRelationTheoryRewriter(t, f.ast), []ast.ASTHandle{guard}, map[uint32]int{0: 0}, map[uint32]ast.ASTHandle{0: f.boolean}))
	if rows, err := source.Len(); err != nil || rows != 1 {
		t.Fatal("checked interpreted filter", rows, err)
	}
	must(t, target.Union(source, delta))
	if rows, err := target.Len(); err != nil || rows != 2 {
		t.Fatal("checked union target", rows, err)
	}
	if rows, err := delta.Len(); err != nil || rows != 2 {
		t.Fatal("checked union delta", rows, err)
	}
	proof, found, err := target.Explanation([]ast.ASTHandle{f.truth})
	if err != nil || !found {
		t.Fatal("checked relation proof", found, err)
	}
	requireRelationUnionProof(t, f.ast, proof, a, b)

	// A checked query still preserves an explicitly selected source primary.
	override, err := NewFiniteCheckedFactory("hashtable", "hashtable")
	must(t, err)
	must(t, f.relations.SetCheckedFiniteFactory(override))
	signature, err := NewSignature([]uint64{2})
	must(t, err)
	sparseSource, err := NewSparseTable(signature, 0)
	must(t, err)
	defer func() { must(t, sparseSource.Close()) }()
	_, err = sparseSource.Add(Fact{1})
	must(t, err)
	preserved, err := newRelationExplanationProductFromFinite(f.relations, []ast.ASTHandle{f.boolean}, sparseSource, b)
	must(t, err)
	defer func() { must(t, preserved.Close()) }()
	primary, checker, checked, err = preserved.CheckedConcreteKinds()
	if err != nil || !checked || primary != "sparse" || checker != "hashtable" {
		t.Fatal("checked source primary not preserved", primary, checker, checked, err)
	}
	lazyFactory := NewLazySparseFiniteFactory()
	lazySource, err := lazyFactory.New(signature)
	must(t, err)
	if product, err := newRelationExplanationProductFromFinite(f.relations, []ast.ASTHandle{f.boolean}, lazySource, b); err == nil || product != nil {
		t.Fatal("unsupported checked lazy source silently changed kind", product, err)
	}
	must(t, lazySource.Close())
	must(t, lazyFactory.Close())
}

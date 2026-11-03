package rel

import "testing"

func TestExplanationIndexUsesConcreteManagerFavourite(t *testing.T) {
	f := newTableRelationFixture(t)
	signature, err := NewSignature([]uint64{4, 16})
	if err != nil {
		t.Fatal(err)
	}

	hashFallback, err := f.relations.NewExplanationIndexTable(signature, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := hashFallback.(*SparseTable); !ok {
		t.Fatalf("Hash favourite fallback %T, want Sparse", hashFallback)
	}
	if err = hashFallback.EnsureFact(Fact{1, 3}); err != nil {
		t.Fatal(err)
	}
	if err = hashFallback.EnsureFact(Fact{1, 7}); err != nil {
		t.Fatal(err)
	}
	probe := Fact{1, 0}
	if found, e := hashFallback.FetchFact(probe); e != nil || !found || probe[1] != 7 {
		t.Fatal("Hash fallback functional update", probe, found, e)
	}
	if rows, e := hashFallback.Len(); e != nil || rows != 1 {
		t.Fatal("Hash fallback row count", rows, e)
	}
	if found, e := hashFallback.FetchFact(Fact{4, 0}); e == nil || found {
		t.Fatal("fallback out-of-domain fetch", found, e)
	}
	if err = hashFallback.Close(); err != nil {
		t.Fatal(err)
	}

	if err = f.relations.SetDefaultFiniteTable("sparse"); err != nil {
		t.Fatal(err)
	}
	sparse, err := f.relations.NewExplanationIndexTable(signature, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sparse.(*SparseTable); !ok {
		t.Fatalf("sparse explanation index %T", sparse)
	}
	if err = sparse.Close(); err != nil {
		t.Fatal(err)
	}

	lazyFactory := NewLazySparseFiniteFactory()
	if err = f.relations.SetLazySparseFiniteFactory(lazyFactory); err != nil {
		t.Fatal(err)
	}
	if err = f.relations.SetDefaultFiniteTable("lazy_sparse"); err != nil {
		t.Fatal(err)
	}
	lazy, err := f.relations.NewExplanationIndexTable(signature, 1)
	if err != nil {
		t.Fatal(err)
	}
	lazyTable, ok := lazy.(*LazySparseTable)
	if !ok || !lazyFactory.Owns(lazyTable) {
		t.Fatalf("lazy explanation index %T ownership=%t", lazy, lazyFactory.Owns(lazyTable))
	}
	if err = lazy.EnsureFact(Fact{2, 4}); err != nil {
		t.Fatal(err)
	}
	if err = lazy.EnsureFact(Fact{2, 9}); err != nil {
		t.Fatal(err)
	}
	probe = Fact{2, 0}
	if found, e := lazy.FetchFact(probe); e != nil || !found || probe[1] != 9 {
		t.Fatal("lazy functional update", probe, found, e)
	}
	if err = lazy.Close(); err != nil {
		t.Fatal(err)
	}
	if err = lazyFactory.Close(); err != nil {
		t.Fatal(err)
	}
}

package rel

import "testing"

func TestLazySparseBulkRemovalAndSizeQueriesFollowLazySource(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	base, err := factory.New(lazySignature(t, 8))
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{1}, {2}, {3}} {
		if _, err = base.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	deferred, err := base.RenameCycle(nil)
	if err != nil {
		t.Fatal(err)
	}
	if known, err := deferred.KnowsExactSize(); err != nil || known {
		t.Fatal("lazy size knowledge", known, err)
	}
	if deferred.Forced() {
		t.Fatal("size metadata forced deferred root")
	}
	if empty, err := deferred.Empty(); err != nil || empty {
		t.Fatal("nonempty deferred root", empty, err)
	}
	if !deferred.Forced() {
		t.Fatal("empty query did not force deferred root")
	}
	if err := deferred.RemoveFacts([]Fact{{1}, {1}, {3}}); err != nil {
		t.Fatal(err)
	}
	if facts, err := deferred.Facts(); err != nil || len(facts) != 1 || facts[0][0] != 2 {
		t.Fatal("bulk removal result", facts, err)
	}
	if facts, err := base.Facts(); err != nil || len(facts) != 3 {
		t.Fatal("derived mutation changed shared source", facts, err)
	}
}

func TestLazySparseNullaryBulkRemovalUsesConcreteFallback(t *testing.T) {
	engine := NewLazySparseEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	signature, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	truth := NewHashTable(signature)
	if _, err = truth.Add(Fact{}); err != nil {
		t.Fatal(err)
	}
	table := engine.adopt(&lazySparseNode{kind: LazyTableBase, signature: signature, nullary: truth})
	if err = table.RemoveFacts([]Fact{{}}); err != nil {
		t.Fatal(err)
	}
	if empty, err := table.Empty(); err != nil || !empty {
		t.Fatal("nullary removal", empty, err)
	}
}

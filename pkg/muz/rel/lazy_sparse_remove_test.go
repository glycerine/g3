package rel

import "testing"

func TestLazySparseRemoveFactSupportsNullaryConcreteFallback(t *testing.T) {
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
	concrete := NewHashTable(signature)
	if _, err = concrete.Add(Fact{}); err != nil {
		t.Fatal(err)
	}
	table := engine.adopt(&lazySparseNode{kind: LazyTableBase, signature: signature, nullary: concrete})
	if removed, err := table.Remove(Fact{}); err != nil || !removed {
		t.Fatal("remove nullary truth", removed, err)
	}
	if removed, err := table.Remove(Fact{}); err != nil || removed {
		t.Fatal("repeat nullary removal", removed, err)
	}
	if empty, err := table.Empty(); err != nil || !empty {
		t.Fatal("nullary removal result", empty, err)
	}
}

func TestLazySparseRemoveFactForcesDeferredSparseOwnerOnly(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	base, err := factory.New(lazySignature(t, 4))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = base.Add(Fact{2}); err != nil {
		t.Fatal(err)
	}
	deferred, err := base.RenameCycle(nil)
	if err != nil {
		t.Fatal(err)
	}
	if removed, err := deferred.Remove(Fact{2}); err != nil || !removed {
		t.Fatal("remove deferred row", removed, err)
	}
	if !deferred.Forced() {
		t.Fatal("remove did not force deferred owner")
	}
	if found, err := base.Contains(Fact{2}); err != nil || !found {
		t.Fatal("derived removal changed source owner", found, err)
	}
}

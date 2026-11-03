package rel

import "testing"

func TestLazySparseClonePreservesFunctionalOwnerAndIndependence(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	base, err := factory.NewFunctional(lazySignature(t, 3, 8), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = base.Add(Fact{1, 4}); err != nil {
		t.Fatal(err)
	}
	deferred, err := base.RenameCycle(nil)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := deferred.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if !deferred.Forced() || !clone.Forced() {
		t.Fatal("clone did not force source and materialize result")
	}
	if functional, err := clone.FunctionalColumns(); err != nil || functional != 1 {
		t.Fatal("functional metadata", functional, err)
	}
	if err = clone.EnsureFact(Fact{1, 7}); err != nil {
		t.Fatal(err)
	}
	probe := Fact{1, 0}
	if found, err := deferred.FetchFact(probe); err != nil || !found || probe[1] != 4 {
		t.Fatal("clone mutation changed source owner", found, probe, err)
	}
}

func TestLazySparseClonePreservesNullaryFallbackOwner(t *testing.T) {
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
	source := engine.adopt(&lazySparseNode{kind: LazyTableBase, signature: signature, nullary: concrete})
	clone, err := source.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if err = clone.RemoveFacts([]Fact{{}}); err != nil {
		t.Fatal(err)
	}
	if empty, err := clone.Empty(); err != nil || !empty {
		t.Fatal("clone nullary mutation", empty, err)
	}
	if empty, err := source.Empty(); err != nil || empty {
		t.Fatal("nullary clone changed source owner", empty, err)
	}
}

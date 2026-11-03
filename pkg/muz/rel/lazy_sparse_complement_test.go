package rel

import "testing"

func TestLazySparseComplementForcesDeferredUnaryAndOwnsResult(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	base, err := factory.New(lazySignature(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = base.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	deferred, err := base.RenameCycle(nil)
	if err != nil {
		t.Fatal(err)
	}
	if deferred.Forced() {
		t.Fatal("rename forced eagerly")
	}
	complement, err := deferred.ComplementFunctional(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !deferred.Forced() || !complement.Forced() {
		t.Fatal("complement did not materialize concrete source and result")
	}
	for value, want := range []bool{true, false, true} {
		found, err := complement.Contains(Fact{Element(value)})
		if err != nil || found != want {
			t.Fatal("complement membership", value, found, want, err)
		}
	}
	if _, err = complement.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	if found, err := deferred.Contains(Fact{1}); err != nil || !found {
		t.Fatal("complement result aliased source", found, err)
	}
}

func TestLazySparseComplementPreservesFunctionalSuffixAndNullaryFallback(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	functional, err := factory.NewFunctional(lazySignature(t, 2, 8), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = functional.Add(Fact{0, 7}); err != nil {
		t.Fatal(err)
	}
	full, err := functional.ComplementFunctional(Fact{7})
	if err != nil {
		t.Fatal(err)
	}
	if found, err := full.Contains(Fact{0, 7}); err != nil || found {
		t.Fatal("functional complement included occupied key", found, err)
	}
	if found, err := full.Contains(Fact{1, 7}); err != nil || !found {
		t.Fatal("functional complement omitted free key", found, err)
	}

	engine := factory.engine
	nullarySignature, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	empty := NewHashTable(nullarySignature)
	nullary := engine.adopt(&lazySparseNode{kind: LazyTableBase, signature: nullarySignature, nullary: empty})
	truth, err := nullary.ComplementFunctional(nil)
	if err != nil {
		t.Fatal(err)
	}
	if found, err := truth.Contains(Fact{}); err != nil || !found {
		t.Fatal("nullary complement", found, err)
	}
}

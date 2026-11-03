package rel

import (
	"errors"
	"testing"
)

func TestFiniteCheckedFactoryLazyConcreteAndNullaryCapabilities(t *testing.T) {
	signature, err := NewSignature([]uint64{4})
	if err != nil {
		t.Fatal(err)
	}
	nullary, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	lazy := NewLazySparseFiniteFactory()
	for _, kinds := range [][2]string{{"lazy_sparse", "hashtable"}, {"sparse", "lazy_sparse"}, {"lazy_sparse", "lazy_sparse"}} {
		factory, e := NewFiniteCheckedFactoryWithLazy(kinds[0], kinds[1], lazy)
		if e != nil {
			t.Fatal(kinds, e)
		}
		if !factory.CanHandleSignature(signature) || !factory.AllowsUncheckedNullaryFallback() {
			t.Fatal("capability", kinds)
		}
		pair, e := factory.New(signature)
		if e != nil {
			t.Fatal(kinds, e)
		}
		primary, checker, e := pair.ConcreteKinds()
		if e != nil || primary != kinds[0] || checker != kinds[1] {
			t.Fatalf("kinds %v = %q/%q (%v)", kinds, primary, checker, e)
		}
		if _, e = pair.Add(Fact{2}); e != nil {
			t.Fatal(kinds, e)
		}
		plain := NewHashTable(signature)
		if _, e = plain.Add(Fact{3}); e != nil {
			t.Fatal(e)
		}
		lifted, e := pair.Lift(plain)
		if e != nil {
			t.Fatal(kinds, e)
		}
		liftedPrimary, liftedChecker, e := lifted.ConcreteKinds()
		if e != nil || liftedPrimary != kinds[0] || liftedChecker != kinds[1] {
			t.Fatalf("lifted kinds %v = %q/%q (%v)", kinds, liftedPrimary, liftedChecker, e)
		}
		if e = errors.Join(lifted.Close(), plain.Close(), pair.Close()); e != nil {
			t.Fatal(kinds, e)
		}
		if _, e = factory.New(nullary); !errors.Is(e, ErrCheckTable) {
			t.Fatalf("nullary pair %v = %v", kinds, e)
		}
	}
	seed := NewHashTable(signature)
	if _, err = seed.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	factory, err := NewFiniteCheckedFactoryWithLazy("hashtable", "lazy_sparse", lazy)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := NewFiniteCheckedPairFromFactory(seed, factory)
	if err != nil {
		t.Fatal(err)
	}
	if primary, checker, e := pair.ConcreteKinds(); e != nil || primary != "hashtable" || checker != "lazy_sparse" {
		t.Fatal(primary, checker, e)
	}
	if err = errors.Join(pair.Close(), seed.Close()); err != nil {
		t.Fatal(err)
	}
	if err = lazy.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFiniteCheckedFactoryHashPairHandlesNullary(t *testing.T) {
	nullary, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	factory, err := NewFiniteCheckedFactory("hashtable", "hashtable")
	if err != nil {
		t.Fatal(err)
	}
	if !factory.CanHandleSignature(nullary) || factory.AllowsUncheckedNullaryFallback() {
		t.Fatal("hash/hash nullary capability")
	}
	pair, err := factory.New(nullary)
	if err != nil {
		t.Fatal(err)
	}
	if primary, checker, e := pair.ConcreteKinds(); e != nil || primary != "hashtable" || checker != "hashtable" {
		t.Fatal(primary, checker, e)
	}
	if err = pair.Close(); err != nil {
		t.Fatal(err)
	}
}

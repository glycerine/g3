package rel

import "testing"

func TestLazySparseOrdinaryComplementUsesSourceDefaultBoundary(t *testing.T) {
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
	complement, err := deferred.Complement()
	if err != nil {
		t.Fatal(err)
	}
	for value, want := range []bool{true, false, true} {
		found, err := complement.Contains(Fact{Element(value)})
		if err != nil || found != want {
			t.Fatal("ordinary complement", value, found, want, err)
		}
	}
	if found, err := base.Contains(Fact{1}); err != nil || !found {
		t.Fatal("complement changed source owner", found, err)
	}
}

func TestLazySparseOrdinaryComplementRequiresFunctionalSuffix(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() { _ = factory.Close() })
	functional, err := factory.NewFunctional(lazySignature(t, 2, 4), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = functional.Add(Fact{1, 2}); err != nil {
		t.Fatal(err)
	}
	if complement, err := functional.Complement(); err == nil || complement != nil {
		t.Fatal("functional complement accepted missing suffix", complement, err)
	}
	if found, err := functional.Contains(Fact{1, 2}); err != nil || !found {
		t.Fatal("rejected complement changed source owner", found, err)
	}
}

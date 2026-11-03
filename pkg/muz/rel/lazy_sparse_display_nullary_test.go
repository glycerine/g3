package rel

import (
	"bytes"
	"testing"
)

func TestLazySparseDisplaySupportsProjectedNullaryConcreteFallback(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	base, err := factory.New(lazySignature(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = base.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	truth, err := base.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	if truth.Forced() {
		t.Fatal("nullary projection forced eagerly")
	}
	var output bytes.Buffer
	if err = truth.Display(&output); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "table with signature ():\n()\n\n"; got != want {
		t.Fatalf("nullary display got %q, want %q", got, want)
	}
	if !truth.Forced() {
		t.Fatal("display did not force nullary fallback")
	}
	if found, err := base.Contains(Fact{1}); err != nil || !found {
		t.Fatal("display changed projected source owner", found, err)
	}
}

func TestLazySparseDisplaySupportsEmptyNullaryConcreteFallback(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() { _ = factory.Close() })
	base, err := factory.New(lazySignature(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	falsity, err := base.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = falsity.Display(&output); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "table with signature ():\n\n"; got != want {
		t.Fatalf("empty nullary display got %q, want %q", got, want)
	}
}

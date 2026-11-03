package rel

import "testing"

func TestLazySparseKindTracksDeferredSourceNodeWithoutForcing(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	left, err := factory.New(lazySignature(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	right, err := factory.New(lazySignature(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	if left.Kind() != LazyTableBase {
		t.Fatal("base kind", left.Kind())
	}
	joined, err := left.Join(right, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if joined.Kind() != LazyTableJoin || joined.Forced() {
		t.Fatal("join kind forced or changed", joined.Kind(), joined.Forced())
	}
	projected, err := joined.Project([]int{1})
	if err != nil {
		t.Fatal(err)
	}
	if projected.Kind() != LazyTableProject || projected.Forced() {
		t.Fatal("project kind forced or changed", projected.Kind(), projected.Forced())
	}
	if _, err = projected.Facts(); err != nil {
		t.Fatal(err)
	}
	if projected.Kind() != LazyTableProject || !projected.Forced() {
		t.Fatal("forcing replaced deferred node identity", projected.Kind(), projected.Forced())
	}
}

func TestLazySparseKindCoversFunctionalAndNullaryBaseOwners(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() { _ = factory.Close() })
	functional, err := factory.NewFunctional(lazySignature(t, 2, 4), 1)
	if err != nil || functional.Kind() != LazyTableBase {
		t.Fatal("functional base kind", functional, err)
	}
	signature, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	nullary := factory.engine.adopt(&lazySparseNode{kind: LazyTableBase, signature: signature, nullary: NewHashTable(signature)})
	if nullary.Kind() != LazyTableBase {
		t.Fatal("nullary base kind", nullary.Kind())
	}
	if err = nullary.Close(); err != nil {
		t.Fatal(err)
	}
	if nullary.Kind() != LazyTableBase {
		t.Fatal("closed handle kind", nullary.Kind())
	}
}

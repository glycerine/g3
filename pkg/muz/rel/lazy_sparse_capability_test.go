package rel

import "testing"

func TestLazySparseCapabilityDelegatesSparseAndManagerFallsBackForNullary(t *testing.T) {
	f := newTableRelationFixture(t)
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	nullary, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	unary := lazySignature(t, 2)
	if factory.CanHandleSignature(nullary) || !factory.CanHandleSignature(unary) {
		t.Fatal("lazy sparse capability did not match concrete sparse plugin")
	}
	if table, err := factory.New(nullary); err == nil || table != nil {
		t.Fatal("lazy sparse factory constructed an unsupported nullary table", table, err)
	}
	if err = f.relations.SetLazySparseFiniteFactory(factory); err != nil {
		t.Fatal(err)
	}
	if err = f.relations.SetDefaultFiniteTable("lazy_sparse"); err != nil {
		t.Fatal(err)
	}
	truth, err := f.relations.NewDefaultFiniteTable(nullary)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = truth.Close() })
	if _, ok := truth.(*HashTable); !ok {
		t.Fatalf("nullary default used %T, want Hash fallback", truth)
	}
	finite, err := f.relations.NewDefaultFiniteTable(unary)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = finite.Close() })
	if lazy, ok := finite.(*LazySparseTable); !ok || !factory.Owns(lazy) {
		t.Fatalf("nonempty default used %T outside selected lazy factory", finite)
	}
}

func TestLazySparseCapabilityClosesWithFactoryLifecycle(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	signature := lazySignature(t, 2)
	if !factory.CanHandleSignature(signature) {
		t.Fatal("open factory declined sparse signature")
	}
	if err := factory.Close(); err != nil {
		t.Fatal(err)
	}
	if factory.CanHandleSignature(signature) {
		t.Fatal("closed factory advertised constructible signature")
	}
	if table, err := factory.New(signature); err == nil || table != nil {
		t.Fatal("closed incapable factory constructed table", table, err)
	}
}

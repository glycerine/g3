package rel

import "testing"

func TestRelationManagerCheckedFactoryPrecedesLazyDefaultForTemporaries(t *testing.T) {
	f := newTableRelationFixture(t)
	lazy := NewLazySparseFiniteFactory()
	t.Cleanup(func() { _ = lazy.Close() })
	if err := f.relations.SetLazySparseFiniteFactory(lazy); err != nil {
		t.Fatal(err)
	}
	if err := f.relations.SetDefaultFiniteTable("lazy_sparse"); err != nil {
		t.Fatal(err)
	}
	checked, err := NewFiniteCheckedFactory("sparse", "hashtable")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.relations.SetDefaultCheckedFactory(checked, true); err != nil {
		t.Fatal(err)
	}
	signature, err := NewSignature([]uint64{2})
	if err != nil {
		t.Fatal(err)
	}
	table, err := f.relations.NewDefaultFiniteTable(signature)
	if err != nil {
		t.Fatal(err)
	}
	pair, ok := table.(*FiniteCheckedPair)
	if !ok {
		t.Fatalf("checked temporary type %T", table)
	}
	primary, checker, err := pair.ConcreteKinds()
	if err != nil || primary != "sparse" || checker != "hashtable" {
		t.Fatal("checked temporary kinds", primary, checker, err)
	}
	if err = pair.Close(); err != nil {
		t.Fatal(err)
	}
	nullary, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	table, err = f.relations.NewDefaultFiniteTable(nullary)
	if err != nil {
		t.Fatal(err)
	}
	if _, checked := table.(*FiniteCheckedPair); checked {
		t.Fatal("source nullary fallback remained checked")
	}
	if _, hash := table.(*HashTable); !hash {
		t.Fatalf("nullary fallback type %T, want hash", table)
	}
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	total := NewTotalInstruction(signature, 0)
	t.Cleanup(func() { _ = total.Close() })
	complete, err := total.Perform(context)
	if err != nil || !complete {
		t.Fatal("checked compiler temporary", complete, err)
	}
	if temporary, err := context.FiniteRegister(0); err != nil {
		t.Fatal(err)
	} else if _, ok := temporary.(*FiniteCheckedPair); !ok {
		t.Fatalf("compiler temporary type %T", temporary)
	}
	if err = f.relations.SetDefaultCheckedFactory(nil, false); err != nil {
		t.Fatal(err)
	}
	table, err = f.relations.NewDefaultFiniteTable(signature)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := table.(*LazySparseTable); !ok || !lazy.Owns(table) {
		t.Fatalf("clearing checked scope did not restore lazy default: %T", table)
	}
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
}

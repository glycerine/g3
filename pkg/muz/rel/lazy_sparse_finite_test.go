package rel

import "testing"

func TestLazySparseFiniteTableBoundaryPreservesConcreteOwner(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	base, err := engine.New(lazySignature(t, 3), 0)
	if err != nil {
		t.Fatal(err)
	}
	var table FiniteTable = base
	for _, fact := range []Fact{{0}, {2}} {
		if added, addErr := table.Add(fact); addErr != nil || !added {
			t.Fatal("add", fact, added, addErr)
		}
	}
	if signature, signatureErr := table.Signature(); signatureErr != nil || !signature.equal(lazySignature(t, 3)) {
		t.Fatal("signature", signature.Domains(), signatureErr)
	}
	if found, containsErr := table.Contains(Fact{2}); containsErr != nil || !found {
		t.Fatal("contains", found, containsErr)
	}
	clone, err := table.CloneFinite()
	if err != nil {
		t.Fatal(err)
	}
	lazyClone, ok := clone.(*LazySparseTable)
	if !ok || lazyClone.root.evaluated == nil || lazyClone.root.evaluated.functional != 0 {
		t.Fatal("finite clone changed concrete lazy-sparse representation")
	}
	if err = table.Reset(); err != nil {
		t.Fatal(err)
	}
	if length, lengthErr := table.Len(); lengthErr != nil || length != 0 {
		t.Fatal("reset", length, lengthErr)
	}
	if facts, factsErr := clone.Facts(); factsErr != nil || len(facts) != 2 {
		t.Fatal("clone independence", facts, factsErr)
	}
	if err = clone.Close(); err != nil {
		t.Fatal(err)
	}
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
}

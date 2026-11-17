package rel

import (
	"testing"
)

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

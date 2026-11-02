package rel

import (
	"errors"
	"testing"
)

func TestCheckedRelationFromTableCopiesSparseAndPrimaryOwners(t *testing.T) {
	signature, err := NewSignature([]uint64{3})
	must(t, err)
	source := NewHashTable(signature)
	defer func() { must(t, source.Close()) }()
	_, err = source.Add(Fact{1})
	must(t, err)
	checked, err := NewCheckedRelationFromTable(source)
	must(t, err)
	primary, err := checked.Primary()
	must(t, err)
	if primary == source {
		t.Fatal("checked relation borrowed source table")
	}
	_, err = source.Add(Fact{2})
	must(t, err)
	if found, err := checked.Contains(Fact{2}); err != nil || found {
		t.Fatal("source mutation changed checked copy", found, err)
	}
	if found, err := checked.Contains(Fact{1}); err != nil || !found {
		t.Fatal("seed fact missing from paired backends", found, err)
	}
	must(t, checked.Close())
	if _, err = primary.Len(); !errors.Is(err, ErrTable) {
		t.Fatal("checked close left primary open", err)
	}
	if rows, err := source.Len(); err != nil || rows != 2 {
		t.Fatal("checked close changed source", rows, err)
	}
}

package rel

import (
	"errors"
	"testing"
)

func TestSieveSizeEstimatesForwardInnerStorage(t *testing.T) {
	signature := sieveSignature(t, 2, 17, 3)
	relation, err := NewSieveRelation(signature, []bool{true, false, true})
	if err != nil {
		t.Fatal(err)
	}
	if rows, sizeErr := relation.SizeEstimateRows(); sizeErr != nil || rows != 0 {
		t.Fatal("empty row estimate", rows, sizeErr)
	}
	if bytes, sizeErr := relation.SizeEstimateBytes(); sizeErr != nil || bytes != 0 {
		t.Fatal("empty byte estimate", bytes, sizeErr)
	}
	for _, fact := range []Fact{{0, 0, 0}, {1, 16, 2}, {1, 0, 2}} {
		if _, err = relation.AddFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := relation.SizeEstimateRows()
	if err != nil || rows != 2 {
		t.Fatal("ignored-coordinate duplicates must not add rows", rows, err)
	}
	innerBytes, err := relation.inner.SizeEstimateBytes()
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := relation.SizeEstimateBytes()
	if err != nil || bytes != innerBytes {
		t.Fatal("byte estimate did not forward inner storage", bytes, innerBytes, err)
	}
	if err = relation.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = relation.SizeEstimateRows(); !errors.Is(err, ErrSieve) {
		t.Fatal("closed row estimate", err)
	}
	if _, err = relation.SizeEstimateBytes(); !errors.Is(err, ErrSieve) {
		t.Fatal("closed byte estimate", err)
	}
}

func TestFullSieveSizeEstimateDoesNotExpandIgnoredDomains(t *testing.T) {
	full, err := NewFullSieve(sieveSignature(t, 1024, 2048))
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	rows, err := full.SizeEstimateRows()
	if err != nil || rows != 1 {
		t.Fatal("full sieve inner row", rows, err)
	}
	bytes, err := full.SizeEstimateBytes()
	if err != nil || bytes != 0 {
		t.Fatal("zero-column inner payload", bytes, err)
	}
}

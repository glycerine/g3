package rel

import (
	"errors"
	"testing"
)

func TestSparseTableEmptySourceLifecycleAndComplementConsumer(t *testing.T) {
	signature, err := NewSignature([]uint64{8})
	if err != nil {
		t.Fatal(err)
	}
	table, err := NewSparseTable(signature, 1)
	if err != nil {
		t.Fatal(err)
	}
	if empty, err := table.Empty(); err != nil || !empty {
		t.Fatalf("new table empty=%v err=%v", empty, err)
	}
	complement, err := table.ComplementFunctional(Fact{4})
	if err != nil {
		t.Fatal(err)
	}
	if empty, err := complement.Empty(); err != nil || empty {
		t.Fatalf("empty-source complement empty=%v err=%v", empty, err)
	}
	if exact, err := complement.ContainsExact(Fact{4}); err != nil || !exact {
		t.Fatalf("empty-source complement fact=%v err=%v", exact, err)
	}
	if err := complement.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := table.Add(Fact{2}); err != nil {
		t.Fatal(err)
	}
	if empty, err := table.Empty(); err != nil || empty {
		t.Fatalf("populated table empty=%v err=%v", empty, err)
	}
	complement, err = table.ComplementFunctional(Fact{7})
	if err != nil {
		t.Fatal(err)
	}
	if empty, err := complement.Empty(); err != nil || !empty {
		t.Fatalf("populated zero-key complement empty=%v err=%v", empty, err)
	}
	if err := complement.Close(); err != nil {
		t.Fatal(err)
	}

	if err := table.Reset(); err != nil {
		t.Fatal(err)
	}
	if empty, err := table.Empty(); err != nil || !empty {
		t.Fatalf("reset table empty=%v err=%v", empty, err)
	}
	if err := table.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Empty(); !errors.Is(err, ErrTable) {
		t.Fatalf("closed table empty error=%v", err)
	}
}

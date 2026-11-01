package rel

import (
	"errors"
	"testing"
)

func sieveSignature(t *testing.T, domains ...uint64) Signature {
	t.Helper()
	signature, err := NewSignature(domains)
	if err != nil {
		t.Fatal(err)
	}
	return signature
}

func closeSieve(t *testing.T, relations ...*SieveRelation) {
	t.Helper()
	t.Cleanup(func() {
		for _, relation := range relations {
			if relation != nil {
				if err := relation.Close(); err != nil {
					t.Error(err)
				}
			}
		}
	})
}

func requireSieveFact(t *testing.T, relation *SieveRelation, fact Fact, want bool) {
	t.Helper()
	got, err := relation.ContainsFact(fact)
	if err != nil || got != want {
		t.Fatalf("fact %v: got %v want %v: %v", fact, got, want, err)
	}
}

func enumerateSignatureFacts(t *testing.T, signature Signature, visit func(Fact)) {
	t.Helper()
	fact := make(Fact, signature.Len())
	var enumerate func(int)
	enumerate = func(column int) {
		if column == len(fact) {
			visit(cloneFact(fact))
			return
		}
		for value := uint64(0); value < signature.domains[column]; value++ {
			fact[column] = Element(value)
			enumerate(column + 1)
		}
	}
	enumerate(0)
}

func TestSieveRelationSourceFactProjectionAndMetadata(t *testing.T) {
	signature := sieveSignature(t, 2, 3, 2)
	relation, err := NewSieveRelation(signature, []bool{true, false, true})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, relation)
	if relation.NoSievedColumns() || relation.NoInnerColumns() {
		t.Fatal("mixed mask classification")
	}
	if got := relation.InnerColumns(); len(got) != 3 || !got[0] || got[1] || !got[2] {
		t.Fatal("inner mask", got)
	}
	if column, err := relation.InnerColumn(2); err != nil || column != 1 {
		t.Fatal("inner column", column, err)
	}
	if _, err := relation.InnerColumn(1); !errors.Is(err, ErrSieve) {
		t.Fatal("ignored column lookup", err)
	}
	inserted, err := relation.AddFact(Fact{1, 0, 1})
	if err != nil || !inserted {
		t.Fatal("add", inserted, err)
	}
	inserted, err = relation.AddFact(Fact{1, 2, 1})
	if err != nil || inserted {
		t.Fatal("ignored-coordinate duplicate", inserted, err)
	}
	for ignored := Element(0); ignored < 3; ignored++ {
		requireSieveFact(t, relation, Fact{1, ignored, 1}, true)
	}
	requireSieveFact(t, relation, Fact{0, 2, 1}, false)
	if _, err = relation.AddFact(Fact{1, 3, 1}); !errors.Is(err, ErrSieve) {
		t.Fatal("domain validation", err)
	}
	if text, err := relation.String(); err != nil || text == "" {
		t.Fatal("display", text, err)
	}
}

func TestSieveRelationCloneResetAndLifecycle(t *testing.T) {
	signature := sieveSignature(t, 2, 2)
	relation, err := NewSieveRelation(signature, []bool{true, false})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = relation.AddFact(Fact{1, 0})
	clone, err := relation.Clone()
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, clone)
	if err = relation.Reset(); err != nil || !relation.Empty() {
		t.Fatal("reset", relation.Empty(), err)
	}
	requireSieveFact(t, clone, Fact{1, 1}, true)
	if err = relation.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = relation.ContainsFact(Fact{1, 0}); !errors.Is(err, ErrSieve) {
		t.Fatal("closed relation", err)
	}
	if err = relation.Close(); err != nil {
		t.Fatal("idempotent close", err)
	}
}

func TestSieveRelationComplementFiniteOracle(t *testing.T) {
	signature := sieveSignature(t, 2, 3, 2)
	relation, err := NewSieveRelation(signature, []bool{true, false, false})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, relation)
	_, _ = relation.AddFact(Fact{0, 0, 1})
	complement, err := relation.Complement()
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, complement)
	enumerateSignatureFacts(t, signature, func(fact Fact) {
		original, originalErr := relation.ContainsFact(fact)
		negated, negatedErr := complement.ContainsFact(fact)
		if originalErr != nil || negatedErr != nil || original == negated {
			t.Fatalf("complement fact %v original=%v complement=%v errors=%v/%v", fact, original, negated, originalErr, negatedErr)
		}
	})

	full, err := NewFullSieve(signature)
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, full)
	enumerateSignatureFacts(t, signature, func(fact Fact) { requireSieveFact(t, full, fact, true) })
	empty, err := full.Complement()
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, empty)
	if !empty.Empty() {
		t.Fatal("full complement must be empty")
	}
	wide, err := NewSieveRelation(signature, []bool{true, false, true})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, wide)
	if _, err = wide.Complement(); !errors.Is(err, ErrSieve) {
		t.Fatal("wide table complement must be compiler-decomposed", err)
	}
}

func TestSieveRelationAdoptionValidatesInnerShape(t *testing.T) {
	signature := sieveSignature(t, 2, 3)
	wrong := NewHashTable(sieveSignature(t, 3))
	if _, err := AdoptSieveRelation(signature, []bool{true, false}, wrong); !errors.Is(err, ErrSieve) {
		t.Fatal("inner domain mismatch", err)
	}
	if _, err := wrong.Len(); err != nil {
		t.Fatal("failed adoption consumed the inner table", err)
	}
	if err := wrong.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSieveRelation(signature, []bool{true}); !errors.Is(err, ErrSieve) {
		t.Fatal("mask arity", err)
	}
}

func TestSieveRelationClosesAdoptedTable(t *testing.T) {
	signature := sieveSignature(t, 2, 3)
	inner := NewHashTable(sieveSignature(t, 2))
	if _, err := inner.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	relation, err := AdoptSieveRelation(signature, []bool{true, false}, inner)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := relation.Clone()
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, clone)
	if err = relation.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = inner.Len(); !errors.Is(err, ErrTable) {
		t.Fatal("sieve did not close its adopted table", err)
	}
	requireSieveFact(t, clone, Fact{1, 0}, true)
	if err = relation.Close(); err != nil {
		t.Fatal("repeated close", err)
	}
}

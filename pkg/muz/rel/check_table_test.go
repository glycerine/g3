package rel

import (
	"errors"
	"testing"
)

func newCheckTableWithFacts(t *testing.T, signature Signature, facts ...Fact) *CheckTable {
	t.Helper()
	table, err := NewCheckTable(signature)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range facts {
		if _, err = table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	return table
}

func closeCheckTables(t *testing.T, tables ...*CheckTable) {
	t.Helper()
	t.Cleanup(func() {
		for _, table := range tables {
			if table != nil {
				if err := table.Close(); err != nil {
					t.Error(err)
				}
			}
		}
	})
}

func requireCheckedFact(t *testing.T, table *CheckTable, fact Fact, want bool) {
	t.Helper()
	got, err := table.Contains(fact)
	if err != nil || got != want {
		t.Fatalf("fact %v: got %v want %v: %v", fact, got, want, err)
	}
}

func TestCheckTableSourceDirectOperationsAndValidationCount(t *testing.T) {
	table := newCheckTableWithFacts(t, lazySignature(t, 2, 3), Fact{0, 1}, Fact{1, 2})
	closeCheckTables(t, table)
	if rows, err := table.SizeEstimateRows(); err != nil || rows != 2 {
		t.Fatalf("row estimate: %d, %v", rows, err)
	}
	if bytes, err := table.SizeEstimateBytes(); err != nil || bytes != 32 {
		t.Fatalf("byte estimate: %d, %v", bytes, err)
	}
	before := table.CheckCount()
	inserted, err := table.Add(Fact{0, 1})
	if err != nil || inserted {
		t.Fatal("duplicate", inserted, err)
	}
	if table.CheckCount() <= before {
		t.Fatal("add did not validate")
	}
	requireCheckedFact(t, table, Fact{1, 2}, true)
	removed, err := table.Remove(Fact{0, 1})
	if err != nil || !removed {
		t.Fatal("remove", removed, err)
	}
	requireCheckedFact(t, table, Fact{0, 1}, false)
	facts, err := table.Facts()
	if err != nil || len(facts) != 1 || facts[0][0] != 1 || facts[0][1] != 2 {
		t.Fatal("facts", facts, err)
	}
	primary, err := table.Primary()
	if err != nil || primary != table.primary {
		t.Fatal("primary", primary, err)
	}
}

func TestCheckTableBulkRemoveChecksEachFactInOrder(t *testing.T) {
	table := newCheckTableWithFacts(t, lazySignature(t, 2), Fact{0}, Fact{1})
	closeCheckTables(t, table)
	before := table.CheckCount()
	if err := table.RemoveFacts([]Fact{{0}, {0}, {1}}); err != nil {
		t.Fatal(err)
	}
	if table.CheckCount()-before != 3 {
		t.Fatal("bulk removal did not validate each source fact", before, table.CheckCount())
	}
	if empty, err := table.Empty(); err != nil || !empty {
		t.Fatal("empty after bulk removal", empty, err)
	}
}

func TestCheckTableCloneResetComplementAndLifecycle(t *testing.T) {
	table := newCheckTableWithFacts(t, lazySignature(t, 3), Fact{0}, Fact{2})
	clone, err := table.Clone()
	if err != nil {
		t.Fatal(err)
	}
	complement, err := table.Complement()
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, table, clone, complement)
	requireCheckedFact(t, complement, Fact{1}, true)
	requireCheckedFact(t, complement, Fact{0}, false)
	if err = table.Reset(); err != nil {
		t.Fatal(err)
	}
	primary, err := table.Primary()
	if err != nil {
		t.Fatal(err)
	}
	checker := table.checker
	empty, err := table.Empty()
	if err != nil || !empty {
		t.Fatal("reset empty", empty, err)
	}
	requireCheckedFact(t, clone, Fact{2}, true)
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = table.Facts(); !errors.Is(err, ErrCheckTable) {
		t.Fatal("closed table", err)
	}
	if _, err = primary.Add(Fact{0, 1}); !errors.Is(err, ErrTable) {
		t.Fatal("owner close left adopted primary table usable", err)
	}
	if _, err = checker.Add(Fact{0, 1}); !errors.Is(err, ErrTable) {
		t.Fatal("owner close left adopted checker table usable", err)
	}
	wide := newCheckTableWithFacts(t, lazySignature(t, 2, 2))
	closeCheckTables(t, wide)
	if _, err = wide.Complement(); !errors.Is(err, ErrCheckTable) {
		t.Fatal("wide complement", err)
	}
}

func TestCheckTableAdoptionAndRuntimeMismatchDiagnostics(t *testing.T) {
	signature := lazySignature(t, 2)
	primary := NewHashTable(signature)
	checker := NewHashTable(signature)
	_, _ = primary.Add(Fact{1})
	if _, err := AdoptCheckTable(primary, checker); !errors.Is(err, ErrCheckTable) {
		t.Fatal("adopted mismatch", err)
	}

	table := newCheckTableWithFacts(t, signature, Fact{0})
	closeCheckTables(t, table)
	_, _ = table.primary.Add(Fact{1})
	if err := table.Validate(); !errors.Is(err, ErrCheckTable) {
		t.Fatal("runtime mismatch", err)
	}
	empty, err := table.Empty()
	if err != nil || empty {
		t.Fatal("nonempty mismatch need not affect empty", empty, err)
	}
	_, _ = table.checker.Add(Fact{1})
	if err = table.Validate(); err != nil {
		t.Fatal("repaired mismatch", err)
	}
	if _, err = table.primary.Remove(Fact{0}); err != nil {
		t.Fatal(err)
	}
	if _, err = table.primary.Remove(Fact{1}); err != nil {
		t.Fatal(err)
	}
	if _, err = table.Empty(); !errors.Is(err, ErrCheckTable) {
		t.Fatal("empty disagreement", err)
	}
}

func TestCheckTableSignatureAndOwnershipValidation(t *testing.T) {
	first := NewHashTable(lazySignature(t, 2))
	second := NewHashTable(lazySignature(t, 3))
	if _, err := AdoptCheckTable(first, second); !errors.Is(err, ErrCheckTable) {
		t.Fatal("signature mismatch", err)
	}
	if _, err := AdoptCheckTable(first, first); !errors.Is(err, ErrCheckTable) {
		t.Fatal("aliased ownership", err)
	}
}

func TestCheckTableZeroArityComplementAndIndependentCopies(t *testing.T) {
	empty := newCheckTableWithFacts(t, lazySignature(t))
	complement, err := empty.Complement()
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, empty, complement)
	if rows, err := complement.Facts(); err != nil || len(rows) != 1 || len(rows[0]) != 0 {
		t.Fatalf("empty zero-arity complement: %#v, %v", rows, err)
	}
	if err := complement.Reset(); err != nil {
		t.Fatal(err)
	}
	if rows, err := empty.Facts(); err != nil || len(rows) != 0 {
		t.Fatalf("complement reset changed source: %#v, %v", rows, err)
	}

	full := newCheckTableWithFacts(t, lazySignature(t), Fact{})
	fullComplement, err := full.Complement()
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, full, fullComplement)
	if rows, err := fullComplement.Facts(); err != nil || len(rows) != 0 {
		t.Fatalf("full zero-arity complement: %#v, %v", rows, err)
	}
}

func TestCheckTableRejectsInvalidOperationsWithoutMutation(t *testing.T) {
	target := newCheckTableWithFacts(t, lazySignature(t, 2, 2), Fact{0, 1})
	wrongSignature := newCheckTableWithFacts(t, lazySignature(t, 2), Fact{1})
	closeCheckTables(t, target, wrongSignature)
	if _, err := target.Join(wrongSignature, []int{2}, []int{0}); err == nil {
		t.Fatal("join accepted an out-of-range column")
	}
	if err := target.UnionFrom(wrongSignature, nil); !errors.Is(err, ErrCheckTable) {
		t.Fatalf("union mismatch: %v", err)
	}
	if err := target.FilterByNegation(wrongSignature, []int{0}, []int{2}); err == nil {
		t.Fatal("negation accepted an out-of-range column")
	}
	if rows, err := target.Facts(); err != nil || len(rows) != 1 || rows[0][0] != 0 || rows[0][1] != 1 {
		t.Fatalf("rejected operation changed target: %#v, %v", rows, err)
	}
}

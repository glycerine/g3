package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSieveTableConversionOwnsIndependentSnapshots(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	table, err := NewTableRelation(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{0, 1}, {1, 2}} {
		if _, err = table.AddTableFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	sieve, err := NewSieveRelationFromTable(table)
	if err != nil {
		t.Fatal(err)
	}
	defer sieve.Close()
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
	if !sieve.NoSievedColumns() {
		t.Fatal("ordinary conversion did not produce all-inner layout")
	}
	requireSieveFact(t, sieve, Fact{0, 1}, true)
	requireSieveFact(t, sieve, Fact{1, 2}, true)

	snapshot, err := sieve.TableRelationSnapshot(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if err = sieve.Reset(); err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{0, 1}, {1, 2}} {
		found, containsErr := snapshot.table.Contains(fact)
		if containsErr != nil || !found {
			t.Fatal("snapshot lost fact after source reset", fact, found, containsErr)
		}
	}
}

func TestSieveTableConversionRefusesApproximateLayout(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	partial, err := NewFullSieveWithColumns(signature, []bool{true, false})
	if err != nil {
		t.Fatal(err)
	}
	defer partial.Close()
	if _, err = partial.TableRelationSnapshot(f.relations, sorts); !errors.Is(err, ErrSieve) {
		t.Fatal("partial sieve converted without expanding ignored column", err)
	}
}

func TestSieveTableConversionNullaryIdentity(t *testing.T) {
	f := newTableRelationFixture(t)
	table, err := NewTableRelation(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()
	if _, err = table.AddTableFact(Fact{}); err != nil {
		t.Fatal(err)
	}
	sieve, err := NewSieveRelationFromTable(table)
	if err != nil {
		t.Fatal(err)
	}
	defer sieve.Close()
	requireSieveFact(t, sieve, Fact{}, true)
	snapshot, err := sieve.TableRelationSnapshot(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	found, err := snapshot.table.Contains(Fact{})
	if err != nil || !found {
		t.Fatal("nullary identity", found, err)
	}
}

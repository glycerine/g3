package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSieveTableTransferMovesConcreteOwner(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	table, err := NewTableRelation(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = table.AddTableFact(Fact{1, 2}); err != nil {
		t.Fatal(err)
	}
	owner := table.table
	sieve, err := TakeSieveRelationFromTable(table)
	if err != nil {
		t.Fatal(err)
	}
	if table.open() == nil || sieve.inner != owner {
		t.Fatal("ordinary-to-sieve transfer did not consume and move owner")
	}
	requireSieveFact(t, sieve, Fact{1, 2}, true)

	ordinary, err := sieve.TakeTableRelation(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	defer ordinary.Close()
	if sieve.open() == nil || ordinary.table != owner {
		t.Fatal("sieve-to-ordinary transfer did not consume and move owner")
	}
	found, err := ordinary.table.Contains(Fact{1, 2})
	if err != nil || !found {
		t.Fatal("transferred fact", found, err)
	}
}

func TestSieveTableTransferFailureLeavesPartialSourceOwned(t *testing.T) {
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
	owner := partial.inner
	if _, err = partial.TakeTableRelation(f.relations, sorts); !errors.Is(err, ErrSieve) {
		t.Fatal("partial sieve transfer accepted", err)
	}
	if partial.open() != nil || partial.inner != owner {
		t.Fatal("failed transfer consumed partial source")
	}
	requireSieveFact(t, partial, Fact{0, 0}, true)
	requireSieveFact(t, partial, Fact{0, 3}, true)
}

func TestSieveTableTransferNullaryIdentity(t *testing.T) {
	f := newTableRelationFixture(t)
	table, err := NewTableRelation(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = table.AddTableFact(Fact{}); err != nil {
		t.Fatal(err)
	}
	sieve, err := TakeSieveRelationFromTable(table)
	if err != nil {
		t.Fatal(err)
	}
	requireSieveFact(t, sieve, Fact{}, true)
	ordinary, err := sieve.TakeTableRelation(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ordinary.Close()
	found, err := ordinary.table.Contains(Fact{})
	if err != nil || !found {
		t.Fatal("nullary transfer identity", found, err)
	}
}

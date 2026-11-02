package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSieveJoinTableRelationBothOrdersAndOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()
	if _, err = table.AddTableFact(Fact{1}); err != nil {
		t.Fatal(err)
	}
	sieve, err := NewSieveRelation(sieveSignature(t, 2, 4), []bool{true, false})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sieve.AddFact(Fact{0, 0}); err != nil {
		t.Fatal(err)
	}
	if _, err = sieve.AddFact(Fact{1, 0}); err != nil {
		t.Fatal(err)
	}
	left, err := sieve.JoinTableRelation(table, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	defer left.Close()
	if mask := left.InnerColumns(); len(mask) != 3 || !mask[0] || mask[1] || !mask[2] {
		t.Fatal("left mixed mask", mask)
	}
	for ignored := Element(0); ignored < 4; ignored++ {
		requireSieveFact(t, left, Fact{1, ignored, 1}, true)
	}
	requireSieveFact(t, left, Fact{0, 0, 1}, false)

	right, err := sieve.JoinFromTableRelation(table, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	defer right.Close()
	if mask := right.InnerColumns(); len(mask) != 3 || !mask[0] || !mask[1] || mask[2] {
		t.Fatal("right mixed mask", mask)
	}
	for ignored := Element(0); ignored < 4; ignored++ {
		requireSieveFact(t, right, Fact{1, 1, ignored}, true)
	}
	requireSieveFact(t, right, Fact{1, 0, 0}, false)

	if err = sieve.Close(); err != nil {
		t.Fatal(err)
	}
	if err = table.Reset(); err != nil {
		t.Fatal(err)
	}
	// Joined results own their tables and do not borrow either input's rows.
	requireSieveFact(t, left, Fact{1, 3, 1}, true)
	requireSieveFact(t, right, Fact{1, 1, 2}, true)
}

func TestSieveJoinTableRelationSkipsFullEndpointEquality(t *testing.T) {
	f := newTableRelationFixture(t)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.bits})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()
	if _, err = table.AddTableFact(Fact{3}); err != nil {
		t.Fatal(err)
	}
	sieve, err := NewSieveRelation(sieveSignature(t, 2, 4), []bool{true, false})
	if err != nil {
		t.Fatal(err)
	}
	defer sieve.Close()
	if _, err = sieve.AddFact(Fact{1, 0}); err != nil {
		t.Fatal(err)
	}
	joined, err := sieve.JoinTableRelation(table, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	defer joined.Close()
	for ignored := Element(0); ignored < 4; ignored++ {
		requireSieveFact(t, joined, Fact{1, ignored, 3}, true)
	}
	if joined.NumInnerRows() != 1 {
		t.Fatal("skipped equality should keep Cartesian inner row", joined.NumInnerRows())
	}
	if _, err = sieve.JoinTableRelation(table, []int{2}, []int{0}); err == nil {
		t.Fatal("out-of-range equality accepted")
	}
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = sieve.JoinTableRelation(table, nil, nil); !errors.Is(err, ErrSieve) {
		t.Fatal("closed table accepted", err)
	}
}

package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSieveUnionOrdinarySourceOrdinaryDelta(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewSieveRelation(signature, []bool{true, true})
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewTableRelation(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	delta, err := NewTableRelation(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	defer source.Close()
	defer delta.Close()
	if _, err = target.AddFact(Fact{0, 0}); err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{0, 0}, {1, 2}, {1, 3}} {
		if _, err = source.AddTableFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err = target.UnionFromTableRelationWithTableDelta(source, delta); err != nil {
		t.Fatal(err)
	}
	if target.NumInnerRows() != 3 {
		t.Fatal("target rows", target.NumInnerRows())
	}
	rows, err := delta.table.Facts()
	if err != nil || len(rows) != 2 {
		t.Fatal("ordinary delta rows", rows, err)
	}
	if err = target.UnionFromTableRelationWithTableDelta(source, delta); err != nil {
		t.Fatal(err)
	}
	rows, _ = delta.table.Facts()
	if len(rows) != 2 {
		t.Fatal("repeated union changed delta", rows)
	}
	partial, err := NewSieveRelation(signature, []bool{true, false})
	if err != nil {
		t.Fatal(err)
	}
	defer partial.Close()
	if err = partial.UnionFromTableRelationWithTableDelta(source, delta); !errors.Is(err, ErrSieve) {
		t.Fatal("partial sieve accepted ordinary delta union", err)
	}
}

func TestSieveUnionOrdinaryTargetSieveDelta(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSieveRelation(signature, []bool{true})
	if err != nil {
		t.Fatal(err)
	}
	delta, err := NewSieveRelation(signature, []bool{true})
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewTableRelation(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	defer delta.Close()
	defer target.Close()
	if _, err = source.AddFact(Fact{0}); err != nil {
		t.Fatal(err)
	}
	if _, err = source.AddFact(Fact{1}); err != nil {
		t.Fatal(err)
	}
	if _, err = target.AddTableFact(Fact{0}); err != nil {
		t.Fatal(err)
	}
	if err = source.UnionIntoTableRelationWithSieveDelta(target, delta); err != nil {
		t.Fatal(err)
	}
	if delta.NumInnerRows() != 1 {
		t.Fatal("sieve delta rows", delta.NumInnerRows())
	}
	requireSieveFact(t, delta, Fact{1}, true)
	requireSieveFact(t, delta, Fact{0}, false)
	partialDelta, err := NewSieveRelation(signature, []bool{false})
	if err != nil {
		t.Fatal(err)
	}
	defer partialDelta.Close()
	if err = source.UnionIntoTableRelationWithSieveDelta(target, partialDelta); !errors.Is(err, ErrSieve) {
		t.Fatal("partial sieve delta accepted", err)
	}
}

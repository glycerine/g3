package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSieveUnionFromTableRelationWithDelta(t *testing.T) {
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
	defer target.Close()
	delta, err := NewSieveRelation(signature, []bool{true, true})
	if err != nil {
		t.Fatal(err)
	}
	defer delta.Close()
	source, err := NewTableRelation(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if _, err = target.AddFact(Fact{0, 0}); err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{0, 0}, {1, 2}, {1, 3}} {
		if _, err = source.AddTableFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err = target.UnionFromTableRelation(source, delta); err != nil {
		t.Fatal(err)
	}
	if target.NumInnerRows() != 3 || delta.NumInnerRows() != 2 {
		t.Fatal("mixed union rows", target.NumInnerRows(), delta.NumInnerRows())
	}
	requireSieveFact(t, delta, Fact{1, 2}, true)
	requireSieveFact(t, delta, Fact{1, 3}, true)
	requireSieveFact(t, delta, Fact{0, 0}, false)
	// Source union appends new rows to caller-managed delta storage; it does not
	// reset rows recorded by an earlier invocation.
	if err = target.UnionFromTableRelation(source, delta); err != nil || delta.NumInnerRows() != 2 {
		t.Fatal("repeated union delta", delta.NumInnerRows(), err)
	}

	partial, err := NewSieveRelation(signature, []bool{true, false})
	if err != nil {
		t.Fatal(err)
	}
	defer partial.Close()
	if err = partial.UnionFromTableRelation(source, nil); !errors.Is(err, ErrSieve) {
		t.Fatal("partial target accepted ordinary source", err)
	}
}

func TestSieveUnionIntoTableRelationWithDelta(t *testing.T) {
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
	defer source.Close()
	if _, err = source.AddFact(Fact{0}); err != nil {
		t.Fatal(err)
	}
	if _, err = source.AddFact(Fact{1}); err != nil {
		t.Fatal(err)
	}
	target, err := NewTableRelation(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	delta, err := NewTableRelation(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	defer delta.Close()
	if _, err = target.AddTableFact(Fact{0}); err != nil {
		t.Fatal(err)
	}
	if err = source.UnionIntoTableRelation(target, delta); err != nil {
		t.Fatal(err)
	}
	if target.NumRows() != 2 || delta.NumRows() != 1 {
		t.Fatal("mixed inverse union rows", target.NumRows(), delta.NumRows())
	}
	found, err := delta.Table().Contains(Fact{1})
	if err != nil || !found {
		t.Fatal("mixed inverse delta", found, err)
	}
	partial, err := NewSieveRelation(signature, []bool{false})
	if err != nil {
		t.Fatal(err)
	}
	defer partial.Close()
	if _, err = partial.AddFact(Fact{0}); err != nil {
		t.Fatal(err)
	}
	if err = partial.UnionIntoTableRelation(target, nil); !errors.Is(err, ErrSieve) {
		t.Fatal("partial source accepted ordinary target", err)
	}
	if err = target.Close(); err != nil {
		t.Fatal(err)
	}
	if err = source.UnionIntoTableRelation(target, nil); !errors.Is(err, ErrSieve) {
		t.Fatal("closed target accepted", err)
	}
}

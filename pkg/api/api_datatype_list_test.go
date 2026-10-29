package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextListDatatypeConstruction(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	list, err := f.c.ListSort("api-bool-list", boolean)
	astTestMust(t, err)
	list.Sort = f.pin(list.Sort, nil)
	for _, handle := range []ast.ASTHandle{list.Nil, list.IsNil, list.Cons, list.IsCons, list.Head, list.Tail} {
		f.pin(handle, nil)
	}
	if recursive, e := f.c.DatatypeIsRecursive(list.Sort); e != nil || !recursive {
		t.Fatal("list recursion", recursive, e)
	}
	if count, e := f.c.DatatypeConstructorCount(list.Sort); e != nil || count != 2 {
		t.Fatal("list constructors", count, e)
	}
	nilValue := f.pin(f.c.Apply(list.Nil))
	truth := f.pin(f.c.Bool(true))
	consValue := f.pin(f.c.Apply(list.Cons, truth, nilValue))
	if sort := f.pin(f.c.SortOf(consValue)); sort != list.Sort {
		t.Fatal("list value sort", sort, list.Sort)
	}
	if rangeSort, e := f.c.DeclarationRange(list.Head); e != nil || rangeSort != boolean {
		t.Fatal("head range", rangeSort, boolean, e)
	}
	if rangeSort, e := f.c.DeclarationRange(list.Tail); e != nil || rangeSort != list.Sort {
		t.Fatal("tail range", rangeSort, list.Sort, e)
	}
}

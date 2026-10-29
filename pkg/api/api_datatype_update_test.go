package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextDatatypeFieldUpdate(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	tuple, err := f.c.TupleSort("api-updatable", []string{"first", "second"}, []ast.ASTHandle{boolean, boolean})
	astTestMust(t, err)
	tuple.Sort = f.pin(tuple.Sort, nil)
	tuple.Constructor = f.pin(tuple.Constructor, nil)
	for i := range tuple.Fields {
		tuple.Fields[i] = f.pin(tuple.Fields[i], nil)
	}
	truth := f.pin(f.c.Bool(true))
	falsehood := f.pin(f.c.Bool(false))
	value := f.pin(f.c.Apply(tuple.Constructor, truth, falsehood))
	updated := f.pin(f.c.DatatypeUpdateField(tuple.Fields[1], value, truth))
	if sort := f.pin(f.c.SortOf(updated)); sort != tuple.Sort {
		t.Fatal("updated sort", sort, tuple.Sort)
	}
	declaration := f.pin(f.c.ApplicationDeclaration(updated))
	if kind, e := f.c.DeclarationKind(declaration); e != nil || kind != ast.ASTDatatypeUpdateField {
		t.Fatal("update declaration", kind, e)
	}
	if accessor, matched, e := f.m.DatatypeUpdateAccessor(declaration); e != nil || !matched || accessor != tuple.Fields[1] {
		t.Fatal("update accessor", accessor, matched, e)
	}
	integer := f.arithmeticVariable("wrong-update-value", true)
	if _, err := f.c.DatatypeUpdateField(tuple.Fields[0], value, integer); err == nil {
		t.Fatal("wrong replacement sort accepted")
	}
}

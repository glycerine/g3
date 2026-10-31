package api

import (
	"errors"
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
	// api_datatype.cpp dispatches kind 4 through the array family, where
	// it denotes array-default and refuses the two-domain declaration.
	updated, err := f.c.DatatypeUpdateField(tuple.Fields[1], value, truth)
	if !updated.IsNull() || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("source array-family update dispatch", updated, err)
	}
	integer := f.arithmeticVariable("wrong-update-value", true)
	if _, err := f.c.DatatypeUpdateField(tuple.Fields[0], value, integer); err == nil {
		t.Fatal("wrong replacement sort accepted")
	}
}

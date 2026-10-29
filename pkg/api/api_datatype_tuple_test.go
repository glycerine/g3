package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextTupleDatatypeConstructionAndQueries(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	integer := f.pin(f.c.IntegerSort())
	tuple, err := f.c.TupleSort("api-pair", []string{"flag", "number"}, []ast.ASTHandle{boolean, integer})
	astTestMust(t, err)
	tuple.Sort = f.pin(tuple.Sort, nil)
	tuple.Constructor = f.pin(tuple.Constructor, nil)
	for i := range tuple.Fields {
		tuple.Fields[i] = f.pin(tuple.Fields[i], nil)
	}
	if recursive, e := f.c.DatatypeIsRecursive(tuple.Sort); e != nil || recursive {
		t.Fatal("tuple recursion", recursive, e)
	}
	if count, e := f.c.TupleFieldCount(tuple.Sort); e != nil || count != 2 {
		t.Fatal("tuple fields", count, e)
	}
	if constructor, e := f.c.TupleConstructor(tuple.Sort); e != nil || constructor != tuple.Constructor {
		t.Fatal("tuple constructor", constructor, tuple.Constructor, e)
	}
	for i, want := range tuple.Fields {
		field, e := f.c.TupleField(tuple.Sort, uint32(i))
		if e != nil || field != want {
			t.Fatal("tuple field", i, field, want, e)
		}
	}
	truth := f.pin(f.c.Bool(true))
	three := f.pin(f.m.ArithmeticIntegerNumeralText("3"))
	value := f.pin(f.c.Apply(tuple.Constructor, truth, three))
	if sort := f.pin(f.c.SortOf(value)); sort != tuple.Sort {
		t.Fatal("tuple value sort", sort, tuple.Sort)
	}
	if _, err = f.c.TupleField(tuple.Sort, 2); err == nil {
		t.Fatal("out-of-bounds tuple field accepted")
	}
}

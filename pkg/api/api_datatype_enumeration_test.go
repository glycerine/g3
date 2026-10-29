package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextEnumerationDatatypeConstruction(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	enumeration, err := f.c.EnumerationSort("api-color", []string{"red", "green", "blue"})
	astTestMust(t, err)
	enumeration.Sort = f.pin(enumeration.Sort, nil)
	if count, e := f.c.DatatypeConstructorCount(enumeration.Sort); e != nil || count != 3 {
		t.Fatal("enumeration constructor count", count, e)
	}
	for i := range enumeration.Constants {
		enumeration.Constants[i] = f.pin(enumeration.Constants[i], nil)
		enumeration.Recognizers[i] = f.pin(enumeration.Recognizers[i], nil)
		constant := f.pin(f.c.Apply(enumeration.Constants[i]))
		predicate := f.pin(f.c.Apply(enumeration.Recognizers[i], constant))
		if sort := f.pin(f.c.SortOf(constant)); sort != enumeration.Sort {
			t.Fatal("enumeration constant sort", i, sort, enumeration.Sort)
		}
		if kind, e := f.c.DeclarationKind(enumeration.Recognizers[i]); e != nil || kind != ast.ASTDatatypeRecognizer {
			t.Fatal("recognizer kind", i, kind, e)
		}
		if sort := f.pin(f.c.SortOf(predicate)); sort != f.pin(f.c.BoolSort()) {
			t.Fatal("recognizer result sort", i, sort)
		}
	}
	if recursive, e := f.c.DatatypeIsRecursive(enumeration.Sort); e != nil || recursive {
		t.Fatal("enumeration recursion", recursive, e)
	}
	if _, err := f.c.EnumerationSort("empty-api-enum", nil); err == nil {
		t.Fatal("empty enumeration accepted")
	}
}

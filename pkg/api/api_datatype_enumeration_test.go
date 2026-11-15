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
		if kind, e := f.c.DeclarationKind(enumeration.Recognizers[i]); e != nil || kind != ast.ASTDatatypeIs { // get_constructor_is (AUDIT api_01-F2)
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

// AUDIT api_01-F2: every public tester path (Z3_mk_enumeration_sort,
// Z3_get_datatype_sort_recognizer, Z3_mk_list_sort, Z3_query_constructor)
// returns datatype_util::get_constructor_is, the OP_DT_IS declaration named
// "is". build/libz3.so reports name "is" and kind Z3_OP_DT_IS for each.
func TestEnumerationTesterIsGenericIsDecl(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	check := func(label string, decl ast.ASTHandle) {
		t.Helper()
		name, err := f.c.DeclarationName(decl)
		astTestMust(t, err)
		kind, err := f.c.PublicDeclarationKind(decl)
		astTestMust(t, err)
		if name != "is" || kind != APIDeclarationDatatypeIs {
			t.Fatalf("%s: name %q kind %v, want \"is\" and APIDeclarationDatatypeIs", label, name, kind)
		}
	}
	e, err := f.c.EnumerationSort("Color", []string{"red", "green"})
	astTestMust(t, err)
	f.pin(e.Sort, nil)
	for i := range e.Recognizers {
		f.pin(e.Recognizers[i], nil)
	}
	check("enumeration tester", e.Recognizers[0])
	check("sort recognizer", f.pin(f.c.DatatypeRecognizer(e.Sort, 1)))

	list, err := f.c.ListSort("IL", f.pin(f.c.IntegerSort()))
	astTestMust(t, err)
	f.pin(list.Sort, nil)
	check("is-nil", f.pin(list.IsNil, nil))
	check("is-cons", f.pin(list.IsCons, nil))

	constructor, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "mk", Recognizer: "is_mk"})
	astTestMust(t, err)
	defer func() { astTestMust(t, constructor.Close()) }()
	f.pin(f.c.DatatypeSortFromConstructors("P", constructor))
	query, err := constructor.QueryFields(0)
	astTestMust(t, err)
	check("query constructor", f.pin(query.Recognizer, nil))
}

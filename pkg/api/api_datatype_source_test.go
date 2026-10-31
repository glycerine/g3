package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestAPIDatatypeSymbolNamesAndGeneratedRecognizers(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	for _, name := range []usymbol.Symbol{usymbol.Numerical(37), {}} {
		tuple, err := f.c.TupleSortWithSymbol(name, []usymbol.Symbol{usymbol.Numerical(8)}, []ast.ASTHandle{boolean})
		astTestMust(t, err)
		f.pin(tuple.Sort, nil)
		actual, err := f.c.SortSymbol(tuple.Sort)
		astTestMust(t, err)
		if actual != name {
			t.Fatal("tuple name lost source symbol identity", actual, name)
		}
		for _, declaration := range []ast.ASTHandle{tuple.Constructor, tuple.Fields[0]} {
			actual, err = f.c.DeclarationSymbol(declaration)
			astTestMust(t, err)
			want := name
			if declaration == tuple.Fields[0] {
				want = usymbol.Numerical(8)
			}
			if actual != want {
				t.Fatal("tuple declaration symbol", actual, want)
			}
		}
		recognizer, err := f.c.DatatypeRecognizer(tuple.Sort, 0)
		astTestMust(t, err)
		actual, err = f.c.DeclarationSymbol(recognizer)
		astTestMust(t, err)
		spelling, err := f.c.SymbolString(actual)
		astTestMust(t, err)
		want := "is_k!37"
		if name.IsNull() {
			want = "is_<null>"
		}
		if spelling != want {
			t.Fatal("source symbol::str recognizer", spelling, want)
		}
	}
	text, err := f.c.StringSymbol("plain")
	astTestMust(t, err)
	elements := []usymbol.Symbol{usymbol.Numerical(2), {}, text}
	enumeration, err := f.c.EnumerationSortWithSymbol(usymbol.Numerical(77), elements)
	astTestMust(t, err)
	f.pin(enumeration.Sort, nil)
	for i, element := range elements {
		actual, err := f.c.DeclarationSymbol(enumeration.Constants[i])
		astTestMust(t, err)
		if actual != element {
			t.Fatal("enumeration constant symbol", i, actual, element)
		}
		symbol, err := f.c.DeclarationSymbol(enumeration.Recognizers[i])
		astTestMust(t, err)
		spelling, err := f.c.SymbolString(symbol)
		astTestMust(t, err)
		if spelling != []string{"is_k!2", "is_<null>", "is_plain"}[i] {
			t.Fatal("enumeration recognizer spelling", i, spelling)
		}
	}
	list, err := f.c.ListSortWithSymbol(usymbol.Numerical(78), boolean)
	astTestMust(t, err)
	f.pin(list.Sort, nil)
	actual, err := f.c.SortSymbol(list.Sort)
	astTestMust(t, err)
	if actual != usymbol.Numerical(78) {
		t.Fatal("list numerical name", actual)
	}
}

func TestAPIDatatypeConstructorSymbolSnapshotAndQueryPrefixes(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	fields := []ast.DatatypeAccessorDefinition{{Name: usymbol.Numerical(1), Range: boolean}, {Range: boolean}}
	object, err := f.c.NewDatatypeConstructorObjectWithSymbols(ast.DatatypeConstructorDefinition{Name: usymbol.Numerical(5), Accessors: fields})
	astTestMust(t, err)
	defer func() { astTestMust(t, object.Close()) }()
	fields[0].Range = ast.ASTHandle{} // The object owns its metadata snapshot.
	sort, err := f.c.DatatypeSortFromConstructorsWithSymbol(usymbol.Numerical(6), object)
	astTestMust(t, err)
	f.pin(sort, nil)
	for count := uint32(0); count <= 2; count++ {
		query, err := object.QueryFields(count)
		astTestMust(t, err)
		if len(query.Accessors) != int(count) {
			t.Fatal("constructor query prefix", count, query)
		}
		name, err := f.c.DeclarationSymbol(query.Constructor)
		astTestMust(t, err)
		if name != usymbol.Numerical(5) {
			t.Fatal("constructor numerical name", name)
		}
		name, err = f.c.DeclarationSymbol(query.Recognizer)
		astTestMust(t, err)
		if !name.IsNull() {
			t.Fatal("constructor null recognizer", name)
		}
		if count != 0 {
			query.Accessors[0] = ast.ASTHandle{}
		}
	}
	if _, err = object.QueryFields(3); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("query beyond source field count", err)
	}
	query, err := object.Query()
	astTestMust(t, err)
	if query.Accessors[0].IsNull() {
		t.Fatal("query result exposed mutable object metadata")
	}
	copied := *object
	if _, err = copied.Query(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("copied constructor query", err)
	}
	if err = copied.Close(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("copied constructor cleanup", err)
	}
}

func TestAPIDatatypeConstructorFailureReleasesEarlierFieldRoots(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	truth := f.pin(f.c.Bool(true))
	before, err := f.m.GetRefCount(boolean)
	astTestMust(t, err)
	object, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "failure", Recognizer: "is_failure", Accessors: []APIDatatypeAccessor{
		{Name: "valid", Sort: boolean}, {Name: "invalid", Sort: truth},
	}})
	if object != nil || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("wrong-kind later field", object, err)
	}
	after, err := f.m.GetRefCount(boolean)
	astTestMust(t, err)
	if after != before {
		t.Fatal("failed constructor leaked earlier field reference", before, after)
	}
}

func TestAPIDatatypeEmptyNamesAndZeroFieldTuple(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if sorts, err := f.c.DatatypeSortsFromConstructorLists(nil, nil); err != nil || len(sorts) != 0 {
		t.Fatal("empty source datatype block", sorts, err)
	}
	if sorts, err := f.c.DatatypeSorts(nil); err != nil || len(sorts) != 0 {
		t.Fatal("empty definition block", sorts, err)
	}
	object, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{})
	astTestMust(t, err)
	defer func() { astTestMust(t, object.Close()) }()
	sort, err := f.c.DatatypeSortFromConstructors("", object)
	astTestMust(t, err)
	f.pin(sort, nil)
	name, err := f.c.SortSymbol(sort)
	astTestMust(t, err)
	if !name.IsNull() {
		t.Fatal("empty convenience name did not become native null symbol", name)
	}
	tuple, err := f.c.TupleSort("unit-tuple", nil, nil)
	astTestMust(t, err)
	f.pin(tuple.Sort, nil)
	if len(tuple.Fields) != 0 {
		t.Fatal("zero-field tuple", tuple)
	}
	if count, err := f.c.TupleFieldCount(tuple.Sort); err != nil || count != 0 {
		t.Fatal("zero-field tuple count", count, err)
	}
}

func TestAPIDatatypeConstructorListsPublishLastAliasAndBorrowObjects(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	constructor, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "shared-constructor", Recognizer: "is_shared"})
	astTestMust(t, err)
	defer func() { astTestMust(t, constructor.Close()) }()
	input := []*APIDatatypeConstructorObject{constructor}
	list, err := f.c.NewDatatypeConstructorList(input...)
	astTestMust(t, err)
	input[0] = nil
	copied := *list
	if err = copied.Close(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("copied list owner", err)
	}
	sorts, err := f.c.DatatypeSortsFromConstructorLists([]string{"alias-first", "alias-last"}, []*APIDatatypeConstructorList{list, list})
	astTestMust(t, err)
	for _, sort := range sorts {
		f.pin(sort, nil)
	}
	query, err := constructor.Query()
	astTestMust(t, err)
	rangeSort, err := f.c.DeclarationRange(query.Constructor)
	astTestMust(t, err)
	if rangeSort != sorts[1] {
		t.Fatal("shared source constructor pointer did not receive last publication", rangeSort, sorts)
	}
	astTestMust(t, list.Close())
	if _, err = constructor.Query(); err != nil {
		t.Fatal("closing pointer list destroyed constructor object", err)
	}
}

func TestAPIDatatypeQueriesRejectInvalidSortsAndIndices(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	other := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	truth := f.pin(f.c.Bool(true))
	foreign := other.pin(other.c.BoolSort())
	queries := []struct {
		name string
		call func(*SMTContext, ast.ASTHandle) error
	}{
		{"count", func(c *SMTContext, s ast.ASTHandle) error { _, err := c.DatatypeConstructorCount(s); return err }},
		{"constructor", func(c *SMTContext, s ast.ASTHandle) error { _, err := c.DatatypeConstructor(s, 0); return err }},
		{"recognizer", func(c *SMTContext, s ast.ASTHandle) error { _, err := c.DatatypeRecognizer(s, 0); return err }},
		{"accessor", func(c *SMTContext, s ast.ASTHandle) error { _, err := c.DatatypeAccessor(s, 0, 0); return err }},
		{"tuple-constructor", func(c *SMTContext, s ast.ASTHandle) error { _, err := c.TupleConstructor(s); return err }},
		{"tuple-count", func(c *SMTContext, s ast.ASTHandle) error { _, err := c.TupleFieldCount(s); return err }},
		{"tuple-field", func(c *SMTContext, s ast.ASTHandle) error { _, err := c.TupleField(s, 0); return err }},
	}
	for _, query := range queries {
		for _, input := range []ast.ASTHandle{{}, boolean, truth, foreign} {
			if err := query.call(f.c, input); err == nil {
				t.Fatalf("%s accepted null, wrong kind/theory or foreign sort", query.name)
			}
		}
	}
	list, err := f.c.ListSort("bounds-list", boolean)
	astTestMust(t, err)
	f.pin(list.Sort, nil)
	if _, err = f.c.DatatypeConstructor(list.Sort, 2); err == nil {
		t.Fatal("constructor index beyond source table")
	}
	if _, err = f.c.DatatypeRecognizer(list.Sort, 2); err == nil {
		t.Fatal("recognizer index beyond source table")
	}
	if _, err = f.c.DatatypeAccessor(list.Sort, 0, 0); err == nil {
		t.Fatal("zero-arity constructor accessor")
	}
	if _, err = f.c.DatatypeAccessor(list.Sort, 1, 2); err == nil {
		t.Fatal("accessor index beyond source arity")
	}
	for _, query := range queries[4:] {
		if err := query.call(f.c, list.Sort); err == nil {
			t.Fatal("recursive sort accepted as tuple", query.name)
		}
	}
	closed := newAPIInterruptFixture(t)
	astTestMust(t, closed.Close())
	for _, query := range queries {
		if err := query.call(closed, list.Sort); !errors.Is(err, ErrSMTContextOwner) {
			t.Fatal("closed-context datatype query", query.name, err)
		}
	}
}

func TestAPIDatatypeTypedNamesRejectForeignSymbols(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	other := newSMTAPIFixture(t, "ALL", nil)
	foreign, err := other.c.StringSymbol("foreign-datatype-name")
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	if _, err = f.c.TupleSortWithSymbol(foreign, nil, nil); err == nil {
		t.Fatal("tuple accepted foreign sort name")
	}
	if _, err = f.c.EnumerationSortWithSymbol(usymbol.Numerical(1), []usymbol.Symbol{foreign}); err == nil {
		t.Fatal("enumeration accepted foreign element name")
	}
	if _, err = f.c.ListSortWithSymbol(foreign, boolean); err == nil {
		t.Fatal("list accepted foreign sort name")
	}
	before, err := f.m.GetRefCount(boolean)
	astTestMust(t, err)
	if object, err := f.c.NewDatatypeConstructorObjectWithSymbols(ast.DatatypeConstructorDefinition{
		Name: usymbol.Numerical(3), Accessors: []ast.DatatypeAccessorDefinition{{Range: boolean}, {Name: foreign, Range: boolean}},
	}); object != nil || err == nil {
		t.Fatal("constructor accepted foreign field name", object, err)
	}
	after, err := f.m.GetRefCount(boolean)
	astTestMust(t, err)
	if after != before {
		t.Fatal("foreign field symbol refusal leaked earlier sort root", before, after)
	}
}

func TestAPITupleQueriesUseSourceStructuralPredicate(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	enumeration, err := f.c.EnumerationSort("singleton-tuple-view", []string{"only"})
	astTestMust(t, err)
	f.pin(enumeration.Sort, nil)
	// The source accepts any nonrecursive one-constructor datatype, including
	// an enumeration created through another public factory.
	constructor, err := f.c.TupleConstructor(enumeration.Sort)
	astTestMust(t, err)
	if constructor != enumeration.Constants[0] {
		t.Fatal("structural tuple constructor", constructor)
	}
	if count, err := f.c.TupleFieldCount(enumeration.Sort); err != nil || count != 0 {
		t.Fatal("structural zero-field tuple", count, err)
	}
	if _, err = f.c.TupleField(enumeration.Sort, 0); err == nil {
		t.Fatal("out-of-range singleton tuple field")
	}
}

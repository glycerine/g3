package ast

import (
	"bytes"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestASTDatatypeDefinitionBlockAndUtilities(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)

	evenName := astTestSymbol(t, m, "EvenList")
	oddName := astTestSymbol(t, m, "OddList")
	sorts, err := m.MkDatatypes([]DatatypeDefinition{
		{
			Name: evenName,
			Constructors: []DatatypeConstructorDefinition{
				{Name: astTestSymbol(t, m, "even-nil"), Recognizer: astTestSymbol(t, m, "is-even-nil")},
				{Name: astTestSymbol(t, m, "even-cons"), Recognizer: astTestSymbol(t, m, "is-even-cons"), Accessors: []DatatypeAccessorDefinition{
					{Name: astTestSymbol(t, m, "even-head"), Range: boolean},
					{Name: astTestSymbol(t, m, "even-tail"), Recursive: true, RecursiveIndex: 1},
				}},
			},
		},
		{
			Name: oddName,
			Constructors: []DatatypeConstructorDefinition{
				{Name: astTestSymbol(t, m, "odd-cons"), Recognizer: astTestSymbol(t, m, "is-odd-cons"), Accessors: []DatatypeAccessorDefinition{
					{Name: astTestSymbol(t, m, "odd-head"), Range: boolean},
					{Name: astTestSymbol(t, m, "odd-tail"), Recursive: true, RecursiveIndex: 0},
				}},
			},
		},
	})
	astTestMust(t, err)
	if len(sorts) != 2 {
		t.Fatal("datatype sort count", len(sorts))
	}
	siblings, err := m.DatatypeAreSiblings(sorts[0], sorts[1])
	astTestMust(t, err)
	if !siblings {
		t.Fatal("mutually recursive datatypes are not siblings")
	}
	count, err := m.DatatypeNumConstructors(sorts[0])
	astTestMust(t, err)
	if count != 2 {
		t.Fatal("constructor count", count)
	}
	constructors, err := m.DatatypeConstructors(sorts[0])
	astTestMust(t, err)
	if len(constructors) != 2 {
		t.Fatal("constructors", len(constructors))
	}
	index, matched, err := m.DatatypeConstructorIndex(constructors[1])
	astTestMust(t, err)
	if !matched || index != 1 {
		t.Fatal("constructor index", index, matched)
	}
	accessors, err := m.DatatypeConstructorAccessors(constructors[1])
	astTestMust(t, err)
	if len(accessors) != 2 {
		t.Fatal("accessors", len(accessors))
	}
	if fieldRange, e := m.GetRange(accessors[1]); e != nil || fieldRange != sorts[1] {
		t.Fatal("recursive accessor range", fieldRange, e)
	}
	recognizer, err := m.DatatypeConstructorRecognizer(constructors[1])
	astTestMust(t, err)
	if original, matched, e := m.DatatypeRecognizerConstructor(recognizer); e != nil || !matched || original != constructors[1] {
		t.Fatal("recognizer constructor", original, matched, e)
	}
	if got, matched, e := m.DatatypeRecognizerConstructorIndex(recognizer); e != nil || !matched || got != 1 {
		t.Fatal("recognizer constructor index", got, matched, e)
	}
	if owner, matched, e := m.DatatypeAccessorConstructor(accessors[0]); e != nil || !matched || owner != constructors[1] {
		t.Fatal("accessor constructor", owner, matched, e)
	}
	if enum, e := m.DatatypeIsEnumSort(sorts[0]); e != nil || enum {
		t.Fatal("recursive datatype classified as enum", enum, e)
	}
	if recursive, e := m.DatatypeIsRecursive(sorts[0]); e != nil || !recursive {
		t.Fatal("mutually recursive datatype", recursive, e)
	}
	if size, e := m.GetSortSize(sorts[0]); e != nil || !size.IsInfinite() {
		t.Fatal("recursive datatype cardinality", size, e)
	}
	nonRecursive, found, err := m.DatatypeNonRecursiveConstructor(sorts[0])
	astTestMust(t, err)
	if !found {
		t.Fatal("missing non-recursive constructor")
	}
	if got, matched, e := m.DatatypeConstructorIndex(nonRecursive); e != nil || !matched || got != 0 {
		t.Fatal("non-recursive constructor", got, matched, e)
	}
	declared, err := m.DatatypeIsDeclared(sorts[0])
	astTestMust(t, err)
	if !declared {
		t.Fatal("installed datatype is not declared")
	}
	var display bytes.Buffer
	astTestMust(t, m.DisplayDatatype(sorts[0], &display))
	wantDisplay := "EvenList where\nEvenList =\n  even-nil :: \n  even-cons :: (even-head: Bool) (even-tail: OddList) \nOddList =\n  odd-cons :: (odd-head: Bool) (odd-tail: EvenList) \n"
	if display.String() != wantDisplay {
		t.Fatalf("datatype display\nwant: %q\n got: %q", wantDisplay, display.String())
	}
	descriptions, err := m.DatatypeDefinitions(sorts[0])
	astTestMust(t, err)
	if len(descriptions) != 2 || descriptions[0].Sort != sorts[0] || len(descriptions[0].Constructors) != 2 || descriptions[0].Constructors[1].Accessors[1].Range != sorts[1] {
		t.Fatal("datatype definition descriptions", descriptions)
	}
	var names ASTBuiltinNames
	astTestMust(t, m.GetDatatypeOpNames(&names, usymbol.Symbol{}))
	if items := names.Items(); len(items) != 2 || items[0].Kind != ASTDatatypeIs || items[1].Kind != ASTDatatypeUpdateField {
		t.Fatal("datatype operation names", items)
	}
}

func TestASTDatatypeDefinitionEnumAndWellFoundedRefusal(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)

	color, err := m.MkDatatypes([]DatatypeDefinition{{
		Name: astTestSymbol(t, m, "Color"),
		Constructors: []DatatypeConstructorDefinition{
			{Name: astTestSymbol(t, m, "red"), Recognizer: astTestSymbol(t, m, "is-red")},
			{Name: astTestSymbol(t, m, "blue"), Recognizer: astTestSymbol(t, m, "is-blue")},
		},
	}})
	astTestMust(t, err)
	if enum, e := m.DatatypeIsEnumSort(color[0]); e != nil || !enum {
		t.Fatal("enum datatype", enum, e)
	}
	if size, e := m.GetSortSize(color[0]); e != nil || !size.IsFinite() {
		t.Fatal("enum cardinality kind", size, e)
	} else if count, e := size.Size(); e != nil || count != 2 {
		t.Fatal("enum cardinality", count, e)
	}
	if recursive, e := m.DatatypeIsRecursive(color[0]); e != nil || recursive {
		t.Fatal("enum recursion", recursive, e)
	}

	_, err = m.MkDatatypes([]DatatypeDefinition{{
		Name: astTestSymbol(t, m, "Bottom"),
		Constructors: []DatatypeConstructorDefinition{{
			Name: astTestSymbol(t, m, "bottom"), Recognizer: astTestSymbol(t, m, "is-bottom"),
			Accessors: []DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "next-bottom"), Recursive: true}},
		}},
	}})
	if err == nil {
		t.Fatal("accepted datatype without a finite constructor")
	}
}

func TestASTParametricDatatypeDefinitionInstantiation(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	formal, err := m.MkSort(astTestSymbol(t, m, "T"), ASTSortInfo{})
	astTestMust(t, err)
	astTestPin(t, m, formal)

	name := astTestSymbol(t, m, "BoxOf")
	_, err = m.MkDatatypes([]DatatypeDefinition{{
		Name:       name,
		Parameters: []ASTHandle{formal},
		Constructors: []DatatypeConstructorDefinition{{
			Name:       astTestSymbol(t, m, "box-of"),
			Recognizer: astTestSymbol(t, m, "is-box-of"),
			Accessors:  []DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "unbox-of"), Range: formal}},
		}},
	}})
	astTestMust(t, err)

	p0, err := m.NewParameterSymbol(name)
	astTestMust(t, err)
	p1, err := m.NewParameterAST(boolean)
	astTestMust(t, err)
	boxBool, err := m.MkFamilySort(family, ASTDatatypeSort, []ASTParameter{p0, p1})
	astTestMust(t, err)
	astTestMust(t, m.DelParameter(&p1))
	astTestMust(t, m.DelParameter(&p0))
	constructors, err := m.DatatypeConstructors(boxBool)
	astTestMust(t, err)
	if len(constructors) != 1 {
		t.Fatal("parametric constructors", len(constructors))
	}
	if domain, e := m.GetDomain(constructors[0], 0); e != nil || domain != boolean {
		t.Fatal("substituted constructor domain", domain, e)
	}
	accessors, err := m.DatatypeConstructorAccessors(constructors[0])
	astTestMust(t, err)
	if len(accessors) != 1 {
		t.Fatal("parametric accessors", len(accessors))
	}
	if fieldRange, e := m.GetRange(accessors[0]); e != nil || fieldRange != boolean {
		t.Fatal("substituted accessor range", fieldRange, e)
	}
	if count, e := m.DatatypeNumParameters(boxBool); e != nil || count != 1 {
		t.Fatal("datatype parameter count", count, e)
	}
	if parameter, e := m.DatatypeParameter(boxBool, 0); e != nil || parameter != boolean {
		t.Fatal("datatype parameter", parameter, e)
	}
	if size, e := m.GetSortSize(boxBool); e != nil || !size.IsFinite() {
		t.Fatal("parametric datatype cardinality kind", size, e)
	} else if count, e := size.Size(); e != nil || count != 2 {
		t.Fatal("parametric datatype cardinality", count, e)
	}
}

func TestASTDatatypeValueAndSomeValueCallbacks(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	nat, err := m.MkDatatypes([]DatatypeDefinition{{
		Name: astTestSymbol(t, m, "UnaryNat"),
		Constructors: []DatatypeConstructorDefinition{
			{Name: astTestSymbol(t, m, "zero"), Recognizer: astTestSymbol(t, m, "is-zero")},
			{Name: astTestSymbol(t, m, "succ"), Recognizer: astTestSymbol(t, m, "is-succ"), Accessors: []DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "pred"), Recursive: true}}},
		},
	}})
	astTestMust(t, err)
	constructors, err := m.DatatypeConstructors(nat[0])
	astTestMust(t, err)
	zero, err := m.MkApp(constructors[0], nil)
	astTestMust(t, err)
	succ, err := m.MkApp(constructors[1], []ASTHandle{zero})
	astTestMust(t, err)
	for _, value := range []ASTHandle{zero, succ} {
		if yes, e := m.IsValue(value); e != nil || !yes {
			t.Fatal("datatype value", yes, e)
		}
		if yes, e := m.IsUniqueValue(value); e != nil || !yes {
			t.Fatal("datatype unique value", yes, e)
		}
	}
	variable, err := m.MkVar(0, nat[0])
	astTestMust(t, err)
	nonValue, err := m.MkApp(constructors[1], []ASTHandle{variable})
	astTestMust(t, err)
	if yes, e := m.IsValue(nonValue); e != nil || yes {
		t.Fatal("symbolic constructor classified as value", yes, e)
	}
	some, err := m.GetSomeValue(nat[0])
	astTestMust(t, err)
	if yes, e := m.IsValue(some); e != nil || !yes {
		t.Fatal("datatype some value", yes, e)
	}
	if sort, e := m.GetSort(some); e != nil || sort != nat[0] {
		t.Fatal("datatype some value sort", sort, e)
	}
}

// AUDIT.md ast_03-F1: the accessor index is the field position in the
// constructor's current definition (datatype_rewriter::mk_app_core walks
// util::get_constructor_accessors), not the order in which accessor
// declarations were first created.
func TestASTDatatypeRedefinitionAccessorIndex(t *testing.T) {
	define := func(m *ASTManager, field ASTHandle, first, second string) []ASTHandle {
		t.Helper()
		sorts, err := m.MkDatatypes([]DatatypeDefinition{{
			Name: astTestSymbol(t, m, "P"),
			Constructors: []DatatypeConstructorDefinition{{
				Name: astTestSymbol(t, m, "mk"), Recognizer: astTestSymbol(t, m, "is-mk"),
				Accessors: []DatatypeAccessorDefinition{
					{Name: astTestSymbol(t, m, first), Range: field},
					{Name: astTestSymbol(t, m, second), Range: field},
				},
			}},
		}})
		astTestMust(t, err)
		return sorts
	}
	constructorAndAccessors := func(m *ASTManager, sort ASTHandle) (ASTHandle, []ASTHandle) {
		t.Helper()
		constructors, err := m.DatatypeConstructors(sort)
		astTestMust(t, err)
		accessors, err := m.DatatypeConstructorAccessors(constructors[0])
		astTestMust(t, err)
		return constructors[0], accessors
	}

	m := astTestManager(t)
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	sorts := define(m, boolean, "a", "b")
	mk, accessors := constructorAndAccessors(m, sorts[0])
	if index, matched, e := m.DatatypeAccessorIndex(accessors[0], mk); e != nil || !matched || index != 0 {
		t.Fatal("first definition accessor index", index, matched, e)
	}
	sorts = define(m, boolean, "b", "a")
	mk, accessors = constructorAndAccessors(m, sorts[0])
	// accessors[1] is a in the new definition.
	if index, matched, e := m.DatatypeAccessorIndex(accessors[1], mk); e != nil || !matched || index != 1 {
		t.Fatal("redefined accessor index", index, matched, e)
	}
	if index, matched, e := m.DatatypeAccessorIndex(accessors[0], mk); e != nil || !matched || index != 0 {
		t.Fatal("redefined accessor index of b", index, matched, e)
	}

	// An accessor declaration created on its own before the constructor's
	// accessors does not take position 0.
	m = astTestManager(t)
	family, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	boolean, err = m.MkBoolSort()
	astTestMust(t, err)
	sorts = define(m, boolean, "a", "b")
	bParameter, err := m.NewParameterSymbol(astTestSymbol(t, m, "b"))
	astTestMust(t, err)
	mkParameter, err := m.NewParameterSymbol(astTestSymbol(t, m, "mk"))
	astTestMust(t, err)
	early, err := m.MkFamilyFuncDecl(family, ASTDatatypeAccessor, []ASTParameter{bParameter, mkParameter}, []ASTHandle{sorts[0]}, boolean)
	astTestMust(t, err)
	astTestPin(t, m, early)
	astTestMust(t, m.DelParameter(&mkParameter))
	astTestMust(t, m.DelParameter(&bParameter))
	mk, accessors = constructorAndAccessors(m, sorts[0])
	if accessors[1] != early {
		t.Fatal("accessor declaration is not hash-consed", accessors[1], early)
	}
	if index, matched, e := m.DatatypeAccessorIndex(accessors[0], mk); e != nil || !matched || index != 0 {
		t.Fatal("accessor a index", index, matched, e)
	}
	if index, matched, e := m.DatatypeAccessorIndex(early, mk); e != nil || !matched || index != 1 {
		t.Fatal("accessor b index", index, matched, e)
	}
}

// AUDIT.md ast_03-F2: plugin::mk_datatypes resets the plugin's util for
// each redeclared datatype, so get_non_rec_constructor does not return a
// constructor of the previous definition and m_start advances.
func TestASTDatatypeRedefinitionResetsUtility(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	define := func(first, second string) ASTHandle {
		t.Helper()
		sorts, err := m.MkDatatypes([]DatatypeDefinition{{
			Name: astTestSymbol(t, m, "L"),
			Constructors: []DatatypeConstructorDefinition{
				{Name: astTestSymbol(t, m, first), Recognizer: astTestSymbol(t, m, "is-"+first)},
				{Name: astTestSymbol(t, m, second), Recognizer: astTestSymbol(t, m, "is-"+second)},
			},
		}})
		astTestMust(t, err)
		return sorts[0]
	}
	sort := define("a", "b")
	constructor, found, err := m.DatatypeNonRecursiveConstructor(sort)
	astTestMust(t, err)
	if !found {
		t.Fatal("no constructor for the first definition")
	}
	firstName, err := m.GetASTName(constructor)
	astTestMust(t, err)
	if text, _ := m.Symbols.String(firstName); text != "b" {
		t.Fatal("first definition constructor", text)
	}
	sort = define("c", "d")
	constructor, found, err = m.DatatypeNonRecursiveConstructor(sort)
	astTestMust(t, err)
	if !found {
		t.Fatal("no constructor for the redefinition")
	}
	// start: 0 -> 1 (first call), 2 (reset), 3 (second call): index 1.
	name, err := m.GetASTName(constructor)
	astTestMust(t, err)
	if text, _ := m.Symbols.String(name); text != "d" {
		t.Fatal("redefined constructor", text)
	}
}

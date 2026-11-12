package protomodel

// Source: value_factory.h get_some_values; theory_seq_empty.h:77-95;
// theory_str.h:53-60. These tests own the real source output references.

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestFactoryOutputsDatalogConstructionFailurePrefixes(t *testing.T) {
	for _, warm := range []bool{false, true} {
		t.Run(map[bool]string{false: "cold", true: "registered-zero"}[warm], func(t *testing.T) {
			m, f, sort := datalogFactoryFixture(t, 1)
			if warm {
				_, err := f.SomeValue(sort)
				protoMust(t, err)
			}
			first, second := protoTestOutputRefs(t, m)
			truth, err := m.MkTrue()
			protoMust(t, err)
			falsity, err := m.MkFalse()
			protoMust(t, err)
			protoMust(t, m.SetRef(first, truth))
			protoMust(t, m.SetRef(second, falsity))
			ok, err := f.SomeValues(sort, first, second)
			if ok || !errors.Is(err, ast.ErrASTDomain) {
				t.Fatal("source numeral one must fail", ok, err)
			}
			a, err := m.RefGet(first)
			protoMust(t, err)
			datalogFactoryNumber(t, m, a, 0)
			b, err := m.RefGet(second)
			protoMust(t, err)
			if warm {
				if b != a {
					t.Fatal("source first writes zero to second before attempting one", a, b)
				}
			} else if b != falsity {
				t.Fatal("failed numeral construction must leave old second output", b, falsity)
			}
		})
	}
}

func TestFactoryOutputsFirstAssignmentStopsSecondConstruction(t *testing.T) {
	m, f, sort := datalogFactoryFixture(t, 8)
	_, second := protoTestOutputRefs(t, m)
	old, err := m.MkTrue()
	protoMust(t, err)
	protoMust(t, m.SetRef(second, old))
	ok, err := f.SomeValues(sort, ast.ASTRef{}, second)
	if ok || !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("invalid first output", ok, err)
	}
	if len(f.sets[sort].values) != 1 {
		t.Fatal("constructed second after first assignment failed", len(f.sets[sort].values))
	}
	b, err := m.RefGet(second)
	protoMust(t, err)
	if b != old {
		t.Fatal("second output changed after first failure")
	}
}

func TestFactoryOutputsFrozenUniverseRefusalLeavesBothOwners(t *testing.T) {
	m, p := protoFixture(t)
	sort, err := m.MkUninterpretedSort(protoSymbol(t, m, "single-output-universe"), nil)
	protoMust(t, err)
	protoPin(t, m, sort)
	_, err = p.SomeValue(sort)
	protoMust(t, err)
	protoMust(t, p.FreezeUniverse(sort))
	first, second := protoTestOutputRefs(t, m)
	a, err := m.MkTrue()
	protoMust(t, err)
	b, err := m.MkFalse()
	protoMust(t, err)
	protoMust(t, m.SetRef(first, a))
	protoMust(t, m.SetRef(second, b))
	beforeA, err := m.GetRefCount(a)
	protoMust(t, err)
	beforeB, err := m.GetRefCount(b)
	protoMust(t, err)
	ok, err := p.SomeValues(sort, first, second)
	protoMust(t, err)
	actualA, err := m.RefGet(first)
	protoMust(t, err)
	actualB, err := m.RefGet(second)
	protoMust(t, err)
	afterA, err := m.GetRefCount(a)
	protoMust(t, err)
	afterB, err := m.GetRefCount(b)
	protoMust(t, err)
	if ok || actualA != a || actualB != b || beforeA != afterA || beforeB != afterB {
		t.Fatal("refusal must not clear or replace caller outputs")
	}
}

func TestFactoryOutputsSequenceForwardsElementFailure(t *testing.T) {
	m, p := protoFixture(t)
	df, err := NewDatalogFactory(m)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(df))
	element, err := m.MkDatalogFiniteSort(protoSymbol(t, m, "sequence-singleton"), 1)
	protoMust(t, err)
	family, err := m.RegisterSequencePlugin()
	protoMust(t, err)
	sf, err := NewSequenceFactory(p, family)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(sf))
	parameter, err := m.NewParameterAST(element)
	protoMust(t, err)
	sequence, err := m.MkFamilySort(family, 0, []ast.ASTParameter{parameter})
	protoMust(t, err)
	protoMust(t, m.DelParameter(&parameter))
	protoMust(t, err)
	protoPin(t, m, sequence)
	first, second := protoTestOutputRefs(t, m)
	old, err := m.MkTrue()
	protoMust(t, err)
	protoMust(t, m.SetRef(second, old))
	ok, err := sf.SomeValues(sequence, first, second)
	if ok || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("element numeral one must fail", ok, err)
	}
	a, err := m.RefGet(first)
	protoMust(t, err)
	datalogFactoryNumber(t, m, a, 0)
	b, err := m.RefGet(second)
	protoMust(t, err)
	if b != old {
		t.Fatal("sequence staged outputs instead of forwarding caller references")
	}
}

func TestFactoryOutputsAliasFollowsSourceAssignments(t *testing.T) {
	d, _ := scalarFactoryDebugActions(t, "T")
	m, f, sort := datalogFactoryFixture(t, 8)
	shared, _ := protoTestOutputRefs(t, m)
	// On the missing-set branch there is no final SASSERT. Both assignments
	// target the same reference and the last publication wins.
	ok, err := f.SomeValues(sort, shared, shared)
	protoMust(t, err)
	if !ok {
		t.Fatal("cold alias refused")
	}
	value, err := m.RefGet(shared)
	protoMust(t, err)
	datalogFactoryNumber(t, m, value, 1)
	ok, err = f.SomeValues(sort, shared, shared)
	if diagnostic.DebugBuild {
		var ex z3exception.DefaultException
		if ok || !errors.As(err, &ex) || ex.Message() != "assertion violation" {
			t.Fatal("registered alias must reach source v1 != v2 assertion", ok, err)
		}
	} else {
		protoMust(t, err)
		if !ok {
			t.Fatal("erased assertion refused alias")
		}
	}
	protoMust(t, d.EnableAssertions(false))
	ok, err = f.SomeValues(sort, shared, shared)
	protoMust(t, err)
	if !ok {
		t.Fatal("disabled alias assertion refused")
	}
	value, err = m.RefGet(shared)
	protoMust(t, err)
	// The warm pair is *begin(), *++begin() of the obj_hashtable; the alias
	// keeps the second publication.
	if value != f.sets[sort].table.FirstKeys(2)[1] {
		t.Fatal("alias lost the second table value")
	}
}

func TestFactoryOutputsStringOwnedByCallerAndSurviveFactoryClose(t *testing.T) {
	m, p := protoFixture(t)
	family, err := m.RegisterSequencePlugin()
	protoMust(t, err)
	f, err := NewStringFactory(p, family)
	protoMust(t, err)
	empty, err := m.MkString("")
	protoMust(t, err)
	sort, err := m.GetSort(empty)
	protoMust(t, err)
	first, second := protoTestOutputRefs(t, m)
	before := m.Stats().LiveReferences
	ok, err := f.SomeValues(sort, first, second)
	protoMust(t, err)
	if !ok || m.Stats().LiveReferences != before {
		t.Fatal("factory allocated a separate reference lifetime cache")
	}
	a, err := m.RefGet(first)
	protoMust(t, err)
	b, err := m.RefGet(second)
	protoMust(t, err)
	countA, err := m.GetRefCount(a)
	protoMust(t, err)
	countB, err := m.GetRefCount(b)
	protoMust(t, err)
	if countA != 1 || countB != 1 {
		t.Fatal("outputs must own exactly their acquisitions", countA, countB)
	}
	protoMust(t, f.Close())
	for i, value := range []ast.ASTHandle{a, b} {
		text, constant, err := m.SequenceStringValue(value)
		protoMust(t, err)
		if !constant || text != []string{"value 1", "value 2"}[i] {
			t.Fatal("factory close destroyed caller output", text)
		}
	}
	protoMust(t, m.ResetRef(first))
	protoMust(t, m.ResetRef(second))
	if _, err := m.GetRefCount(a); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("factory kept first output after caller released it", err)
	}
	if _, err := m.GetRefCount(b); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("factory kept second output after caller released it", err)
	}
}

func TestFactoryOutputsSequenceUnsupportedSortReportsSourceMacro(t *testing.T) {
	d, report := scalarFactoryDebugActions(t, "T")
	protoMust(t, d.EnableAssertions(false))
	m, p := protoFixture(t)
	family, err := m.RegisterSequencePlugin()
	protoMust(t, err)
	f, err := NewSequenceFactory(p, family)
	protoMust(t, err)
	defer func() { protoMust(t, f.Close()) }()
	empty, err := m.MkString("")
	protoMust(t, err)
	re, err := m.MkSequenceToRegex(empty)
	protoMust(t, err)
	sort, err := m.GetSort(re)
	protoMust(t, err)
	first, second := protoTestOutputRefs(t, m)
	truth, err := m.MkTrue()
	protoMust(t, err)
	protoMust(t, m.SetRef(first, truth))
	protoMust(t, m.SetRef(second, truth))
	ok, err := f.SomeValues(sort, first, second)
	if ok {
		t.Fatal("unsupported sequence sort produced a pair")
	}
	if diagnostic.DebugBuild {
		var ex z3exception.DefaultException
		if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
			t.Fatal("source unsupported-sort macro", err)
		}
	} else if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
		t.Fatal("native unsupported-sort macro", err)
	}
	a, err := m.RefGet(first)
	protoMust(t, err)
	b, err := m.RefGet(second)
	protoMust(t, err)
	if a != truth || b != truth {
		t.Fatal("unsupported-sort macro changed outputs")
	}
	data, err := os.ReadFile(report)
	protoMust(t, err)
	if !strings.Contains(string(data), "NOT IMPLEMENTED YET!\n") {
		t.Fatal("unsupported-sort source message", string(data))
	}
	if diagnostic.DebugBuild && !strings.Contains(string(data), "File: src/smt/theory_seq_empty.h\nLine: 94\n") {
		t.Fatal("unsupported-sort source location", string(data))
	}
}

func TestFactoryOutputsArrayConsumesAndReleasesRangeLocals(t *testing.T) {
	m, p := protoFixture(t)
	family, err := m.RegisterSequencePlugin()
	protoMust(t, err)
	strings, err := NewStringFactory(p, family)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(strings))
	arrays, err := NewArrayFactory(p)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(arrays))
	empty, err := m.MkString("")
	protoMust(t, err)
	rangeSort, err := m.GetSort(empty)
	protoMust(t, err)
	boolean, err := m.MkBoolSort()
	protoMust(t, err)
	sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), rangeSort)
	protoMust(t, err)
	protoPin(t, m, sort)
	first, second := protoTestOutputRefs(t, m)
	ok, err := p.SomeValues(sort, first, second)
	protoMust(t, err)
	if !ok {
		t.Fatal("array pair refused")
	}
	texts := map[string]bool{}
	for i, ref := range []ast.ASTRef{first, second} {
		value, err := m.RefGet(ref)
		protoMust(t, err)
		declaration, err := m.AsArrayFunction(value)
		protoMust(t, err)
		interpretation, err := p.Core().FunctionInterpretation(declaration)
		protoMust(t, err)
		if interpretation == nil || interpretation.NumEntries() != 1 {
			t.Fatal("range locals did not survive array construction")
		}
		entry, err := interpretation.Entry(0)
		protoMust(t, err)
		result, err := entry.Result()
		protoMust(t, err)
		text, found, err := m.SequenceStringValue(result)
		protoMust(t, err)
		// The pair follows the array value_set obj_hashtable order.
		if !found || text != "value 1" && text != "value 2" || texts[text] {
			t.Fatal("array entry lost its range output", i, text)
		}
		texts[text] = true
		count, err := m.GetRefCount(result)
		protoMust(t, err)
		if count != 1 {
			t.Fatal("range reference must transfer to the entry and release the source local", i, count)
		}
	}
}

func TestFactoryOutputsDatatypeSingletonPublishesBeforeFalse(t *testing.T) {
	for _, warm := range []bool{false, true} {
		t.Run(map[bool]string{false: "cold", true: "one-registered"}[warm], func(t *testing.T) {
			m, p := protoFixture(t)
			sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: protoSymbol(t, m, "SinglePairResult"), Constructors: []ast.DatatypeConstructorDefinition{{Name: protoSymbol(t, m, "only-pair-result"), Recognizer: protoSymbol(t, m, "is-only-pair-result")}}}})
			protoMust(t, err)
			sort := sorts[0]
			protoPin(t, m, sort)
			f, err := NewDatatypeFactory(p)
			protoMust(t, err)
			protoMust(t, p.RegisterFactory(f))
			if warm {
				_, err = f.SomeValue(sort)
				protoMust(t, err)
			}
			first, second := protoTestOutputRefs(t, m)
			truth, err := m.MkTrue()
			protoMust(t, err)
			protoMust(t, m.SetRef(first, truth))
			protoMust(t, m.SetRef(second, truth))
			ok, err := f.SomeValues(sort, first, second)
			protoMust(t, err)
			a, err := m.RefGet(first)
			protoMust(t, err)
			b, err := m.RefGet(second)
			protoMust(t, err)
			if ok || a.IsNull() || !b.IsNull() {
				t.Fatal("source struct_factory publishes first and null second before returning false", ok, a, b)
			}
			got, err := m.GetSort(a)
			protoMust(t, err)
			if got != sort {
				t.Fatal("first constructor was not published")
			}
		})
	}
}

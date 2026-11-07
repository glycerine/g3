package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTArraySortDeclarationsOwnedAndScoped(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	indexSort, _ := smtArrayQueueIndices(t, m, "sort-registration-", 1)
	baseSort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean, indexSort}), boolean)
	astTestMust(t, err)
	childSort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort, boolean}), boolean)
	astTestMust(t, err)
	base, err := m.MkConst(astTestSymbol(t, m, "sort-registration-base"), baseSort)
	astTestMust(t, err)
	child, err := m.MkConst(astTestSymbol(t, m, "sort-registration-child"), childSort)
	astTestMust(t, err)
	astTestPin(t, m, child)
	astTestMust(t, state.Internalize(base, false))
	declarations := append([]ast.ASTHandle(nil), theory.sortDeclarations[baseSort]...)
	if len(declarations) != 2 || len(theory.sortTrail) != 1 || theory.CanPropagate() {
		t.Fatal("array sort did not eagerly register both witness declarations")
	}
	for i, declaration := range declarations {
		astTestPin(t, m, declaration)
		rangeSort, err := m.GetRange(declaration)
		astTestMust(t, err)
		want := boolean
		if i == 1 {
			want = indexSort
		}
		if rangeSort != want {
			t.Fatal("witness declarations lost dimension order")
		}
		count, err := m.GetRefCount(declaration)
		astTestMust(t, err)
		if count != 2 {
			t.Fatal("declaration must have its cache owner and test pin", count)
		}
	}
	astTestMust(t, theory.registerArraySort(baseSort))
	if len(theory.sortTrail) != 1 {
		t.Fatal("duplicate sort registration added another scope entry")
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(child, false))
	childDeclarations := append([]ast.ASTHandle(nil), theory.sortDeclarations[childSort]...)
	if len(childDeclarations) != 2 || len(theory.sortTrail) != 2 {
		t.Fatal("child sort declaration registration")
	}
	for _, declaration := range childDeclarations {
		astTestPin(t, m, declaration)
	}
	astTestMust(t, state.PopScopes(1))
	if len(theory.sortTrail) != 1 || len(theory.sortDeclarations) != 1 {
		t.Fatal("child sort survived scope pop")
	}
	for _, declaration := range childDeclarations {
		count, err := m.GetRefCount(declaration)
		astTestMust(t, err)
		if count != 1 {
			t.Fatal("scope pop retained child declaration ownership", count)
		}
	}
	astTestMust(t, theory.Close())
	for _, declaration := range declarations {
		count, err := m.GetRefCount(declaration)
		astTestMust(t, err)
		if count != 1 {
			t.Fatal("theory teardown retained base declaration ownership", count)
		}
	}
	if len(theory.sortDeclarations) != 0 || len(theory.sortTrail) != 0 {
		t.Fatal("theory teardown retained the sort cache")
	}
}

func TestSMTFullArrayExplicitExtensionalityOriginalFingerprint(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean, boolean}), boolean)
	astTestMust(t, err)
	arrays := make([]ast.ASTHandle, 3)
	for i, name := range []string{"explicit-ext-a", "explicit-ext-b", "explicit-ext-c"} {
		arrays[i], err = m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(arrays[i], false))
	}
	declarations := theory.sortDeclarations[sort]
	witness, err := m.MkApp(declarations[0], arrays[:2])
	astTestMust(t, err)
	astTestMust(t, state.Internalize(witness, true))
	node := state.context.enodes.get(witness)
	variable, found := state.BoolVar(witness)
	if node == nil || !theory.IsAttachedToVariable(node) || !found {
		t.Fatal("full theory did not internalize explicit Boolean witness")
	}
	data, err := state.BooleanData(variable)
	astTestMust(t, err)
	if !data.IsENode() || data.Theory() != family || len(theory.extensionalityTodo) != 1 || theory.statistics.extensionality != 1 {
		t.Fatal("explicit witness enode flags or extensionality scheduling")
	}
	first := state.context.enodes.get(arrays[0])
	second := state.context.enodes.get(arrays[1])
	third := state.context.enodes.get(arrays[2])
	astTestMust(t, state.PushScope())
	astTestMust(t, state.addEquality(first, third, NewSMTEqualityAxiom()))
	if first.root != third {
		t.Fatal("fixture did not move the original fingerprint root")
	}
	otherWitness, err := m.MkApp(declarations[1], arrays[:2])
	astTestMust(t, err)
	astTestMust(t, state.Internalize(otherWitness, true))
	if len(theory.extensionalityTrail) != 1 || len(theory.extensionalityTodo) != 1 || theory.statistics.extensionality != 1 {
		t.Fatal("root change forgot the original witness fingerprint")
	}
	if !theory.extensionalityFingerprints[smtArrayEnodePair{first, second}] {
		t.Fatal("original fingerprint was replaced by mutable representatives")
	}
	astTestMust(t, state.PopScopes(1))
	if first.root != first || len(theory.extensionalityTrail) != 1 || theory.CanPropagate() {
		t.Fatal("scope pop lost the base fingerprint or retained pending witness work")
	}
}

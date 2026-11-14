package model_test

// Source: src/model/model.cpp:28-40,60-66,111-150;
// src/model/model_core.h:31-45,70-76; src/tactic/model_converter.cpp:176-181.

import (
	"errors"
	"github.com/glycerine/g3/pkg/tactic"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestModelDerivedConstructorAndVirtualReferenceDestruction(t *testing.T) {
	m := astTestManager(t)
	before := m.BudgetField.Used()
	md, e := model.NewModel(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, md.Close()) })
	if md.CoreField.Derived != md || md.CoreField.References != 0 || md.CoreField.Functions.Len() != 0 || md.Universes.Len() != 0 {
		t.Fatal("source member construction")
	}
	n, e := md.NumUninterpretedSorts()
	satOK(t, e)
	if n != 0 {
		t.Fatal(n)
	}
	var ref tactic.ModelRef
	satOK(t, ref.Set(&md.CoreField))
	satOK(t, ref.Close())
	if !md.CoreField.Closed || !md.Closed {
		t.Fatal("base final DEC did not invoke derived destructor")
	}
	if m.BudgetField.Used() != before {
		t.Fatal("derived members leaked", before, m.BudgetField.Used())
	}
}

func TestModelDerivedConstructorHeaderRefusalUnwinds(t *testing.T) {
	m := astTestManager(t)
	before := m.BudgetField.Used()
	// Native dictionaries do not reserve through the numeric arena. The real
	// model owner header remains a fallible construction boundary.
	m.BudgetField.FailAfter(0)
	t.Cleanup(func() { m.BudgetField.FailAfter(-1) })
	md, e := model.NewModel(m)
	if !errors.Is(e, arena.ErrInjectedFailure) || md != nil {
		t.Fatal("model owner header refusal", md, e)
	}
	if m.BudgetField.Used() != before {
		t.Fatal("constructed member prefix leaked", before, m.BudgetField.Used())
	}
	m.BudgetField.FailAfter(-1)
}

func TestModelDerivedUniverseUpdatePreservesLiteralAppendAndCounts(t *testing.T) {
	m := astTestManager(t)
	md, e := model.NewModel(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, md.Close()) })
	s, e := m.MkUninterpretedSort(astTestSymbol(t, m, "update-u"), nil)
	satOK(t, e)
	v, e := m.GetSomeValue(s)
	satOK(t, e)
	satOK(t, md.RegisterUniverse(s, []ast.ASTHandle{v}))
	// A caller reference makes the source's stale physical prefix observable
	// through destruction. The source update DEC's the old graph, then APPENDs
	// without reset; destruction therefore also consumes this caller reference.
	satOK(t, m.IncRef(v))
	satOK(t, md.RegisterUniverse(s, []ast.ASTHandle{v}))
	u, e := md.Universe(s)
	satOK(t, e)
	if !reflect.DeepEqual(u, []ast.ASTHandle{v, v}) {
		t.Fatal("source update was replaced with reset", u)
	}
	refs, e := m.GetRefCount(v)
	satOK(t, e)
	if refs != 2 {
		t.Fatal("invented retention of source stale prefix", refs)
	}
	n, e := md.NumUninterpretedSorts()
	satOK(t, e)
	if n != 1 {
		t.Fatal("source update duplicated sort", n)
	}
	satOK(t, md.Close())
	if m.IsLiveHandle(v) {
		t.Fatal("literal source destructor did not consume both physical entries")
	}
}

func TestModelDerivedUniverseOwnershipCopyAndInsertionOrder(t *testing.T) {
	m := astTestManager(t)
	md, e := model.NewModel(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, md.Close()) })
	sorts := []ast.ASTHandle{}
	values := []ast.ASTHandle{}
	for _, name := range []string{"u-first", "u-second", "u-third"} {
		s, e := m.MkUninterpretedSort(astTestSymbol(t, m, name), nil)
		satOK(t, e)
		v, e := m.GetSomeValue(s)
		satOK(t, e)
		satOK(t, md.RegisterUniverse(s, []ast.ASTHandle{v}))
		sorts = append(sorts, s)
		values = append(values, v)
	}
	n, e := md.NumUninterpretedSorts()
	satOK(t, e)
	if n != 3 {
		t.Fatal(n)
	}
	for i, s := range sorts {
		got, e := md.UninterpretedSort(uint32(i))
		satOK(t, e)
		if got != s {
			t.Fatal("source sort insertion order")
		}
		u, e := md.Universe(s)
		satOK(t, e)
		if !reflect.DeepEqual(u, []ast.ASTHandle{values[i]}) {
			t.Fatal(u)
		}
	}
	copy, e := model.NewModel(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, copy.Close()) })
	satOK(t, copy.CopyUniverseInterpretations(md))
	satOK(t, md.Close())
	for _, v := range values {
		_, e = m.GetKind(v)
		satOK(t, e)
	}
	var out tactic.ModelRef
	satOK(t, out.Set(&copy.CoreField))
	satOK(t, out.Close())
	for i, v := range values {
		if m.IsLiveHandle(v) {
			t.Fatal("universe value leaked", i)
		}
		if m.IsLiveHandle(sorts[i]) {
			t.Fatal("universe sort leaked", i)
		}
	}
}

func TestModelDerivedEmptyUniverseUpdateAndHashGrowth(t *testing.T) {
	m := astTestManager(t)
	md, e := model.NewModel(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, md.Close()) })
	for i := 0; i < 40; i++ {
		s, e := m.MkUninterpretedSort(astTestSymbol(t, m, string(rune('a'+i))), nil)
		satOK(t, e)
		satOK(t, md.RegisterUniverse(s, nil))
		yes, e := md.HasUninterpretedSort(s)
		satOK(t, e)
		if !yes {
			t.Fatal("empty universe is present")
		}
		v, e := m.GetSomeValue(s)
		satOK(t, e)
		satOK(t, md.RegisterUniverse(s, []ast.ASTHandle{v}))
		u, e := md.Universe(s)
		satOK(t, e)
		if len(u) != 1 || u[0] != v {
			t.Fatal("source append to empty universe")
		}
	}
	n, e := md.NumUninterpretedSorts()
	satOK(t, e)
	if n != 40 || md.Universes.Len() != 40 {
		t.Fatal("replacement duplicated sort or lost a universe", n)
	}
}

func TestModelConverterToModelOriginalAllocationAndApplication(t *testing.T) {
	m := astTestManager(t)
	old, e := model.NewModel(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, old.Close()) })
	var out tactic.ModelRef
	satOK(t, out.Set(&old.CoreField))
	t.Cleanup(func() { satOK(t, out.Close()) })
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	satOK(t, tactic.ModelConverterToModel(m, nil, &out))
	got, e := out.Get()
	satOK(t, e)
	if got != &old.CoreField || m.BudgetField.Used() != before {
		t.Fatal("null converter changed result")
	}
	m.BudgetField.FailAfter(-1)
	calls := []string{}
	leaf := &modelConverterTestLeaf{name: "inspect-empty", calls: &calls}
	leaf.run = func(r *tactic.ModelRef, idx uint32) error {
		md, e := r.Get()
		if e != nil {
			return e
		}
		if idx != 0 || md.Derived == nil || md == &old.CoreField || !old.Closed || md.Constants.Len() != 0 || md.Functions.Len() != 0 {
			t.Fatal("source allocation/assignment must precede call", idx)
		}
		return nil
	}
	satOK(t, tactic.ModelConverterToModel(m, leaf, &out))
	got, e = out.Get()
	satOK(t, e)
	if got.Derived == nil {
		t.Fatal("converter received a substitute core")
	}
	first := got.Derived
	sentinel := errors.New("source converter application failed")
	failing := &modelConverterTestLeaf{name: "fail-after-assignment", calls: &calls, fail: sentinel}
	if e = tactic.ModelConverterToModel(m, failing, &out); e != sentinel {
		t.Fatal("converter error lost", e)
	}
	got, e = out.Get()
	satOK(t, e)
	if !first.Closed || got.Derived == nil || got.Derived == first || got.References != 1 {
		t.Fatal("source assigned model lost on converter error")
	}
	stored, e := model.NewModel(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, stored.Close()) })
	c, e := tactic.NewStoredModel(&stored.CoreField)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, c.Close()) })
	transient := got.Derived
	satOK(t, tactic.ModelConverterToModel(m, c, &out))
	if !transient.Closed {
		t.Fatal("previous model not released")
	}
	got, e = out.Get()
	satOK(t, e)
	if got != &stored.CoreField {
		t.Fatal("stored converter application")
	}
	// Retire the retrieved reference before the fixture's direct owner cleanup.
	satOK(t, out.Close())
}

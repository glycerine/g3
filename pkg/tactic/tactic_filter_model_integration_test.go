package tactic_test

// Source: src/tactic/filter_model_converter.cpp:23-57;
// src/tactic/filter_model_converter.h:26-49.
// Licensed under the MIT license. Source author: Leonardo de Moura.

import (
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestTacticFilterModelRealSATBindingsUniverseAndReplacement(t *testing.T) {
	m := astTestManager(t)
	s, _, _ := satSolverCheckFixture(t)
	tr, e := sattactic.NewGoalTranslator(m, s)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	b, e := m.MkBoolSort()
	satOK(t, e)
	var atoms, decls []ast.ASTHandle
	for _, name := range []string{"filter-p", "filter-q", "filter-aux"} {
		a, e := m.MkConst(astTestSymbol(t, m, name), b)
		satOK(t, e)
		d, e := m.GetDecl(a)
		satOK(t, e)
		atoms, decls = append(atoms, a), append(decls, d)
	}
	nq, e := m.MkNot(atoms[1])
	satOK(t, e)
	satOK(t, tr.Compile([]ast.ASTHandle{atoms[0], nq, atoms[2]}))
	r, e := s.Check(nil)
	satOK(t, e)
	if r != lbool.True {
		t.Fatal(r)
	}
	values, e := s.AppendModel(nil)
	satOK(t, e)
	old, e := sattactic.BuildBooleanModel(tr, values)
	satOK(t, e)
	var out tactic.ModelRef
	t.Cleanup(func() { satOK(t, out.Close()) })
	satOK(t, out.Set(old))
	sort, e := m.MkUninterpretedSort(astTestSymbol(t, m, "filter-universe"), nil)
	satOK(t, e)
	v, e := m.GetSomeValue(sort)
	satOK(t, e)
	satOK(t, old.Derived.RegisterUniverse(sort, []ast.ASTHandle{v}))
	var wantOrder []ast.ASTHandle
	for _, d := range old.ConstantDeclarations {
		if d != decls[2] {
			wantOrder = append(wantOrder, d)
		}
	}
	c, e := tactic.NewFilterModel(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, c.Close()) })
	satOK(t, c.Insert(decls[2]))
	satOK(t, tr.Close())
	satOK(t, s.Close())
	satOK(t, c.ApplyModel(&out, 77))
	got, e := out.Get()
	satOK(t, e)
	if got == old || !old.Closed {
		t.Fatal("source replacement did not retire old model")
	}
	if !reflect.DeepEqual(got.ConstantDeclarations, wantOrder) {
		t.Fatal("source constant insertion order", got.ConstantDeclarations, wantOrder)
	}
	removed, e := got.ConstantInterpretation(decls[2])
	satOK(t, e)
	if !removed.IsNull() {
		t.Fatal("auxiliary retained in filtered model")
	}
	for i, expected := range []bool{true, false} {
		value, e := got.ConstantInterpretation(decls[i])
		satOK(t, e)
		truth, e := m.MkTrue()
		satOK(t, e)
		falsity, e := m.MkFalse()
		satOK(t, e)
		if (value == truth) != expected || (value == falsity) == expected {
			t.Fatal("filtered model violates original assertion", i)
		}
	}
	u, e := got.Derived.Universe(sort)
	satOK(t, e)
	if !reflect.DeepEqual(u, []ast.ASTHandle{v}) {
		t.Fatal("source universe copy", u)
	}
	satOK(t, c.Close())
	if m.IsLiveHandle(decls[2]) {
		t.Fatal("filtered auxiliary declaration leaked")
	}
	_, e = m.GetKind(v)
	satOK(t, e)
	satOK(t, out.Close())
	if m.IsLiveHandle(v) {
		t.Fatal("copied universe value leaked")
	}
}

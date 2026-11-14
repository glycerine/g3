package tactic_test

// Source: src/tactic/model_converter.cpp:127-151,164-174;
// src/util/buffer.h:25-48,68-82,94-96; src/util/vector.h:443-447.
// Licensed under the MIT license. Source author: Leonardo de Moura.

import (
	"github.com/glycerine/g3/pkg/model"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"

	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestTacticStoredModelRealSATBindingOwnershipAndReplacement(t *testing.T) {
	m := astTestManager(t)
	s, _, _ := satSolverCheckFixture(t)
	tr, e := sattactic.NewGoalTranslator(m, s)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	sort, e := m.MkBoolSort()
	satOK(t, e)
	p, e := m.MkConst(astTestSymbol(t, m, "stored-p"), sort)
	satOK(t, e)
	satOK(t, tr.Compile([]ast.ASTHandle{p}))
	result, e := s.Check(nil)
	satOK(t, e)
	if result != lbool.True {
		t.Fatal(result)
	}
	values, e := s.AppendModel(nil)
	satOK(t, e)
	md, e := sattactic.BuildBooleanModel(tr, values)
	satOK(t, e)
	d, e := m.GetDecl(p)
	satOK(t, e)
	c, e := tactic.NewStoredModel(md)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, c.Close()) })
	if md.References != 1 {
		t.Fatal("wrapper did not retain model", md.References)
	}
	satOK(t, tr.Close())
	satOK(t, s.Close())
	if m.IsLiveHandle(p) {
		t.Fatal("test unexpectedly retained the atom")
	}
	old, e := model.NewModelCore(m)
	satOK(t, e)
	var out tactic.ModelRef
	t.Cleanup(func() { satOK(t, out.Close()); satOK(t, old.Close()) })
	satOK(t, out.Set(old))
	satOK(t, c.ApplyModel(&out, ^uint32(0)))
	if !old.Closed {
		t.Fatal("replacement leaked old model")
	}
	satOK(t, c.ApplyUnindexed(&out))
	got, e := out.Get()
	satOK(t, e)
	if got != md || md.References != 2 {
		t.Fatal("stored assignment/alias ownership", got, md.References)
	}
	truth, e := m.MkTrue()
	satOK(t, e)
	binding, e := got.ConstantInterpretation(d)
	satOK(t, e)
	if binding != truth {
		t.Fatal("retrieved model violates original assertion")
	}
	satOK(t, c.Cancel())
	satOK(t, out.Close())
	if md.Closed || md.References != 1 {
		t.Fatal("stored model lost before wrapper")
	}
	satOK(t, c.IncRef())
	satOK(t, c.DecRef())
	if !md.Closed {
		t.Fatal("wrapper final DEC did not retire stored model")
	}
	if m.IsLiveHandle(d) {
		t.Fatal("stored declaration leaked")
	}
}

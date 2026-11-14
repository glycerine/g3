package ast

// Source: src/ast/ast.cpp:1798-1844; src/ast/ast.h:1479-1480,1600.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestASTDebugRefCountActualDeletedIDMapAndSkippedRecycle(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.EnableDebugRefCount())
	bs, e := m.MkBoolSort()
	astTestMust(t, e)
	v, e := m.MkVar(17, bs)
	astTestMust(t, e)
	id, e := m.GetASTID(v)
	astTestMust(t, e)
	astTestMust(t, m.IncRef(v))
	astTestMust(t, m.DecRef(v))
	if _, found := m.DebugFreeIndices[id]; !found || len(m.DebugFreeIndices) != 1 {
		t.Fatal("deleted ID was not inserted before node free")
	}
	if m.IsLiveHandle(v) {
		t.Fatal("deleted node remained callable")
	}
	next, e := m.MkVar(18, bs)
	astTestMust(t, e)
	nextID, e := m.GetASTID(next)
	astTestMust(t, e)
	if nextID == id {
		t.Fatal("debug deletion recycled actual source ID")
	}
	astTestMust(t, m.IncRef(next))
	astTestMust(t, m.DecRef(next))
	if _, found := m.DebugFreeIndices[nextID]; !found || len(m.DebugFreeIndices) != 2 {
		t.Fatal("actual second deleted ID absent")
	}
}

func TestASTDebugRefCountSkipsOwnedInfoDelButStillDecrementsNodeChildren(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.EnableDebugRefCount())
	bs, e := m.MkBoolSort()
	astTestMust(t, e)
	info, e := m.NewFuncDeclInfo(NullFamilyID, 42, nil, ASTFuncFlags{}, false)
	astTestMust(t, e)
	d, e := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "debug-info"), []ASTHandle{bs}, bs, info)
	astTestMust(t, e)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	owned := m.declAt(d.slot).info
	record, e := m.funcInfo(owned)
	astTestMust(t, e)
	before, e := m.GetRefCount(bs)
	astTestMust(t, e)
	astTestMust(t, m.IncRef(d))
	astTestMust(t, m.DecRef(d))
	after, e := m.GetRefCount(bs)
	astTestMust(t, e)
	if before != after+2 {
		t.Fatal("domain/range children were not decremented", before, after)
	}
	retained, e := m.funcInfo(owned)
	astTestMust(t, e)
	if retained != record || !retained.live || !retained.retainedByDebug {
		t.Fatal("source debug branch destroyed rather than retained actual owned info")
	}
}

func TestASTSourceFinalizeRetainsDebugInfoParametersForExactOwningClose(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	b := arena.NewBudget(1 << 30)
	m, e := NewSourceASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b}, nil)
	astTestMust(t, e)
	astTestMust(t, m.EnableDebugRefCount())
	bs, e := m.MkBoolSort()
	astTestMust(t, e)
	parameter, e := m.NewParameterInt(37)
	astTestMust(t, e)
	info, e := m.NewFuncDeclInfo(NullFamilyID, 42, []ASTParameter{parameter}, ASTFuncFlags{}, false)
	astTestMust(t, e)
	d, e := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "retained-parameter"), nil, bs, info)
	astTestMust(t, e)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	astTestMust(t, m.DelParameter(&parameter))
	owned := m.declAt(d.slot).info
	astTestMust(t, m.IncRef(d))
	astTestMust(t, m.DecRef(d))
	astTestMust(t, m.SourceFinalize())
	if _, e = m.MkBoolSort(); e == nil {
		t.Fatal("source-ended manager remained callable")
	}
	retained, e := m.funcInfo(owned)
	astTestMust(t, e)
	token, e := m.infoParameter(retained, 0)
	astTestMust(t, e)
	value, e := m.parameter(token)
	astTestMust(t, e)
	if value.integer != 37 || !retained.retainedByDebug {
		t.Fatal("source DT erased independently allocated retained info/parameter", retained, value)
	}
	if b.Used() != m.Stats().AllocatedBytes {
		t.Fatal("source-completed named owning graph before ACK", b.Used(), m.Stats())
	}
	astTestMust(t, m.Close())
	if b.Used() != 0 {
		t.Fatal("owning close did not drain exact source-leaked info", b.Used())
	}
}

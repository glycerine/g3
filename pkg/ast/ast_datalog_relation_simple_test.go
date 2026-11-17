package ast

import (
	"errors"
	"testing"
)

func TestASTDatalogOriginalSimpleRelationDeclarations(t *testing.T) {
	m := astTestManager(t)
	a, e := m.MkDatalogFiniteSort(astTestSymbol(t, m, "dl-a"), 3)
	astTestMust(t, e)
	b, e := m.MkDatalogFiniteSort(astTestSymbol(t, m, "dl-b"), 5)
	astTestMust(t, e)
	relation, e := m.MkDatalogRelationSort([]ASTHandle{a, b})
	astTestMust(t, e)
	astTestPin(t, m, relation)
	one, e := m.MkDatalogNumeral(1, a)
	astTestMust(t, e)
	two, e := m.MkDatalogNumeral(2, b)
	astTestMust(t, e)
	empty, e := m.MkDatalogEmpty(relation)
	astTestMust(t, e)
	astTestPin(t, m, empty)
	stored, e := m.MkDatalogStore(empty, one, two)
	astTestMust(t, e)
	astTestPin(t, m, stored)
	for _, tc := range []struct {
		kind int32
		mk   func() (ASTHandle, error)
	}{
		{DatalogOpIsEmpty, func() (ASTHandle, error) { return m.MkDatalogIsEmpty(empty) }},
		{DatalogOpUnion, func() (ASTHandle, error) { return m.MkDatalogUnion(empty, stored) }},
		{DatalogOpWiden, func() (ASTHandle, error) { return m.MkDatalogWiden(empty, stored) }},
		{DatalogOpComplement, func() (ASTHandle, error) { return m.mkDatalogRelationApp(DatalogOpComplement, []ASTHandle{empty}) }},
		{DatalogOpClone, func() (ASTHandle, error) { return m.mkDatalogRelationApp(DatalogOpClone, []ASTHandle{empty}) }},
	} {
		h, e := tc.mk()
		astTestMust(t, e)
		astTestPin(t, m, h)
		kind, e := m.GetDeclKind(h)
		astTestMust(t, e)
		if kind != tc.kind {
			t.Fatal("relation operation kind", kind, tc.kind)
		}
		got, e := m.GetSort(h)
		astTestMust(t, e)
		expected := relation
		if tc.kind == DatalogOpIsEmpty {
			expected, e = m.MkBoolSort()
			astTestMust(t, e)
		}
		if got != expected {
			t.Fatal("relation operation range", tc.kind, got, expected)
		}
	}
	// This preserves the source's observable OP_RA_SELECT guard: mk_store_select
	// changes its local result to Bool before applying is_rel_sort to that value.
	if _, e = m.MkDatalogSelect(empty, one, two); !errors.Is(e, ErrASTDomain) {
		t.Fatal("source select relation-sort refusal", e)
	}
	if _, e = m.MkDatalogStore(empty, two, one); !errors.Is(e, ErrASTDomain) {
		t.Fatal("store column mismatch", e)
	}
	if _, e = m.MkDatalogUnion(empty, one); !errors.Is(e, ErrASTDomain) {
		t.Fatal("union relation mismatch", e)
	}
}

func TestASTDatalogOriginalLessEqualAndDeclarationGuards(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkDatalogFiniteSort(astTestSymbol(t, m, "dl-order"), 4)
	astTestMust(t, e)
	one, e := m.MkDatalogNumeral(1, sort)
	astTestMust(t, e)
	two, e := m.MkDatalogNumeral(2, sort)
	astTestMust(t, e)
	le, e := m.MkDatalogLE(one, two)
	astTestMust(t, e)
	astTestPin(t, m, le)
	kind, e := m.GetDeclKind(le)
	astTestMust(t, e)
	if kind != ASTOpNot {
		t.Fatal("less-equal outer kind", kind)
	}
	lt, e := m.GetArg(le, 0)
	astTestMust(t, e)
	kind, e = m.GetDeclKind(lt)
	astTestMust(t, e)
	if kind != DatalogOpLT {
		t.Fatal("less-equal comparison kind", kind)
	}
	left, e := m.GetArg(lt, 0)
	astTestMust(t, e)
	right, e := m.GetArg(lt, 1)
	astTestMust(t, e)
	if left != two || right != one {
		t.Fatal("less-equal reversed arguments", left, right, two, one)
	}
	other, e := m.MkDatalogFiniteSort(astTestSymbol(t, m, "dl-other"), 4)
	astTestMust(t, e)
	foreign, e := m.MkDatalogNumeral(1, other)
	astTestMust(t, e)
	if _, e = m.MkDatalogLT(one, foreign); !errors.Is(e, ErrASTDomain) {
		t.Fatal("comparison identical-sort guard", e)
	}
}

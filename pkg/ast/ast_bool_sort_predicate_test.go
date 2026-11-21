package ast

import (
	"errors"
	"testing"
)

func TestASTHasBoolSortGenerationAndKind(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	nonbool, err := m.MkSort(astTestSymbol(t, m, "Other"), ASTSortInfo{})
	astTestMust(t, err)
	astTestPin(t, m, nonbool)
	for _, item := range []struct {
		sort   ASTHandle
		expect bool
	}{{boolean, true}, {nonbool, false}} {
		x, err := m.MkVar(10, item.sort)
		astTestMust(t, err)
		astTestPin(t, m, x)
		a, err := m.MkConst(astTestSymbol(t, m, "owned"), item.sort)
		astTestMust(t, err)
		astTestPin(t, m, a)
		for _, expr := range []ASTHandle{x, a} {
			got, err := m.HasBoolSort(expr)
			if err != nil || got != item.expect {
				t.Fatalf("sort of %v: %v, %v", expr, got, err)
			}
		}
	}
	if _, err := m.HasBoolSort(boolean); !errors.Is(err, ErrASTDomain) {
		t.Fatal("sort used as an expression", err)
	}
	other := astTestManager(t)
	foreign, err := other.MkTrue()
	astTestMust(t, err)
	stale, err := m.MkVar(791, boolean)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(stale))
	astTestMust(t, m.DecRef(stale))
	replacement, err := m.MkVar(792, nonbool)
	astTestMust(t, err)
	astTestPin(t, m, replacement)
	for _, bad := range []ASTHandle{stale, foreign, {}} {
		if _, err := m.HasBoolSort(bad); !errors.Is(err, ErrASTHandle) {
			t.Fatal("invalid handle", bad, err)
		}
	}
}

//go:build g3debug

package tactic

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGoalRefFinalDecrementConsumedBeforeStoppedFormulaRelease(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.EnableDebugRefCount())
	boolSort, e := m.MkBoolSort()
	astTestMust(t, e)
	formula, e := m.MkConst(astTestSymbol(t, m, "stopped-goal-formula"), boolSort)
	astTestMust(t, e)
	g, e := NewGoal(m, false, true, false)
	astTestMust(t, e)
	astTestMust(t, g.Assert(formula))
	id, e := m.GetASTID(formula)
	astTestMust(t, e)
	astTestMust(t, g.IncRef()) // the retained goal_ref's acquired reference

	// The checked AST guard stops the actual formula release after its
	// reference has been consumed by the goal destructor.
	m.DebugFreeIndices[id] = struct{}{}
	consumed, e := g.DecRefConsumed()
	if !consumed || !errors.Is(e, ast.ErrASTDomain) || g.RefCount != 0 || g.Closed || int(g.forms.size()) != 0 {
		t.Fatalf("stopped final goal reference: consumed=%v error=%v count=%d closed=%v forms=%d", consumed, e, g.RefCount, g.Closed, int(g.forms.size()))
	}
	delete(m.DebugFreeIndices, id)
	astTestMust(t, g.Close())
	if !g.Closed || g.RefCount != 0 || g.ownerBytes != 0 {
		t.Fatalf("owning cleanup replayed final decrement: closed=%v count=%d bytes=%d", g.Closed, g.RefCount, g.ownerBytes)
	}
	consumed, e = g.DecRefConsumed()
	if consumed || e == nil || g.RefCount != 0 {
		t.Fatalf("closed goal decremented again: consumed=%v error=%v count=%d", consumed, e, g.RefCount)
	}
}

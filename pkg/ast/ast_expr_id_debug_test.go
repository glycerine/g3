package ast

import "testing"

func TestSetNextExprIDAvoidsExistingASTIDs(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(73, sort)
	astTestMust(t, err)
	astTestPin(t, m, variable)
	occupied, err := m.GetASTID(variable)
	astTestMust(t, err)
	astTestMust(t, m.SetNextExprID(occupied))
	if got := m.ExprIDs.Range(); got <= occupied {
		t.Fatalf("next ID %d collides with live ID %d", got, occupied)
	}
	astTestMust(t, m.SetNextExprID(100000))
	if got := m.ExprIDs.Range(); got != 100000 {
		t.Fatalf("next ID %d, want 100000", got)
	}
}

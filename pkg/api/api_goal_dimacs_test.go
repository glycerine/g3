package api

import "testing"

func TestAPIGoalDIMACSExportsCNFWithoutTrailingNewline(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-cnf-p", boolean))
	q := f.pin(f.c.Constant("api-cnf-q", boolean))
	notQ := f.pin(f.c.Not(q))
	clause := f.pin(f.c.Or(p, notQ))
	astTestMust(t, goal.Assert(clause))
	text, err := goal.DIMACS(false)
	astTestMust(t, err)
	if want := "p cnf 2 1\n1 -2 0"; text != want {
		t.Fatalf("DIMACS = %q; want %q", text, want)
	}
}

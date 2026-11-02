package sat

import "testing"

func TestSATCutSimplifierSnapshotDoesNotMutateSolverClauses(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 3; i++ {
		_, err := s.MkVar(false, true)
		satOK(t, err)
	}
	r, e := s.Clauses.MkClause([]Literal{0, 2, 4}, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Lifecycle.pushClause(false, r); e != nil {
		t.Fatal(e)
	}
	satOK(t, s.Lifecycle.attachClause(r))
	x, e := newSATCutSimplifierLive(s)
	if e != nil {
		t.Fatal(e)
	}
	defer x.Close()
	before := len(s.Lifecycle.Original)
	cs, e := x.snapshotClauses()
	if e != nil || len(cs) != before || len(s.Lifecycle.Original) != before {
		t.Fatal(e, len(cs), before)
	}
	cs[0].Literals[0] = 99
	l, e := s.Clauses.Literal(s.Lifecycle.Original[0], 0)
	if e != nil || l == 99 {
		t.Fatal("snapshot aliased", e, l)
	}
}

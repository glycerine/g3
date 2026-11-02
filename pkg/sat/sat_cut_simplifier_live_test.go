package sat

import "testing"

func TestSATCutSimplifierLiveExtractsSolverLUT(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 3; i++ {
		_, err := s.MkVar(false, true)
		satOK(t, err)
	}
	clauses := satLUTDefinitionClauses(2, func(x uint32) bool { return x&3 == 3 })
	for _, c := range clauses {
		r, e := s.Clauses.MkClause(c.Literals, false)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Lifecycle.pushClause(false, r); e != nil {
			t.Fatal(e)
		}
		satOK(t, s.Lifecycle.attachClause(r))
	}
	x, e := newSATCutSimplifierLive(s)
	if e != nil {
		t.Fatal(e)
	}
	defer x.Close()
	if e = x.ExtractLUTs(); e != nil {
		t.Fatal(e)
	}
	c, ok := findSATAIGCut(x.aig.cuts[2], []uint32{0, 1})
	if !ok || c.Table() != 8 {
		t.Fatal(c, ok)
	}
	if x.stats.numCalls != 1 || x.stats.extractedLUTs == 0 {
		t.Fatal(x.stats)
	}
}

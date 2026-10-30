package sat

import "testing"

func TestSATLiveCutSimplifierExtractsAIGFromOwnedSolverState(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 3; i++ {
		if _, e := s.MkVar(false, true); e != nil {
			t.Fatal(e)
		}
	}
	h, a, b := MakeLiteral(2, false), MakeLiteral(0, false), MakeLiteral(1, false)
	if e := s.MkClause([]Literal{h.Negated(), a}); e != nil {
		t.Fatal(e)
	}
	if e := s.MkClause([]Literal{h.Negated(), b}); e != nil {
		t.Fatal(e)
	}
	r, e := s.Clauses.MkClause([]Literal{h, a.Negated(), b.Negated()}, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Lifecycle.pushClause(false, r); e != nil {
		t.Fatal(e)
	}
	x, e := newSATCutSimplifierLive(s)
	if e != nil {
		t.Fatal(e)
	}
	defer x.Close()
	if e = x.ExtractAIGs(); e != nil {
		t.Fatal(e)
	}
	cuts, e := x.aig.Compute()
	if e != nil {
		t.Fatal(e)
	}
	cut, ok := findSATAIGCut(cuts[2], []uint32{0, 1})
	if !ok || cut.Table() != 8 || x.stats.extractedAnds != 1 {
		t.Fatal(cut, ok, x.stats)
	}
}

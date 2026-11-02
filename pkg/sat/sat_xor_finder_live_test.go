package sat

import "testing"

func TestSATLiveCutSimplifierExtractsXORFromOwnedClauses(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 3; i++ {
		if _, e := s.MkVar(false, true); e != nil {
			t.Fatal(e)
		}
	}
	clauses := satLUTDefinitionClauses(2, func(v uint32) bool { return (v&1)^((v>>1)&1) != 0 })
	for _, cl := range clauses {
		r, e := s.Clauses.MkClause(cl.Literals, false)
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
	if e = x.ExtractXORs(); e != nil {
		t.Fatal(e)
	}
	for v := BoolVar(0); v < 3; v++ {
		external, err := s.Variables.IsExternal(v)
		if err != nil || !external {
			t.Fatal("xor variable not external", v, external, err)
		}
	}
	cuts, e := x.aig.Compute()
	if e != nil {
		t.Fatal(e)
	}
	c, ok := findSATAIGCut(cuts[2], []uint32{0, 1})
	if !ok || c.Table() != 6 || x.stats.extractedXors != 1 {
		t.Fatal(c, ok, x.stats)
	}
}

package sat

import "testing"

func TestSATCutSimplifierLiveGateAPI(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	x, e := newSATCutSimplifierLive(s)
	if e != nil {
		t.Fatal(e)
	}
	defer x.Close()
	if e = x.AddOr(MakeLiteral(2, false), []Literal{MakeLiteral(0, false), MakeLiteral(1, false)}); e != nil {
		t.Fatal(e)
	}
	cuts, e := x.aig.Compute()
	if e != nil {
		t.Fatal(e)
	}
	c, ok := findSATAIGCut(cuts[2], []uint32{0, 1})
	if !ok || c.Table() != 14 {
		t.Fatal(c, ok, c.Table())
	}
}

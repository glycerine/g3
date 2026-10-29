package sat

import "testing"

func TestSATLUTFinderPublishesIntoAIGCuts(t *testing.T) {
	a := testSATAIG(t)
	defer a.Close()
	clauses := satLUTDefinitionClauses(2, func(x uint32) bool { return (x&1)^((x>>1)&1) != 0 })
	remaining, e := a.FindAndAddLUTs(clauses, nil)
	if e != nil || len(remaining) != 0 {
		t.Fatal(e, len(remaining))
	}
	cuts, e := a.Compute()
	if e != nil {
		t.Fatal(e)
	}
	c, ok := findSATAIGCut(cuts[2], []uint32{0, 1})
	if !ok || c.Table() != 6 {
		t.Fatal(c, ok, c.Table())
	}
}

func TestSATLUTFinderUsesLearnedSupportClauses(t *testing.T) {
	clauses := satLUTDefinitionClauses(2, func(x uint32) bool { return x&3 == 3 })
	clauses[1].Learned = true
	calls := 0
	remaining, e := newSATLUTFinder(func(uint64, []BoolVar, BoolVar) { calls++ }).Run(clauses, nil)
	if e != nil || calls != 1 || len(remaining) != 0 {
		t.Fatal(e, calls, len(remaining))
	}
}

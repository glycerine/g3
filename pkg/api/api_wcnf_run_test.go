package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextRunWCNFReturnsOwnedResults(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	r, err := f.c.RunWCNFString("p wcnf 2 2 8\n8 1 0\n3 -1 2 0\n", &WCNFRunOptions{DisplayStatistics: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != lbool.True || r.Summary != (cmd_context.WCNFLoadSummary{Variables: 2, HardClauses: 1, SoftClauses: 1}) {
		t.Fatal(r.Status, r.Summary)
	}
	if !strings.HasPrefix(r.Output, "sat\n") || !strings.Contains(r.Output, ":total-time") || !strings.HasSuffix(r.Output, "   0\n") {
		t.Fatal(r.Output)
	}
	r.Document.Clauses[0].Literals[0].Variable = 99
	if r.Elapsed < 0 {
		t.Fatal(r.Elapsed)
	}
}

func TestSMTContextRunWCNFGuards(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	if _, err := f.c.RunWCNF(nil, nil); err != ErrSMTContextOwner {
		t.Fatal(err)
	}
	if _, err := f.c.RunWCNFString("bad", nil); err == nil {
		t.Fatal("accepted malformed input")
	}
}

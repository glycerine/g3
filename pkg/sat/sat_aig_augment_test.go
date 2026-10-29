package sat

import (
	"github.com/glycerine/g3/pkg/arena"
	"math"
	"testing"
)

func testSATAIG(t *testing.T) *satAIGCuts {
	t.Helper()
	a, e := newSATAIGCuts(arena.NewBudget(math.MaxUint64))
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func findSATAIGCut(cs *satCutSet, ids []uint32) (satCut, bool) {
	for _, c := range cs.cuts {
		if c.DomainEqual(testSATCut(ids, 0)) {
			return c, true
		}
	}
	return satCut{}, false
}

func TestSATAIGCutsComputesAndXorITEAndLUT(t *testing.T) {
	a := testSATAIG(t)
	defer a.Close()
	if e := a.AddNode(MakeLiteral(2, false), satAIGAnd, []Literal{MakeLiteral(0, false), MakeLiteral(1, false)}); e != nil {
		t.Fatal(e)
	}
	if e := a.AddNode(MakeLiteral(3, false), satAIGXor, []Literal{MakeLiteral(0, false), MakeLiteral(1, false)}); e != nil {
		t.Fatal(e)
	}
	if e := a.AddNode(MakeLiteral(4, false), satAIGITE, []Literal{MakeLiteral(0, false), MakeLiteral(1, false), MakeLiteral(3, false)}); e != nil {
		t.Fatal(e)
	}
	if e := a.AddLUT(5, 0x9, []BoolVar{0, 1}); e != nil {
		t.Fatal(e)
	}
	cuts, e := a.Compute()
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		v     uint32
		ids   []uint32
		table uint64
	}{{2, []uint32{0, 1}, 8}, {3, []uint32{0, 1}, 6}, {5, []uint32{0, 1}, 9}} {
		c, ok := findSATAIGCut(cuts[tc.v], tc.ids)
		if !ok || c.Table() != tc.table {
			t.Fatalf("v%d cut=%v found=%v table=%x", tc.v, c, ok, c.Table())
		}
	}
	c, ok := findSATAIGCut(cuts[4], []uint32{0, 1})
	if !ok {
		t.Fatal("ITE did not compose through XOR cut")
	}
	for x := uint32(0); x < 4; x++ {
		cond, th := x&1, (x>>1)&1
		xor := cond ^ th
		want := cond&th | (cond^1)&xor
		if uint32(c.Table()>>x&1) != want {
			t.Fatalf("ITE assignment %d table %x", x, c.Table())
		}
	}
	values, e := a.Simulate(3)
	if e != nil || len(values) != 6 {
		t.Fatal(e, len(values))
	}
}

func TestSATAIGCutsNaryComposition(t *testing.T) {
	a := testSATAIG(t)
	defer a.Close()
	args := []Literal{MakeLiteral(0, false), MakeLiteral(1, false), MakeLiteral(2, false)}
	if e := a.AddNode(MakeLiteral(3, false), satAIGAnd, args); e != nil {
		t.Fatal(e)
	}
	cuts, e := a.Compute()
	if e != nil {
		t.Fatal(e)
	}
	c, ok := findSATAIGCut(cuts[3], []uint32{0, 1, 2})
	if !ok || c.Table() != 0x80 {
		t.Fatal(c, ok, c.Table())
	}
}

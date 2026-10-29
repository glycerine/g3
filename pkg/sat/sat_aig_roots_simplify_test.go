package sat

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"math"
	"testing"
)

func TestSATAIGRootFlushAndDontCareSimplify(t *testing.T) {
	a := testSATAIG(t)
	defer a.Close()
	if e := a.AddCut(2, 0b1010, []BoolVar{0, 1}); e != nil {
		t.Fatal(e)
	}
	if e := a.Simplify(); e != nil {
		t.Fatal(e)
	}
	c, ok := findSATAIGCut(a.cuts[2], []uint32{0})
	if !ok || c.Table() != 2 {
		t.Fatal("simplified cut", c, ok)
	}
	if e := a.SetRoot(2, MakeLiteral(0, false)); e != nil {
		t.Fatal(e)
	}
	if _, e := a.Compute(); e != nil {
		t.Fatal(e)
	}
	if len(a.nodes[2]) != 0 || a.cuts[2].Size() != 0 {
		t.Fatal("rooted node retained", a.nodes[2], a.cuts[2].cuts)
	}
}

func TestSATAIGCutsCloseAndCopiedOwner(t *testing.T) {
	b := arena.NewBudget(math.MaxUint64)
	a, e := newSATAIGCuts(b)
	if e != nil {
		t.Fatal(e)
	}
	if e = a.AddVar(3); e != nil {
		t.Fatal(e)
	}
	copyA := *a
	if e = copyA.AddVar(4); e != ErrSATOwner {
		t.Fatal("copied owner", e)
	}
	if e = a.Close(); e != nil || b.Used() != 0 {
		t.Fatal(e, b.Used())
	}
}

func TestSATAIGReserveFailureRollsBackNewVariablePrefix(t *testing.T) {
	b := arena.NewBudget(math.MaxUint64)
	a, e := newSATAIGCuts(b)
	if e != nil {
		t.Fatal(e)
	}
	header := b.Used()
	b.FailAfter(2)
	if e = a.AddVar(3); !errors.Is(e, arena.ErrInjectedFailure) || len(a.nodes) != 0 || len(a.cuts) != 0 || b.Used() != header {
		t.Fatal("reserve failure prefix", e, len(a.nodes), len(a.cuts), b.Used(), header)
	}
	b.FailAfter(-1)
	if e = a.AddVar(0); e != nil {
		t.Fatal(e)
	}
	if e = a.Close(); e != nil || b.Used() != 0 {
		t.Fatal(e, b.Used())
	}
}

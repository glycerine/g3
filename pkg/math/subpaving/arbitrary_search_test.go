package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestArbitrarySearchBreadthFirstLimitAndChildCancellation(t *testing.T) {
	f := newArbitraryClauseFixtureWithPolicy(t, 0, true)
	c := f.context
	x := f.y
	out, err := c.DefinePolynomial(&f.five, []numeral.ArbitraryFloat{f.four}, []uint32{x})
	if err != nil {
		t.Fatal(err)
	}
	var one numeral.ArbitraryFloat
	if err := f.numbers.SetInt32(&one, 1); err != nil {
		t.Fatal(err)
	}
	lower, err := c.NewInequality(x, &one, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.numbers.Del(&one); err != nil {
		t.Fatal(err)
	}
	upper, err := c.NewInequality(x, &f.five, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddUnitClause(lower, true); err != nil {
		t.Fatal(err)
	}
	if err := c.AddUnitClause(upper, true); err != nil {
		t.Fatal(err)
	}
	for _, direction := range []bool{true, false} {
		a, err := c.NewInequality(f.x, &f.four, direction, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddUnitClause(a, true); err != nil {
			t.Fatal(err)
		}
	}
	c.maxDepth = 1
	c.maxNodes = 2
	if err := c.Search(); err != nil {
		t.Fatal(err)
	}
	if c.numSplits != 1 || len(c.nodes) != 3 || c.leafTail != c.nodes[1] || c.root.queuedLeaf {
		t.Fatal("root split must precede max-node stop")
	}
	left, right := c.nodes[1], c.nodes[2]
	core, err := f.numbers.BorrowCore()
	if err != nil {
		t.Fatal(err)
	}
	value := func(b *ArbitraryBound) float64 {
		t.Helper()
		if b == nil {
			t.Fatal("missing search bound")
		}
		v, err := core.ToDouble(&b.value)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if value(left.upper[x]) != 2 || value(right.lower[x]) != 3 ||
		value(c.root.lower[out]) != 4 || value(c.root.upper[out]) != 20 {
		t.Fatal("integer midpoint and root polynomial bounds")
	}
	c.maxNodes = 3
	c.limit.Limit, c.limit.Steps = 2, 0
	if err := c.Search(); !errors.Is(err, ErrArbitraryContextCanceled) || c.searchNode != nil || c.searchPhase != 0 || c.leafHead != right ||
		c.queueHead != 1 || len(c.queue) != 2 || left.queuedLeaf || !right.queuedLeaf {
		t.Fatal("canceled child must not retain the source-local selected node", err, c.searchPhase, c.queueHead, len(c.queue))
	}
	c.limit.Limit, c.limit.Steps = 100, 0
	if err := c.Search(); !errors.Is(err, arena.ErrInvalidSlot) || c.queueHead != 1 || len(c.queue) != 2 || c.leafHead != right {
		t.Fatal("later search must encounter the source queue-empty invariant", err, c.queueHead, len(c.queue))
	}
}

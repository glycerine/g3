package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestHardwareSearchBreadthFirstSplitLimitAndChildCancellation(t *testing.T) {
	m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	limit := &control.Reslimit{Limit: 1000}
	c, err := NewHardwareContext(m, limit, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	x, err := c.NewVariable(true)
	if err != nil {
		t.Fatal(err)
	}
	y, err := c.DefineMonomial([]HardwarePower{{Variable: x, Degree: 2}})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		value int32
		lower bool
	}{{0, true}, {4, false}} {
		var v numeral.HardwareFloat
		if err := m.SetInt32(&v, entry.value); err != nil {
			t.Fatal(err)
		}
		a, err := c.NewInequality(x, &v, entry.lower, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddUnitClause(a, true); err != nil {
			t.Fatal(err)
		}
	}
	c.maxDepth = 1
	if err := c.InitializeRoot(); err != nil {
		t.Fatal(err)
	}
	if c.root.lower[y] == nil || c.root.upper[y] == nil ||
		m.ToFloat64(&c.root.lower[y].value) != 0 || m.ToFloat64(&c.root.upper[y].value) != 16 {
		t.Fatal("root definition before search")
	}
	c.maxNodes = 2
	if err := c.Search(); err != nil {
		t.Fatal(err)
	}
	if c.numSplits != 1 || c.activeNodes != 3 || len(c.nodes) != 3 || c.leafTail != c.nodes[1] || c.root.queuedLeaf {
		t.Fatal("root split must precede max-node stop")
	}
	left, right := c.nodes[1], c.nodes[2]
	if m.ToFloat64(&left.upper[x].value) != 1 || m.ToFloat64(&right.lower[x].value) != 2 {
		t.Fatal("integer midpoint decisions")
	}
	c.maxNodes = 3
	limit.Steps, limit.Limit = 0, 2
	if err := c.Search(); !errors.Is(err, ErrHardwareContextCanceled) {
		t.Fatal("child queue checkpoint", err)
	}
	if c.searchNode != nil || c.searchPhase != 0 || c.queueHead != 1 || len(c.queue) != 2 || c.leafHead != right ||
		left.queuedLeaf || !right.queuedLeaf {
		t.Fatalf("canceled selected child state: pending=%p phase=%d head=%d queue=%d leaf=%p leftLeaf=%t rightLeaf=%t", c.searchNode, c.searchPhase, c.queueHead, len(c.queue), c.leafHead, left.queuedLeaf, right.queuedLeaf)
	}
	limit.Steps, limit.Limit = 0, 100
	if err := c.Search(); !errors.Is(err, arena.ErrInvalidSlot) || c.queueHead != 1 || len(c.queue) != 2 || c.leafHead != right {
		t.Fatal("later search must encounter the source queue-empty invariant", err, c.queueHead, len(c.queue))
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if !left.closed || !right.closed || !c.root.closed || c.activeNodes != 0 {
		t.Fatal("search tree closes children before root")
	}
}

package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestFastSearchBreadthFirstCancellationAndOwnedChildren(t *testing.T) {
	_, c, _, root := fastMonomialFixture(t, []bool{false})
	c.maxDepth = 1
	c.limit.Limit, c.limit.Steps = 1, 1
	err := c.Search()
	var source z3exception.DefaultException
	if !errors.Is(err, ErrFastContextCanceled) || !errors.As(err, &source) || source.Message() != "canceled" || len(c.leafQueue) != 1 || c.leafQueue[0] != root || c.searchNode != nil {
		t.Fatal("pre-selection checkpoint", err)
	}
	c.limit.Limit, c.limit.Steps = 1, 0
	if err := c.Search(); !errors.Is(err, ErrFastContextCanceled) || c.numSplits != 1 || len(c.leafQueue) != 2 || root.children != 2 || len(c.searchOwned) != 2 {
		t.Fatal("child queue retained on cancellation", err)
	}
	if c.leafQueue[0] != c.searchOwned[0] || c.leafQueue[1] != c.searchOwned[1] {
		t.Fatal("breadth-first child order")
	}
	c.limit.Limit, c.limit.Steps = 2, 1
	if err := c.Search(); !errors.Is(err, ErrFastContextCanceled) || c.searchNode != nil || c.searchPhase != 0 || len(c.queue) != 1 || len(c.leafQueue) != 1 || c.leafQueue[0] != c.searchOwned[1] {
		t.Fatal("selected child cancellation must preserve queue prefix without a resumable node", err)
	}
	c.limit.Limit, c.limit.Steps = 100, 0
	if err := c.Search(); !errors.Is(err, arena.ErrInvalidSlot) || len(c.queue) != 1 || len(c.leafQueue) != 1 || c.leafQueue[0] != c.searchOwned[1] {
		t.Fatal("later search must encounter the source queue-empty invariant", err, len(c.queue), len(c.leafQueue))
	}
	if err := c.Close(); err != nil || len(c.nodes) != 0 || len(c.searchOwned) != 0 {
		t.Fatal("owned child cleanup", err)
	}
}

func TestFastSearchMaxNodesStopsBeforeNextLeaf(t *testing.T) {
	_, c, _, root := fastMonomialFixture(t, []bool{false})
	c.maxDepth, c.maxNodes = 2, 1
	if err := c.Search(); err != nil || c.numSplits != 1 || len(c.leafQueue) != 2 || root.children != 2 {
		t.Fatal("source node-count stop", err)
	}
}

func TestFastSearchRetainsRefusedSplitChildrenForOwnerClose(t *testing.T) {
	m, c, v, root := fastMonomialFixture(t, []bool{false})
	if err := m.SetInt32(v, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateBound(root, 0, v, true, false); err != nil {
		t.Fatal(err)
	}
	if err := m.Next(v); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateBound(root, 0, v, false, false); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateQueued(root); err != nil {
		t.Fatal(err)
	}
	if err := c.Search(); !errors.Is(err, ErrFastSplitNoInterior) || !errors.Is(err, ErrSubpavingFailure) || len(c.searchOwned) != 2 || root.children != 2 {
		t.Fatal("source split refusal ownership", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal("refused search cleanup", err)
	}
}

package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestFixedSearchBreadthFirstCancellationAndOwnedChildren(t *testing.T) {
	m, err := numeral.NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewFixedContext(m, &control.Reslimit{Limit: 1000}, []bool{false}, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	c.maxDepth = 1
	root := c.root
	c.limit.Limit, c.limit.Steps = 1, 1
	if err := c.Search(); !errors.Is(err, ErrFixedContextCanceled) || len(c.leafQueue) != 1 || c.leafQueue[0] != root || c.searchNode != nil {
		t.Fatal("pre-selection checkpoint", err)
	}
	c.limit.Limit, c.limit.Steps = 1, 0
	if err := c.Search(); !errors.Is(err, ErrFixedContextCanceled) || c.numSplits != 1 || len(c.leafQueue) != 2 || root.children != 2 || len(c.searchOwned) != 2 {
		t.Fatal("owned child queue at checkpoint", err)
	}
	if c.leafQueue[0] != c.searchOwned[0] || c.leafQueue[1] != c.searchOwned[1] {
		t.Fatal("oldest child selection order")
	}
	c.limit.Limit, c.limit.Steps = 2, 1
	if err := c.Search(); !errors.Is(err, ErrFixedContextCanceled) || c.searchNode != nil || c.searchPhase != 0 || len(c.queue) != 1 || len(c.leafQueue) != 1 || c.leafQueue[0] != c.searchOwned[1] {
		t.Fatal("selected child cancellation must retain the source queue prefix", err)
	}
	c.limit.Limit, c.limit.Steps = 100, 0
	if err := c.Search(); !errors.Is(err, arena.ErrInvalidSlot) || len(c.queue) != 1 || len(c.leafQueue) != 1 || c.leafQueue[0] != c.searchOwned[1] {
		t.Fatal("later search must encounter the source queue-empty invariant", err, len(c.queue), len(c.leafQueue))
	}
	if err := c.Close(); err != nil || root.children != 0 || len(c.searchOwned) != 0 {
		t.Fatal("owned child cleanup", err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFixedSearchStopsAtSourceNodeLimit(t *testing.T) {
	m, err := numeral.NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewFixedContext(m, &control.Reslimit{Limit: 1000}, []bool{false}, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	c.maxDepth, c.maxNodes = 2, 1
	if err := c.Search(); err != nil || c.numSplits != 1 || len(c.leafQueue) != 2 || c.root.children != 2 {
		t.Fatal("source node-count stop", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFixedSearchRetainsRefusedSplitChildrenForClose(t *testing.T) {
	m, err := numeral.NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewFixedContext(m, &control.Reslimit{Limit: 1000}, []bool{false}, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	v, err := m.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetInt64(v, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateBound(c.root, 0, v, true, false); err != nil {
		t.Fatal(err)
	}
	if err := m.SetRatio(v, 1, 1<<32); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateBound(c.root, 0, v, false, false); err != nil {
		t.Fatal(err)
	}
	if err := c.Search(); !errors.Is(err, ErrFixedSplitNoInterior) || !errors.Is(err, ErrSubpavingFailure) || len(c.searchOwned) != 2 || c.root.children != 2 || len(c.leafQueue) != 2 {
		t.Fatal("source split failure and owned children", err)
	}
	if err := c.Close(); err != nil || c.numNodes != 0 {
		t.Fatal("failed split children retired", err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}

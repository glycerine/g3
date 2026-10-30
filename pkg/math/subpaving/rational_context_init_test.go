package subpaving

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestRationalContextInitializeRootDoesNotResumeCanceledSourceInit(t *testing.T) {
	c, m, value, x, y := rationalClauseFixture(t)
	if err := m.SetInt32(value, 5); err != nil {
		t.Fatal(err)
	}
	xLower, err := c.NewInequality(x, value, true, false)
	if err != nil {
		t.Fatal(err)
	}
	yLower, err := c.NewInequality(y, value, true, false)
	if err != nil {
		t.Fatal(err)
	}
	clause, err := c.AddClause([]*RationalInequality{yLower, xLower})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetInt32(value, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddInequality(x, value, false, false, true); err != nil {
		t.Fatal(err)
	}
	if err := m.SetInt32(value, 8); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddInequality(y, value, false, false, true); err != nil {
		t.Fatal(err)
	}
	c.limit.Limit = 1
	if err := c.InitializeRoot(); !errors.Is(err, ErrRationalContextCanceled) || c.unitHead != 1 || c.initStage != 0 || len(c.queue) != 1 || c.numBounds != 1 {
		t.Fatal("root did not retain first unit on cancellation", err, c.unitHead, c.initStage)
	}
	c.limit.Limit = 3
	if err := c.InitializeRoot(); err != nil || c.unitHead != 1 || c.initStage != 0 || len(c.queue) != 1 || c.queueHead != 0 || c.numBounds != 1 {
		t.Fatal("later call resumed the canceled unit prefix", err, c.unitHead, c.initStage)
	}
	c.limit.Limit = 0
	if err := c.InitializeRoot(); err != nil {
		t.Fatal(err)
	}
	n := c.root
	if n == nil || n.conflictVar != math.MaxUint32 || n.lower[y] != nil || c.initStage != 0 || c.numBounds != 1 || c.numVisited != 0 || len(c.queue) != 1 || c.queueHead != 0 {
		t.Fatal("later call changed the partial source root", n, c.initStage, c.numBounds, c.numVisited, clause)
	}
	if _, err := c.NewVariable(false); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("variable growth after root creation", err)
	}
}

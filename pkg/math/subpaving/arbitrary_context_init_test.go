package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestArbitraryContextRootInitializationStagesAndConflict(t *testing.T) {
	setup := func(t *testing.T, conflicting bool) (*arbitraryClauseFixture, uint32) {
		t.Helper()
		f := newArbitraryClauseFixture(t)
		c := f.context
		out, err := c.DefineMonomial([]ArbitraryPower{{f.x, 2}})
		if err != nil {
			t.Fatal(err)
		}
		lowerValue, upperValue := &f.four, &f.five
		if conflicting {
			lowerValue, upperValue = &f.five, &f.four
		}
		lower, err := c.NewInequality(f.x, lowerValue, true, false)
		if err != nil {
			t.Fatal(err)
		}
		upper, err := c.NewInequality(f.x, upperValue, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddUnitClause(lower, true); err != nil {
			t.Fatal(err)
		}
		if err := c.AddUnitClause(upper, true); err != nil {
			t.Fatal(err)
		}
		return f, out
	}
	t.Run("canceled unit leaves source root initialization incomplete", func(t *testing.T) {
		f, out := setup(t, false)
		c := f.context
		c.limit.Limit, c.limit.Steps = 1, 0
		if err := c.InitializeRoot(); !errors.Is(err, ErrArbitraryContextCanceled) || c.unitHead != 1 || c.initStage != 0 {
			t.Fatalf("first unit checkpoint: %v, head=%d, stage=%d", err, c.unitHead, c.initStage)
		}
		if c.root.lower[f.x] == nil || c.root.upper[f.x] != nil || c.root.lower[out] != nil {
			t.Fatal("unit suffix or definition ran before resumed checkpoint")
		}
		c.limit.Limit, c.limit.Steps = 1000, 0
		if err := c.InitializeRoot(); err != nil {
			t.Fatal(err)
		}
		if c.initStage != 0 || c.initDefHead != 0 || c.unitHead != 1 || c.root.upper[f.x] != nil || c.root.lower[out] != nil || c.root.upper[out] != nil {
			t.Fatal("later initialization resumed the canceled source prefix", c.initStage, c.initDefHead, c.unitHead)
		}
		if _, err := c.NewVariable(false); !errors.Is(err, arena.ErrInvalidSlot) {
			t.Fatal("variable growth after root creation", err)
		}
	})
	t.Run("unit conflict skips definitions", func(t *testing.T) {
		f, out := setup(t, true)
		c := f.context
		if err := c.InitializeRoot(); err != nil {
			t.Fatal(err)
		}
		if c.root.conflictVar != f.x || c.root.lower[out] != nil || c.numVisited != 0 || c.initStage != 3 || len(c.queue) != 0 {
			t.Fatal("root conflict must stop definition sweep and clear queue")
		}
	})
	t.Run("canceled queue remains pending after root initialization", func(t *testing.T) {
		f, out := setup(t, false)
		c := f.context
		c.limit.Limit, c.limit.Steps = 2, 0
		if err := c.InitializeRoot(); !errors.Is(err, ErrArbitraryContextCanceled) || c.initStage != 2 || c.queueHead != 0 || c.root.lower[out] == nil {
			t.Fatal("queue checkpoint must follow completed definition sweep", err, c.initStage, c.queueHead)
		}
		visits, trail := c.numVisited, c.root.trail
		c.limit.Limit, c.limit.Steps = 0, 0
		if err := c.InitializeRoot(); err != nil || c.initStage != 2 || c.root.trail != trail || c.numVisited != visits || c.queueHead != 0 || len(c.queue) == 0 {
			t.Fatal("later initialization drained the canceled source queue", err, c.initStage, c.numVisited, visits)
		}
	})
}

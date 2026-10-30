package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestSubpavingFastContextRootInitializationOrderAndNoResume(t *testing.T) {
	m, c, v, root := fastMonomialFixture(t, []bool{false, false})
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	x, err := c.DefineMonomial([]FastPower{{Variable: 0, Degree: 2}})
	must(err)
	makeAtom := func(variable uint32, value int32, lower bool) *FastInequality {
		t.Helper()
		must(m.SetInt32(v, value))
		a, e := c.NewInequality(variable, v, lower, false)
		must(e)
		return a
	}
	must(c.AddUnitClause(makeAtom(0, 2, true), true))
	must(c.AddUnitClause(makeAtom(0, 3, false), true))
	clause, err := c.AddClause([]*FastInequality{makeAtom(1, 5, true), makeAtom(0, 0, false)}, false, true)
	must(err)
	if clause.atoms[0].variable != 0 {
		t.Fatal("source stable clause order")
	}
	c.limit.Limit, c.limit.Steps = 1, 0
	if err := c.InitializeRoot(); !errors.Is(err, ErrFastContextCanceled) || c.unitHead != 1 || c.initStage != 0 {
		t.Fatal("first root checkpoint", c.unitHead, c.initStage, err)
	}
	fastMonomialCheckBound(t, m, root, v, 0, 2, true)
	if root.upper[0] != nil || root.lower[1] != nil {
		t.Fatal("unasserted suffix already published")
	}
	c.limit.Limit, c.limit.Steps = 1000, 0
	must(c.InitializeRoot())
	if c.unitHead != 1 || c.initStage != 0 || c.initDefHead != 0 || len(c.queue) != 1 || c.queueHead != 0 {
		t.Fatal("source root must retain the interrupted prefix without resuming", c.unitHead, c.initStage, c.initDefHead, len(c.queue), c.queueHead)
	}
	fastMonomialCheckBound(t, m, root, v, 0, 2, true)
	if root.upper[0] != nil || root.lower[1] != nil || root.lower[x] != nil {
		t.Fatal("interrupted root initialization ran a later stage")
	}
	if _, err := c.DefineMonomial([]FastPower{{Variable: 1, Degree: 2}}); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("source root allocation must freeze variable definitions", err)
	}
	if err := c.Search(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("search after interrupted source init must expose the nonempty queue invariant", err)
	}
}

func TestSubpavingFastContextRootInitializationUnitConflict(t *testing.T) {
	m, c, v, root := fastMonomialFixture(t, []bool{false})
	for _, tc := range []struct {
		value int32
		lower bool
	}{{5, true}, {4, false}} {
		if err := m.SetInt32(v, tc.value); err != nil {
			t.Fatal(err)
		}
		a, err := c.NewInequality(0, v, tc.lower, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddUnitClause(a, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.InitializeRoot(); err != nil {
		t.Fatal(err)
	}
	if !root.inconsistent || root.conflictVar != 0 || c.initStage != 3 || len(c.queue) != 0 {
		t.Fatal("unit conflict did not end initialization", root.inconsistent, root.conflictVar, c.initStage)
	}
}

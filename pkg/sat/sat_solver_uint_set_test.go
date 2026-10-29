package sat

// Source: src/sat/sat_types.h:153-204; src/sat/sat_model_converter.cpp.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"reflect"
	"testing"
)

func satSolverSetOwner(t *testing.T) (*satSolverUintSet, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(^uint64(0))
	s, e := newSATSolverUintSet(b)
	paramsMust(t, e)
	t.Cleanup(func() {
		paramsMust(t, s.Close())
		if b.Used() != 0 {
			t.Fatal("set retained", b.Used())
		}
	})
	return s, b
}

func TestSATSolverUintSetSourceRemovePopCopyAndSetOperations(t *testing.T) {
	left, _ := satSolverSetOwner(t)
	right, _ := satSolverSetOwner(t)
	copySet, _ := satSolverSetOwner(t)
	for _, v := range []uint32{7, 2, 9, 4} {
		paramsMust(t, left.Insert(v))
	}
	for _, v := range []uint32{9, 7, 12} {
		paramsMust(t, right.Insert(v))
	}
	paramsMust(t, left.Remove(2))
	if !reflect.DeepEqual(left.values, []uint32{7, 4, 9}) || left.Contains(2) {
		t.Fatal("source swap remove", left.values)
	}
	paramsMust(t, left.Remove(2))
	paramsMust(t, copySet.Assign(left))
	paramsMust(t, left.Intersect(right))
	if !reflect.DeepEqual(left.values, []uint32{7, 9}) || left.Contains(4) {
		t.Fatal("source stable intersection", left.values)
	}
	paramsMust(t, left.Union(right))
	if !reflect.DeepEqual(left.values, []uint32{7, 9, 12}) {
		t.Fatal("source ordered union", left.values)
	}
	if !reflect.DeepEqual(copySet.values, []uint32{7, 4, 9}) {
		t.Fatal("assignment aliases source", copySet.values)
	}
	paramsMust(t, right.Reset())
	if !reflect.DeepEqual(copySet.values, []uint32{7, 4, 9}) {
		t.Fatal("assignment aliases other storage", copySet.values)
	}
	for _, want := range []uint32{9, 4, 7} {
		got, err := copySet.Pop()
		if err != nil || got != want || copySet.Contains(want) {
			t.Fatal("source LIFO erase", got, want, err, copySet.values)
		}
	}
	if _, err := copySet.Pop(); !errors.Is(err, arena.ErrBounds) && !errors.Is(err, ErrSATAssertion) {
		t.Fatal("empty erase", err)
	}
}

func TestSATSolverUintSetIndependentMutationModel(t *testing.T) {
	set, _ := satSolverSetOwner(t)
	model := make(map[uint32]bool)
	var order []uint32
	state := uint32(29)
	for step := 0; step < 3000; step++ {
		state = state*1664525 + 1013904223
		value := (state >> 8) % 97
		if state&3 != 0 {
			paramsMust(t, set.Insert(value))
			if !model[value] {
				model[value] = true
				order = append(order, value)
			}
		} else {
			paramsMust(t, set.Remove(value))
			if model[value] {
				delete(model, value)
				for i, candidate := range order {
					if candidate == value {
						order[i] = order[len(order)-1]
						order = order[:len(order)-1]
						break
					}
				}
			}
		}
		if !reflect.DeepEqual(set.values, order) {
			t.Fatal("independent source order", step, set.values, order)
		}
		for candidate := uint32(0); candidate < 97; candidate++ {
			if set.Contains(candidate) != model[candidate] {
				t.Fatal("independent membership", step, candidate)
			}
		}
	}
}

func TestSATSolverUintSetNativeGrowthAndReset(t *testing.T) {
	s, b := satSolverSetOwner(t)
	used := b.Used()
	b.FailAfter(0)
	defer b.FailAfter(-1)
	for _, v := range []uint32{400, 1, 7, 400, 0} {
		paramsMust(t, s.Insert(v))
	}
	if !reflect.DeepEqual(s.values, []uint32{400, 1, 7, 0}) || b.Used() != used {
		t.Fatal("unique insertion order/native growth", s.values)
	}
	membershipCap, valuesCap := cap(s.inSet), cap(s.values)
	paramsMust(t, s.Reset())
	if s.Size() != 0 || s.Contains(400) || cap(s.inSet) != membershipCap || cap(s.values) != valuesCap {
		t.Fatal("reset lost capacity or membership")
	}
	paramsMust(t, s.Insert(7))
	paramsMust(t, s.Insert(400))
	if s.Contains(1) || !reflect.DeepEqual(s.values, []uint32{7, 400}) {
		t.Fatal("reset resurrected old membership", s.values)
	}
	paramsMust(t, s.Finalize())
	if s.values != nil || s.inSet != nil {
		t.Fatal("finalize retained backing")
	}
	paramsMust(t, s.Insert(1))
	if !s.Contains(1) || s.Size() != 1 {
		t.Fatal("set not reusable")
	}
	if e := s.Insert(^uint32(0)); !errors.Is(e, arena.ErrBounds) || s.Size() != 1 {
		t.Fatal("wrapped index mutated set", e)
	}
}

func TestSATSolverUintSetIndependentNativeModel(t *testing.T) {
	s, _ := satSolverSetOwner(t)
	seen := make(map[uint32]bool)
	var order []uint32
	state := uint32(17)
	for i := 0; i < 2000; i++ {
		state = state*1664525 + 1013904223
		v := state % 256
		if i%47 == 0 {
			paramsMust(t, s.Reset())
			clear(seen)
			order = order[:0]
		}
		paramsMust(t, s.Insert(v))
		if !seen[v] {
			order = append(order, v)
			seen[v] = true
		}
		if !reflect.DeepEqual(s.values, order) {
			t.Fatal("independent order", i, s.values, order)
		}
		for candidate := uint32(0); candidate < 256; candidate++ {
			if s.Contains(candidate) != seen[candidate] {
				t.Fatal("independent membership", i, candidate)
			}
		}
	}
}

func TestSATSolverModelConverterActualCollectVars(t *testing.T) {
	c, _ := satConverterTestOwner(t)
	s, b := satSolverSetOwner(t)
	paramsMust(t, s.Insert(2))
	for _, v := range []BoolVar{2, 8, 1} {
		_, e := c.Mk(satModelBlockLit, v)
		paramsMust(t, e)
	}
	used := b.Used()
	b.FailAfter(0)
	defer b.FailAfter(-1)
	paramsMust(t, c.CollectVars(s))
	if !reflect.DeepEqual(s.values, []uint32{2, 8, 1}) || b.Used() != used {
		t.Fatal("source traversal/prepopulated dedup", s.values)
	}
	paramsMust(t, s.Reset())
	paramsMust(t, c.CollectVars(s))
	if !reflect.DeepEqual(s.values, []uint32{2, 8, 1}) {
		t.Fatal("repeated collection", s.values)
	}
}

package sat

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestSATCutSetAntichainAndUpdateOrder(t *testing.T) {
	b := arena.NewBudget(math.MaxUint64)
	s, err := newSATCutSet(b)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Init(99, 17); err != nil {
		t.Fatal(err)
	}
	var events []string
	add := func(_ uint32, c satCut) { events = append(events, "add:"+c.String()) }
	del := func(_ uint32, c satCut) { events = append(events, "del:"+c.String()) }
	for _, c := range []satCut{testSATCut([]uint32{1, 2}, 6), testSATCut([]uint32{1, 3}, 6)} {
		if ok, e := s.Insert(add, del, c); e != nil || !ok {
			t.Fatal(ok, e)
		}
	}
	events = nil
	narrow := testSATCut([]uint32{1}, 2)
	if ok, e := s.Insert(add, del, narrow); e != nil || !ok {
		t.Fatal(ok, e)
	}
	if len(events) != 3 || events[0][:4] != "add:" || events[1][:4] != "del:" || events[2][:4] != "del:" {
		t.Fatalf("source add-before-delete order: %v", events)
	}
	if s.Size() != 1 || !s.cuts[0].Equal(narrow) || !s.NoDuplicates() {
		t.Fatal("antichain replacement", s.cuts)
	}
	if ok, e := s.Insert(add, del, testSATCut([]uint32{1, 9}, 6)); e != nil || ok || s.Size() != 1 {
		t.Fatal("existing subset must reject candidate", ok, e)
	}
	if err = s.Close(); err != nil || b.Used() != 0 {
		t.Fatal("close accounting", err, b.Used())
	}
}

func TestSATCutSetGrowthFailureAndLifecycle(t *testing.T) {
	b := arena.NewBudget(math.MaxUint64)
	s, err := newSATCutSet(b)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Init(1, 5); err != nil {
		t.Fatal(err)
	}
	adds := 0
	add := func(uint32, satCut) { adds++ }
	for _, id := range []uint32{1, 2} {
		if err = s.PushBack(add, newSATCut(id)); err != nil {
			t.Fatal(err)
		}
	}
	used := b.Used()
	b.FailAfter(0)
	inserted, err := s.Insert(add, nil, newSATCut(3))
	if !errors.Is(err, arena.ErrInjectedFailure) || inserted || s.Size() != 2 || s.cuts[0].Elem(0) != 1 || s.cuts[1].Elem(0) != 2 || adds != 2 || b.Used() != used {
		t.Fatal("failed growth published state", inserted, err, s.cuts, adds, b.Used(), used)
	}
	b.FailAfter(-1)
	var deleted []uint32
	del := func(_ uint32, c satCut) { deleted = append(deleted, c.Elem(0)) }
	if err = s.EvictIndex(del, 0); err != nil || len(deleted) != 1 || deleted[0] != 1 || s.cuts[0].Elem(0) != 2 {
		t.Fatal("swap eviction", err, deleted, s.cuts)
	}
	if err = s.Reset(del); err != nil || len(deleted) != 2 || deleted[1] != 2 || s.Size() != 0 {
		t.Fatal("reset callbacks", err, deleted)
	}
	if err = s.Close(); err != nil || b.Used() != 0 {
		t.Fatal(err, b.Used())
	}
}

func TestSATCutSetSwapRequiresSharedOwner(t *testing.T) {
	b := arena.NewBudget(math.MaxUint64)
	a, err := newSATCutSet(b)
	if err != nil {
		t.Fatal(err)
	}
	c, err := newSATCutSet(b)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Init(2, 10); err != nil {
		t.Fatal(err)
	}
	if err = c.Init(2, 20); err != nil {
		t.Fatal(err)
	}
	if err = a.PushBack(nil, newSATCut(1)); err != nil {
		t.Fatal(err)
	}
	if err = c.PushBack(nil, newSATCut(2)); err != nil {
		t.Fatal(err)
	}
	if err = a.Swap(c); err != nil || a.Var() != 20 || c.Var() != 10 || a.cuts[0].Elem(0) != 2 || c.cuts[0].Elem(0) != 1 {
		t.Fatal("source swap", err, a.variable, c.variable, a.cuts, c.cuts)
	}
	foreign, err := newSATCutSet(arena.NewBudget(math.MaxUint64))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Swap(foreign); !errors.Is(err, ErrSATOwner) {
		t.Fatal("foreign swap", err)
	}
	foreignBudget := foreign.budget
	if err = foreign.Close(); err != nil || foreignBudget.Used() != 0 {
		t.Fatal(err, foreignBudget.Used())
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil || b.Used() != 0 {
		t.Fatal(err, b.Used())
	}
}

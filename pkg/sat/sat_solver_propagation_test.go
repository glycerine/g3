package sat

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func satPropagationPush(t *testing.T, s *satSolverPropagationState, l Literal, w satWatched) {
	t.Helper()
	v, e := s.variables.watches.Get(l.index())
	satOK(t, e)
	satOK(t, v.Push(w))
}
func TestSATSolverPropagateBinaryTernaryAndConflictCleanup(t *testing.T) {
	s, v, _ := satPropagationFixture(t, 4)
	v.scopeLevel = 2
	w, e := newSATBinaryWatch(MakeLiteral(1, false), false)
	satOK(t, e)
	satPropagationPush(t, s, 0, w)
	w, e = newSATTernaryWatch(MakeLiteral(1, true), MakeLiteral(2, false))
	satOK(t, e)
	satPropagationPush(t, s, 0, w)
	w, e = newSATBinaryWatch(MakeLiteral(3, false), false)
	satOK(t, e)
	satPropagationPush(t, s, 0, w)
	satOK(t, s.assign(MakeLiteral(3, true), satJustification{}))
	satOK(t, s.assign(0, satJustification{}))
	ok, e := s.propagateCore(false)
	satOK(t, e)
	if ok || !s.inconsistent || s.binaryPropagations != 1 || s.ternaryPropagations != 1 {
		t.Fatal(ok, s.binaryPropagations, s.ternaryPropagations)
	}
	if satJustificationTestValue(t, &v.justification, 1) != newSATBinaryJustification(1) || satJustificationTestValue(t, &v.justification, 2) != newSATTernaryJustification(MakeLiteral(1, true), 1) {
		t.Fatal("source justifications")
	}
	a, e := v.watches.Get(0)
	satOK(t, e)
	n, e := a.Size()
	satOK(t, e)
	if n != 3 {
		t.Fatal("cleanup suffix", n)
	}
}
func TestSATSolverPropagateClauseMoveUnitGlueAndConflict(t *testing.T) {
	s, v, _ := satPropagationFixture(t, 4)
	v.scopeLevel = 3
	c, e := s.clauses.MkClause([]Literal{1, 2, 4}, true)
	satOK(t, e)
	satOK(t, s.clauses.SetGlue(c, 7))
	o, e := s.clauses.Offset(c)
	satOK(t, e)
	w, e := newSATClauseWatch(2, o)
	satOK(t, e)
	satPropagationPush(t, s, 0, w)
	satOK(t, s.assign(0, satJustification{}))
	ok, e := s.propagateCore(true)
	satOK(t, e)
	if !ok {
		t.Fatal("move")
	}
	x, e := s.clauses.Literal(c, 1)
	satOK(t, e)
	if x != 4 {
		t.Fatal(x)
	}
	a, e := v.watches.Get(5)
	satOK(t, e)
	n, e := a.Size()
	satOK(t, e)
	if n != 1 {
		t.Fatal("destination", n)
	}
	satOK(t, s.assign(5, satJustification{}))
	ok, e = s.propagateCore(true)
	satOK(t, e)
	if !ok || s.clausePropagations != 1 {
		t.Fatal(ok, s.clausePropagations)
	}
	if satJustificationTestValue(t, &v.justification, 1) != newSATClauseJustification(o) {
		t.Fatal("unit justification")
	}
	glue, e := s.clauses.Glue(c)
	satOK(t, e)
	if glue != 1 {
		t.Fatal("actual diff levels", glue)
	}
	used, e := s.clauses.Used(c)
	satOK(t, e)
	if !used {
		t.Fatal("mark used")
	}
}

func TestSATPropagationMoveToRecycledList(t *testing.T) {
	s, v, _ := satPropagationFixture(t, 4)
	clause, err := s.clauses.MkClause([]Literal{1, 2, 4}, true)
	satOK(t, err)
	offset, err := s.clauses.Offset(clause)
	satOK(t, err)
	watch, err := newSATClauseWatch(2, offset)
	satOK(t, err)
	satPropagationPush(t, s, 0, watch)

	// Propagating literal 0 moves this clause to the list for literal 5.
	// Keep the old directory reference and give its replacement spare capacity.
	lists := v.watches
	old := lists.lists[5]
	satOK(t, lists.store.Free(old))
	current, err := lists.store.NewList()
	satOK(t, err)
	defer func() { lists.lists[5] = current }()
	if old.slot != current.slot || old.generation == current.generation {
		t.Fatal("expected a recycled destination slot")
	}
	satOK(t, lists.store.Push(current, watch))
	satOK(t, lists.store.SetEnd(current, 0))
	satOK(t, s.assign(0, satJustification{}))
	if _, err = s.propagateCore(false); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("move to a stale destination returned %v", err)
	}
	n, err := lists.store.Size(current)
	satOK(t, err)
	if n != 0 {
		t.Fatalf("moved watch into the replacement list, size %d", n)
	}
}

func TestSATSolverPropagateExternalActualDispatchBoundary(t *testing.T) {
	s, v, _ := satPropagationFixture(t, 1)
	s.extensionPresent = true
	w, e := newSATExternalWatch(3)
	satOK(t, e)
	satPropagationPush(t, s, 0, w)
	satOK(t, s.assign(0, satJustification{}))
	_, e = s.propagateCore(false)
	if !errors.Is(e, ErrSATMissingSourceExtBinding) || v.qhead != 1 || s.cleanerCounter != -1 || s.asymmCounter != -1 || s.probingCounter != -1 {
		t.Fatal(e, v.qhead)
	}
}

func TestSATSolverPropagationIndependentTruthTables(t *testing.T) {
	for a := 0; a < 3; a++ {
		for b := 0; b < 3; b++ {
			t.Run(string(rune('0'+a))+string(rune('0'+b)), func(t *testing.T) {
				s, v, _ := satPropagationFixture(t, 3)
				v.scopeLevel = 1
				vals := []lbool.LBool{lbool.False, lbool.Undef, lbool.True}
				for i, x := range []lbool.LBool{vals[a], vals[b]} {
					if x == lbool.Undef {
						continue
					}
					l := MakeLiteral(BoolVar(i+1), x == lbool.False)
					satOK(t, s.assign(l, satJustification{}))
				}
				w, e := newSATTernaryWatch(2, 4)
				satOK(t, e)
				satPropagationPush(t, s, 0, w)
				satOK(t, s.assign(0, satJustification{}))
				ok, e := s.propagateCore(false)
				satOK(t, e)
				conf := a == 0 && b == 0
				if ok == conf || s.inconsistent != conf {
					t.Fatal("independent disjunction", a, b, ok)
				}
				want := uint32(0)
				if (a == 0 && b == 1) || (a == 1 && b == 0) {
					want = 1
				}
				if s.ternaryPropagations != want {
					t.Fatal(s.ternaryPropagations, want)
				}
				satPropagationAssertGraph(t, s)
			})
		}
	}
	for a := 0; a < 3; a++ {
		t.Run("binary"+string(rune('0'+a)), func(t *testing.T) {
			s, _, _ := satPropagationFixture(t, 2)
			if a != 1 {
				satOK(t, s.assign(MakeLiteral(1, a == 0), satJustification{}))
			}
			w, e := newSATBinaryWatch(2, false)
			satOK(t, e)
			satPropagationPush(t, s, 0, w)
			satOK(t, s.assign(0, satJustification{}))
			ok, e := s.propagateCore(false)
			satOK(t, e)
			if ok != (a != 0) || s.binaryPropagations != uint32(map[bool]int{true: 1}[a == 1]) {
				t.Fatal(a, ok)
			}
			satPropagationAssertGraph(t, s)
		})
	}
	// All 27 three-literal status combinations, independent boolean disjunction.
	s, v, _ := satPropagationFixture(t, 3)
	c, e := s.clauses.MkClause([]Literal{0, 2, 4}, false)
	satOK(t, e)
	vals := []lbool.LBool{lbool.False, lbool.Undef, lbool.True}
	for a := 0; a < 3; a++ {
		for b := 0; b < 3; b++ {
			for d := 0; d < 3; d++ {
				want := lbool.False
				for i, x := range []int{a, b, d} {
					satOK(t, v.setAssignment(Literal(uint32(2*i)), lbool.LBool(vals[x])))
					if x == 2 {
						want = lbool.True
					} else if x == 1 && want != lbool.True {
						want = lbool.Undef
					}
				}
				got, e := s.status(c)
				satOK(t, e)
				if got != want {
					t.Fatal(a, b, d, got, want)
				}
			}
		}
	}
}
func TestSATSolverPropagationClauseBranchCompactionAndNativeMove(t *testing.T) {
	t.Run("native-move", func(t *testing.T) {
		s, v, b := satPropagationFixture(t, 3)
		c, e := s.clauses.MkClause([]Literal{1, 2, 4}, false)
		satOK(t, e)
		o, e := s.clauses.Offset(c)
		satOK(t, e)
		w, e := newSATClauseWatch(2, o)
		satOK(t, e)
		satPropagationPush(t, s, 0, w)
		dest, e := v.watches.Get(5)
		satOK(t, e)
		filler, e := newSATBinaryWatch(0, false)
		satOK(t, e)
		for i := 0; i < 40; i++ {
			satOK(t, dest.Push(filler))
		}
		r, e := dest.ref()
		satOK(t, e)
		record, e := v.watches.store.record(r)
		satOK(t, e)
		for len(record.values()) < cap(record.values()) {
			satOK(t, dest.Push(filler))
		}
		satOK(t, s.assign(0, satJustification{}))
		b.FailAfter(0)
		_, e = s.propagateCore(false)
		satOK(t, e)
		x, e := s.clauses.Literal(c, 1)
		satOK(t, e)
		y, e := s.clauses.Literal(c, 2)
		satOK(t, e)
		if x != 4 || y != 1 || v.qhead != 1 {
			t.Fatal("completed clause writes", x, y, v.qhead)
		}
		src, e := v.watches.Get(0)
		satOK(t, e)
		n, e := src.Size()
		satOK(t, e)
		if n != 0 {
			t.Fatal(n)
		}
		satPropagationAssertGraph(t, s)
		b.FailAfter(-1)
	})
	for _, route := range []string{"blocked", "removed", "mismatch", "first-true", "conflict"} {
		t.Run(route, func(t *testing.T) {
			s, v, _ := satPropagationFixture(t, 3)
			v.scopeLevel = 1
			c, e := s.clauses.MkClause([]Literal{2, 1, 4}, false)
			satOK(t, e)
			o, e := s.clauses.Offset(c)
			satOK(t, e)
			blocked := Literal(4)
			switch route {
			case "blocked":
				satOK(t, s.assign(4, satJustification{}))
			case "removed":
				satOK(t, s.clauses.SetRemoved(c, true))
			case "mismatch":
				satOK(t, s.clauses.SetLiteral(c, 1, 5))
			case "first-true":
				satOK(t, s.assign(2, satJustification{}))
			case "conflict":
				satOK(t, s.assign(3, satJustification{}))
				satOK(t, s.assign(5, satJustification{}))
			}
			w, e := newSATClauseWatch(blocked, o)
			satOK(t, e)
			satPropagationPush(t, s, 0, w)
			tail, e := newSATBinaryWatch(4, false)
			satOK(t, e)
			satPropagationPush(t, s, 0, tail)
			satOK(t, s.assign(0, satJustification{}))
			ok, e := s.propagateCore(false)
			satOK(t, e)
			if ok == (route == "conflict") {
				t.Fatal(route, ok)
			}
			src, e := v.watches.Get(0)
			satOK(t, e)
			n, e := src.Size()
			satOK(t, e)
			if n != 2 {
				t.Fatal("retained conflict/skip suffix", n)
			}
			if route == "first-true" {
				got, e := src.Read(0)
				satOK(t, e)
				l, e := got.BlockedLiteral()
				satOK(t, e)
				if l != 2 {
					t.Fatal(l)
				}
			}
			satPropagationAssertGraph(t, s)
		})
	}
}
func TestSATSolverPropagationCapturedBackingAtEnd(t *testing.T) {
	s, v, _ := satPropagationFixture(t, 1)
	list, e := v.watches.Get(0)
	satOK(t, e)
	w, e := newSATBinaryWatch(0, false)
	satOK(t, e)
	satOK(t, list.Push(w))
	satOK(t, list.Push(w))
	r, e := list.ref()
	satOK(t, e)
	it, e := v.watches.store.Iter(r)
	satOK(t, e)
	satOK(t, list.Push(w))
	if e = s.finishPropagationList(&it, list, 0); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal("old end against new backing", e)
	}
	// Actual empty null backing must not force a nonexistent payload dereference.
	empty, e := v.watches.Get(1)
	satOK(t, e)
	r, e = empty.ref()
	satOK(t, e)
	it, e = v.watches.store.Iter(r)
	satOK(t, e)
	satOK(t, s.finishPropagationList(&it, empty, 0))
}

func TestSATSolverPropagationWarmActualBackingPlateau(t *testing.T) {
	s, v, b := satPropagationFixture(t, 3)
	v.scopeLevel = 1
	w, e := newSATBinaryWatch(2, false)
	satOK(t, e)
	satPropagationPush(t, s, 0, w)
	w, e = newSATTernaryWatch(3, 4)
	satOK(t, e)
	satPropagationPush(t, s, 0, w)
	cycle := func() {
		for i := uint32(0); i < uint32(len(v.assignment)); i++ {
			satOK(t, v.setAssignment(Literal(i), lbool.Undef))
		}
		satOK(t, v.trail.Shrink(v.trailRef, 0))
		v.qhead = 0
		satOK(t, s.assign(0, satJustification{}))
		ok, e := s.propagateCore(false)
		satOK(t, e)
		if !ok {
			t.Fatal("finite chain")
		}
	}
	cycle()
	before := b.Used()
	b.FailAfter(0)
	allocs := testing.AllocsPerRun(20, cycle)
	if allocs != 0 || b.Used() != before {
		t.Fatal("warmed actual owners", allocs, b.Used(), before)
	}
	satPropagationAssertGraph(t, s)
	b.FailAfter(-1)
}
func TestSATSolverPropagationLastSelfMoveRawLifetime(t *testing.T) {
	s, v, _ := satPropagationFixture(t, 2)
	// Source release raw-state boundary: make the false watched literal undef,
	// so replacement aliases current list. Actual source final pointer subtraction
	// is invalid after growth even when there is no following iterator read.
	c, e := s.clauses.MkClause([]Literal{2, 1, 1}, false)
	satOK(t, e)
	o, e := s.clauses.Offset(c)
	satOK(t, e)
	filler, e := newSATBinaryWatch(0, false)
	satOK(t, e)
	satPropagationPush(t, s, 0, filler)
	w, e := newSATClauseWatch(2, o)
	satOK(t, e)
	satPropagationPush(t, s, 0, w)
	satOK(t, s.assign(0, satJustification{}))
	satOK(t, v.setAssignment(Literal(1), lbool.Undef))
	_, e = s.propagateCore(false)
	if satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) || v.qhead != 1 || s.cleanerCounter != -1 || s.asymmCounter != 0 || s.probingCounter != 0 || s.numCheckpoints != 1 {
			t.Fatal("source pre-watch assertion prefix", e, v.qhead)
		}
		list, x := v.watches.Get(0)
		satOK(t, x)
		n, x := list.Size()
		satOK(t, x)
		if n != 2 {
			t.Fatal("unchanged watches", n)
		}
		for i, want := range []Literal{2, 1, 1} {
			got, x := s.clauses.Literal(c, uint32(i))
			satOK(t, x)
			if got != want {
				t.Fatal("unchanged clause", i, got)
			}
		}
		satPropagationAssertGraph(t, s)
		return
	}
	if !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal("actual last-item relocation guard", e)
	}
	list, e := v.watches.Get(0)
	satOK(t, e)
	n, e := list.Size()
	satOK(t, e)
	if n != 3 {
		t.Fatal("completed append before invalid subtraction", n)
	}
	satPropagationAssertGraph(t, s)
}

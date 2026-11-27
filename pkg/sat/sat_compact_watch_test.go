package sat

import (
	"bytes"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSATWatchFullWidthSequence(t *testing.T) {
	store, _ := satWatchStoreFixture(t)
	list, err := store.NewList()
	satOK(t, err)
	// Include both sides of every compressed address boundary. Real watches
	// can come from recycled slots and separate clause storage regions.
	offsets := []satClauseOffset{
		{owner: 0, slot: 0, generation: 0, storageKind: 0},
		{owner: 0, slot: 1<<24 - 1, generation: 126, storageKind: 1},
		{owner: 0, slot: 1 << 24, generation: 1, storageKind: 0},
		{owner: 0, slot: 7, generation: 127, storageKind: 1},
		{owner: 0, slot: 7, generation: 128, storageKind: 0},
		{owner: 0, slot: 7, generation: 1, storageKind: 2},
		{owner: math.MaxUint64, slot: math.MaxUint32, generation: math.MaxUint32, storageKind: math.MaxUint32},
	}
	var clauses []satWatched
	for _, offset := range offsets {
		watch, err := newSATClauseWatch(10, offset)
		satOK(t, err)
		clauses = append(clauses, watch)
	}
	binary, err := newSATBinaryWatch(14, true)
	satOK(t, err)
	ternary, err := newSATTernaryWatch(6, 22)
	satOK(t, err)
	for _, watch := range clauses {
		satOK(t, store.Push(list, watch))
		satOK(t, store.Push(list, ternary))
		satOK(t, store.Push(list, binary))
	}
	satOK(t, store.SortByKind(list))
	want := make([]satWatched, 0, 3*len(offsets))
	for range offsets {
		want = append(want, binary)
	}
	for range offsets {
		want = append(want, ternary)
	}
	want = append(want, clauses...)
	check := func() {
		t.Helper()
		it, err := store.Iter(list)
		satOK(t, err)
		for i, expected := range want {
			got, ok, err := it.Next()
			satOK(t, err)
			if !ok || got != expected {
				t.Fatalf("watch %d got %+v, want %+v", i, got, expected)
			}
		}
		_, ok, err := it.Next()
		satOK(t, err)
		if ok {
			t.Fatal("extra watch after list end")
		}
	}
	check()
	found, err := store.EraseClauseWatch(list, offsets[2])
	satOK(t, err)
	if !found {
		t.Fatal("full-width offset missing after sort")
	}
	i := 2*len(offsets) + 2
	want = append(want[:i], want[i+1:]...)
	check()
	satOK(t, store.SetEnd(list, 1))
	want = want[:1]
	for i := len(clauses) - 1; i >= 0; i-- {
		satOK(t, store.Push(list, clauses[i]))
		want = append(want, clauses[i])
	}
	satOK(t, store.Write(list, 1, binary))
	want[1] = binary
	check()
}

type satWatchTraceWriter func([]byte) (int, error)

func (w satWatchTraceWriter) Write(p []byte) (int, error) { return w(p) }

// The trace writer can change a row while the propagation loop owns a copy
// of the watch. The original full address belongs to that retained copy.
func TestSATWatchKeepsOffsetAcrossTraceWrite(t *testing.T) {
	s, v, _ := satPropagationFixture(t, 3)
	list, err := v.watches.Get(0)
	satOK(t, err)
	other, err := newSATClauseWatch(4, satClauseOffset{owner: math.MaxUint64, slot: 7, generation: math.MaxUint32, storageKind: 1})
	satOK(t, err)
	satOK(t, list.Push(other))
	satOK(t, list.SetEnd(0))
	clause, err := s.clauses.MkClause([]Literal{2, 1, 4}, false)
	satOK(t, err)
	offset, err := s.clauses.Offset(clause)
	satOK(t, err)
	watch, err := newSATClauseWatch(4, offset)
	satOK(t, err)
	satOK(t, list.Push(watch))
	called := false
	tags := diagnostic.NewTraceTagsWithWriter(satWatchTraceWriter(func(p []byte) (int, error) {
		if bytes.Contains(p, []byte("blocked literal")) {
			called = true
			if err := list.Write(0, other); err != nil {
				return 0, err
			}
		}
		return len(p), nil
	}))
	tags.Enable("propagate_clause_bug")
	s.traceTags = tags
	satOK(t, s.assign(4, satJustification{}))
	satOK(t, s.assign(0, satJustification{}))
	_, err = s.propagateCore(false)
	satOK(t, err)
	if called != tags.ShouldTrace("propagate_clause_bug") {
		t.Fatal("trace callback coverage", called)
	}
	got, err := list.Read(0)
	satOK(t, err)
	if got != watch {
		t.Fatalf("retained watch %+v, want %+v", got, watch)
	}
}

func TestSATPropagationTraceReleasesWatchedClause(t *testing.T) {
	for _, tc := range []struct {
		name, tag, needle string
		lits              []Literal
	}{
		{"processing", "propagate_clause_bug", "processing...", []Literal{1, 2, 4}},
		{"mismatch", "propagate_bug", "l: ", []Literal{2, 4, 6}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, v, _ := satPropagationFixture(t, 4)
			c, err := s.clauses.MkClause(tc.lits, false)
			satOK(t, err)
			off, err := s.clauses.Offset(c)
			satOK(t, err)
			w, err := newSATClauseWatch(4, off)
			satOK(t, err)
			list, err := v.watches.Get(0)
			satOK(t, err)
			satOK(t, list.Push(w))
			released := false
			tags := diagnostic.NewTraceTagsWithWriter(satWatchTraceWriter(func(b []byte) (int, error) {
				if !released && bytes.Contains(b, []byte(tc.needle)) {
					released = true
					satOK(t, s.clauses.Delete(c))
				}
				return len(b), nil
			}))
			tags.Enable(tc.tag)
			if !tags.ShouldTrace(tc.tag) {
				t.Skip("requires trace build")
			}
			s.traceTags = tags
			satOK(t, s.assign(0, satJustification{}))
			_, err = s.propagateCore(false)
			if !released || err != arena.ErrInvalidSlot {
				t.Fatalf("released = %t, propagation returned %v", released, err)
			}
		})
	}
}

func TestSATBlockedWideWatchAfterMove(t *testing.T) {
	s, v, _ := satPropagationFixture(t, 4)
	c, err := s.clauses.MkClause([]Literal{1, 2, 4}, false)
	satOK(t, err)
	off, err := s.clauses.Offset(c)
	satOK(t, err)
	moving, err := newSATClauseWatch(2, off)
	satOK(t, err)
	satPropagationPush(t, s, 0, moving)

	// The true blocking literal allows propagation to retain this watch
	// without resolving its exceptional offset.
	off.generation = math.MaxUint32
	blocked, err := newSATClauseWatch(6, off)
	satOK(t, err)
	satPropagationPush(t, s, 0, blocked)
	satOK(t, s.assign(6, satJustification{}))
	satOK(t, s.assign(0, satJustification{}))
	ok, err := s.propagateCore(false)
	satOK(t, err)
	if !ok {
		t.Fatal("unexpected conflict")
	}
	list, err := v.watches.Get(0)
	satOK(t, err)
	n, err := list.Size()
	satOK(t, err)
	if n != 1 {
		t.Fatalf("retained %d watches, want 1", n)
	}
	got, err := list.Read(0)
	satOK(t, err)
	if got != blocked {
		t.Fatalf("retained %+v, want %+v", got, blocked)
	}
}

func TestSATPropagationValidatesUnblockedWatchOffset(t *testing.T) {
	for _, tc := range []struct {
		name string
		want error
	}{
		{"stale", arena.ErrInvalidSlot},
		{"foreign", ErrSATOwner},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := satPropagationFixture(t, 4)
			c, err := s.clauses.MkClause([]Literal{1, 2, 4}, false)
			satOK(t, err)
			off, err := s.clauses.Offset(c)
			satOK(t, err)
			// Give the list a local owner before inserting the invalid watch.
			keep, err := newSATClauseWatch(6, off)
			satOK(t, err)
			satPropagationPush(t, s, 0, keep)
			if tc.name == "foreign" {
				off.owner++
			} else {
				off.generation++
			}
			bad, err := newSATClauseWatch(2, off)
			satOK(t, err)
			satPropagationPush(t, s, 0, bad)
			satOK(t, s.assign(6, satJustification{}))
			satOK(t, s.assign(0, satJustification{}))
			_, err = s.propagateCore(false)
			if err != tc.want {
				t.Fatalf("propagation returned %v, want %v", err, tc.want)
			}
		})
	}
}

package sat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func satVariableBinaryWatch(t *testing.T, l Literal, learned bool) satWatched {
	t.Helper()
	w, e := newSATBinaryWatch(l, learned)
	satOK(t, e)
	return w
}

func satVariableWatchFixture(t *testing.T) (*satSolverWatchLists, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(^uint64(0))
	w, e := newSATSolverWatchLists(b)
	satOK(t, e)
	t.Cleanup(func() {
		satOK(t, w.Close())
		if b.Used() != 0 {
			t.Fatal(b.Used())
		}
	})
	return w, b
}

func TestSATMovedClauseWatchChecksDestination(t *testing.T) {
	lists, _ := satVariableWatchFixture(t)
	offset := satClauseOffset{owner: 5, slot: 11, generation: 3}
	_, wantBounds := lists.Get(0)
	if got := lists.pushClause(0, 8, offset); wantBounds == nil || got != wantBounds {
		t.Fatalf("empty destination returned %v, want %v", got, wantBounds)
	}
	satOK(t, lists.PushEmpty())
	old := lists.lists[0]
	satOK(t, lists.Finalize())
	satOK(t, lists.PushEmpty())
	current := lists.lists[0]
	if old.slot != current.slot || old.generation == current.generation {
		t.Fatal("expected the destination slot to be reused")
	}
	lists.lists[0] = old
	err := lists.pushClause(0, 8, offset)
	lists.lists[0] = current
	if !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("stale destination returned %v", err)
	}
	satOK(t, lists.pushClause(0, 8, offset))
	view, err := lists.Get(0)
	satOK(t, err)
	watch, err := view.Read(0)
	satOK(t, err)
	want, err := newSATClauseWatch(8, offset)
	satOK(t, err)
	if watch != want {
		t.Fatalf("moved watch %v, want %v", watch, want)
	}
}

func TestSATSolverWatchListsRealRelocationAndPayloadLifetime(t *testing.T) {
	w, b := satVariableWatchFixture(t)
	satOK(t, w.PushEmpty())
	satOK(t, w.PushEmpty())
	v, e := w.Get(0)
	satOK(t, e)
	satOK(t, v.Push(satVariableBinaryWatch(t, 2, false)))
	ref, e := v.ref()
	satOK(t, e)
	epoch := w.backingID
	before := w.store.Stats()
	w.lists = w.lists[:len(w.lists):len(w.lists)]
	satOK(t, w.PushEmpty())
	if w.backingID == epoch {
		t.Fatal("expected outer source relocation")
	}
	if _, e = v.Size(); e != ErrSATOwner {
		t.Fatal("old inline object address", e)
	}
	if !w.store.IsLive(ref) || w.store.Stats().Lists != before.Lists+1 {
		t.Fatal("move must not clone/free payload", w.store.Stats(), before)
	}
	moved, e := w.Get(0)
	satOK(t, e)
	got, e := moved.Read(0)
	satOK(t, e)
	if got != satVariableBinaryWatch(t, 2, false) {
		t.Fatal(got)
	}
	for i := 0; i < 8; i++ {
		satOK(t, moved.Push(satVariableBinaryWatch(t, Literal(4+i*2), false)))
	}
	n, e := moved.Size()
	satOK(t, e)
	if n != 9 {
		t.Fatal(n)
	}
	used := b.Used()
	if w.Stats().TotalBytes != used {
		t.Fatal(w.Stats(), used)
	}
	satOK(t, w.Finalize())
	if w.store.IsLive(ref) {
		t.Fatal("source ascending member destruction")
	}
	if _, e = moved.Size(); e == nil {
		t.Fatal(e)
	}
	// The real descriptor free list receives ascending destruction; the last
	// source child is reused first. This is Go physical metadata, not C++ malloc.
	satOK(t, w.PushEmpty())
	last, e := w.Get(0)
	satOK(t, e)
	lastRef, e := last.ref()
	satOK(t, e)
	if lastRef.slot != 2 {
		t.Fatal("ascending child destruction", lastRef)
	}
}
func TestSATSolverWatchListsNativePlacementAndColdUnwind(t *testing.T) {
	w, b := satVariableWatchFixture(t)
	base := b.Used()
	b.FailAfter(0)
	e := w.PushEmpty()
	b.FailAfter(-1)
	satOK(t, e)
	if len(w.lists) != 1 || b.Used() != base {
		t.Fatal("native placement reserved backing", w.Stats(), b.Used(), base)
	}
	for fail := int64(0); fail < 20; fail++ {
		bb := arena.NewBudget(^uint64(0))
		bb.FailAfter(fail)
		x, e := newSATSolverWatchLists(bb)
		bb.FailAfter(-1)
		if e == nil {
			satOK(t, x.Close())
		} else if !errors.Is(e, arena.ErrInjectedFailure) {
			t.Fatal(fail, e)
		}
		if bb.Used() != 0 {
			t.Fatal(fail, bb.Used())
		}
		if e == nil {
			break
		}
	}
}

func TestSATSolverNativeOuterWatchGrowthAndShrink(t *testing.T) {
	w, b := satVariableWatchFixture(t)
	before := b.Used()
	b.FailAfter(0)
	refs := make([]satSolverWatchListRef, 257)
	for i := uint32(0); i < 257; i++ {
		satOK(t, w.PushEmpty())
		view, e := w.Get(i)
		satOK(t, e)
		refs[i], e = view.ref()
		satOK(t, e)
		satOK(t, view.Push(satVariableBinaryWatch(t, Literal(2*i), false)))
	}
	if b.Used() != before || w.Stats().TotalBytes != before {
		t.Fatal("native outer/inner watch backing reserved", w.Stats(), before)
	}
	for i := uint32(0); i < 257; i++ {
		view, e := w.Get(i)
		satOK(t, e)
		value, e := view.Read(0)
		satOK(t, e)
		if value != satVariableBinaryWatch(t, Literal(2*i), false) {
			t.Fatal(i, value)
		}
	}
	satOK(t, w.Shrink(128))
	for i, r := range refs {
		if w.store.IsLive(r) != (i < 128) {
			t.Fatal("child lifetime after shrink", i)
		}
	}
	satOK(t, w.Finalize())
	for i, r := range refs {
		if w.store.IsLive(r) {
			t.Fatal("child survived finalize", i)
		}
	}
	if w.Stats().TotalBytes != before || b.Used() != before {
		t.Fatal("native cleanup changed cold header ownership")
	}
	b.FailAfter(-1)
}

package sat

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"

	utext "github.com/glycerine/g3/pkg/util/text"
	"reflect"
	"testing"
)

func satWatchStoreFixture(t *testing.T) (*satSolverWatchStore, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(^uint64(0))
	s, e := newSATSolverWatchStore(b)
	paramsMust(t, e)
	t.Cleanup(func() {
		paramsMust(t, s.Close())
		if b.Used() != 0 {
			t.Fatalf("retained watch graph %d", b.Used())
		}
	})
	return s, b
}
func satWatchStoreValue(i uint32) satWatched {
	return satWatched{val1: i, val2: 2, locator: satClauseOffset{owner: 0xfedcba9876543210, slot: i, generation: 123, storageKind: 2}}
}
func TestSATSolverWatchStoreOwnerLedgerRetirement(t *testing.T) {
	s, b := satWatchStoreFixture(t)
	other, _ := satWatchStoreFixture(t)
	r, e := s.NewList()
	paramsMust(t, e)
	if _, e = other.Size(r); !errors.Is(e, ErrSATOwner) {
		t.Fatal(e)
	}
	paramsMust(t, s.Free(r))
	if s.IsLive(r) {
		t.Fatal("reset reused generation")
	}
	q, e := s.NewList()
	paramsMust(t, e)
	x, e := s.record(q)
	paramsMust(t, e)
	x.generation = ^uint32(0)
	q.generation = x.generation
	paramsMust(t, s.Free(q))
	n, e := s.NewList()
	paramsMust(t, e)
	if n.slot == q.slot {
		t.Fatal("exhausted slot recycled")
	}
	stats := s.Stats()
	if stats.Retired != 1 || stats.ChargedBytes != b.Used() {
		t.Fatal(stats)
	}
	copied := *s
	if _, e = copied.NewList(); !errors.Is(e, ErrSATOwner) {
		t.Fatal("cold owner copy accepted")
	}
}

func TestSATSolverWatchStoreActualWatchDisplayAndStableSort(t *testing.T) {
	s, _ := satWatchStoreFixture(t)
	r, e := s.NewList()
	paramsMust(t, e)
	a, e := newSATBinaryWatch(Literal(2), true)
	paramsMust(t, e)
	b, e := newSATTernaryWatch(4, 7)
	paramsMust(t, e)
	c, e := newSATExternalWatch(3)
	paramsMust(t, e)
	for _, w := range []satWatched{c, b, a, c, a} {
		paramsMust(t, s.Push(r, w))
	}
	paramsMust(t, s.SortByKind(r))
	want := []satWatched{a, a, b, c, c}
	for i, w := range want {
		v, e := s.Read(r, uint32(i))
		paramsMust(t, e)
		if v != w {
			t.Fatal("sort kind/stability")
		}
	}
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	paramsMust(t, e)
	defer out.Close()
	paramsMust(t, s.DisplayInto(r, nil, out))
	text, e := out.String()
	paramsMust(t, e)
	if text != "1* 1* (2 -3) 3 3" {
		t.Fatal(text)
	}
	ca := satStore(t, nil)
	cl := satMk(t, ca, satLs(0, 2), false)
	cw, e := newSATClauseWatch(4, satClauseOffset(cl))
	paramsMust(t, e)
	paramsMust(t, s.SetEnd(r, 0))
	paramsMust(t, s.Push(r, cw))
	out2, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	paramsMust(t, e)
	defer out2.Close()
	paramsMust(t, s.DisplayInto(r, ca, out2))
	text, e = out2.String()
	paramsMust(t, e)
	if text != "(2 (0 1)+)" {
		t.Fatal(text)
	}
}

func satWatchGraphBytes(v arena.VectorStats) uint64 {
	d := v.Descriptors
	return v.OwnerBytes + v.PayloadBytes + v.BlockMetadataBytes + v.DirectoryBytes + d.OwnerBytes + d.RecordBytes + d.MetadataBytes + d.DirectoryBytes
}

func TestSATSolverNativeWatchGrowthAndSequenceOracle(t *testing.T) {
	s, b := satWatchStoreFixture(t)
	r, e := s.NewList()
	paramsMust(t, e)
	before := b.Used()
	b.FailAfter(0)
	var want []satWatched
	for i := uint32(0); i < 4097; i++ {
		w := satWatchStoreValue(i)
		paramsMust(t, s.Push(r, w))
		want = append(want, w)
	}
	seed := uint32(7)
	for step := uint32(0); step < 4096; step++ {
		seed = 1664525*seed + 1013904223
		switch seed >> 29 {
		case 0, 1, 2:
			w := satWatchStoreValue(step + 5000)
			paramsMust(t, s.Push(r, w))
			want = append(want, w)
		case 3:
			if len(want) > 0 {
				i := int(seed>>8) % len(want)
				w := want[i]
				found, e := s.Erase(r, w)
				paramsMust(t, e)
				if !found {
					t.Fatal("erase oracle", step)
				}
				// These clause watches have full scalar identity; remove the first equal value.
				for j, v := range want {
					if v == w {
						copy(want[j:], want[j+1:])
						want = want[:len(want)-1]
						break
					}
				}
			}
		case 4:
			if len(want) > 0 {
				i := int(seed>>8) % len(want)
				w := satWatchStoreValue(step + 10000)
				paramsMust(t, s.Write(r, uint32(i), w))
				want[i] = w
			}
		case 5:
			n := 0
			if len(want) > 0 {
				n = int(seed>>8) % (len(want) + 1)
			}
			paramsMust(t, s.SetEnd(r, uint32(n)))
			want = want[:n]
		case 6:
			paramsMust(t, s.Finalize(r))
			want = nil
		case 7:
			found, e := s.Contains(r, satWatchStoreValue(step+5000))
			paramsMust(t, e)
			expected := false
			for _, w := range want {
				if w == satWatchStoreValue(step+5000) {
					expected = true
					break
				}
			}
			if found != expected {
				t.Fatal("contains oracle", step)
			}
		}
		n, e := s.Size(r)
		paramsMust(t, e)
		if int(n) != len(want) {
			t.Fatal(step, n, len(want))
		}
		for i, w := range want {
			v, e := s.Read(r, uint32(i))
			paramsMust(t, e)
			if v != w {
				t.Fatal("native sequence", step, i, v, w)
			}
		}
	}
	// The native directory must also grow without reservations.
	for i := 0; i < 257; i++ {
		q, e := s.NewList()
		paramsMust(t, e)
		paramsMust(t, s.Push(q, satWatchStoreValue(uint32(i))))
	}
	b.FailAfter(-1)
	if b.Used() != before || s.Stats().ChargedBytes != before {
		t.Fatal("native watch storage charged backing", b.Used(), before)
	}
}

func TestSATSolverNativeWatchIteratorLifetimes(t *testing.T) {
	s, _ := satWatchStoreFixture(t)
	r, e := s.NewList()
	paramsMust(t, e)
	for i := uint32(0); i < 3; i++ {
		paramsMust(t, s.Push(r, satWatchStoreValue(i)))
	}
	it, e := s.Iter(r)
	paramsMust(t, e)
	paramsMust(t, s.Write(r, 0, satWatchStoreValue(8)))
	w, ok, e := it.Next()
	paramsMust(t, e)
	if !ok || w != satWatchStoreValue(8) {
		t.Fatal("borrow did not see actual writes")
	}
	paramsMust(t, s.SetEnd(r, 1))
	if _, _, e = it.Next(); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("borrow read discarded tail", e)
	}
	x, e := s.record(r)
	paramsMust(t, e)
	x.backing.data = x.backing.data[:len(x.backing.data):len(x.backing.data)]
	it, e = s.Iter(r)
	paramsMust(t, e)
	paramsMust(t, s.Push(r, satWatchStoreValue(9)))
	if _, _, e = it.Next(); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal("growth did not retire old backing", e)
	}
	it, e = s.Iter(r)
	paramsMust(t, e)
	paramsMust(t, s.Finalize(r))
	if _, _, e = it.Next(); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal("finalize retained iterator borrow", e)
	}
	paramsMust(t, s.Push(r, satWatchStoreValue(10)))
	old, e := s.Iter(r)
	paramsMust(t, e)
	paramsMust(t, s.Free(r))
	q, e := s.NewList()
	paramsMust(t, e)
	paramsMust(t, s.Push(q, satWatchStoreValue(11)))
	if s.IsLive(r) || q.slot != r.slot || q.generation == r.generation {
		t.Fatal("native list generation reused")
	}
	if _, _, e = old.Next(); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal("reused list revived old iterator", e)
	}
	if _, e = s.Read(q, 1); satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal(e)
		}
	} else if !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	paramsMust(t, s.Close())
	if _, _, e = old.Next(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
}

func TestSATSolverNativeWatchColdHeaderAndWarmIterator(t *testing.T) {
	b := arena.NewBudget(uint64(reflect.TypeOf(satSolverWatchStore{}).Size()))
	b.FailAfter(0)
	s, e := newSATSolverWatchStore(b)
	if s != nil || !errors.Is(e, arena.ErrInjectedFailure) || b.Used() != 0 {
		t.Fatal("cold refusal", s, e, b.Used())
	}
	b.FailAfter(1)
	s, e = newSATSolverWatchStore(b)
	paramsMust(t, e)
	before := b.Used()
	r, e := s.NewList()
	paramsMust(t, e)
	for i := uint32(0); i < 257; i++ {
		paramsMust(t, s.Push(r, satWatchStoreValue(i)))
	}
	if b.Used() != before {
		t.Fatal("native growth exceeded cold header budget")
	}
	if n := testing.AllocsPerRun(100, func() {
		it, e := s.Iter(r)
		if e != nil {
			panic(e)
		}
		if _, ok, e := it.Next(); e != nil || !ok {
			panic("native iterator")
		}
	}); n != 0 {
		t.Fatal("warm iterator allocation", n)
	}
	copied := *s
	if e = copied.Close(); !errors.Is(e, ErrSATOwner) || b.Used() != before {
		t.Fatal("owner copy refund", e)
	}
	paramsMust(t, s.Close())
	paramsMust(t, s.Close())
	if b.Used() != 0 {
		t.Fatal("cold header not refunded", b.Used())
	}
}

func TestSATSolverWatchStoreDisplaysActualExternalIndex(t *testing.T) {
	store, _ := satWatchStoreFixture(t)
	list, err := store.NewList()
	paramsMust(t, err)
	watch, err := newSATExternalWatch(42)
	paramsMust(t, err)
	paramsMust(t, store.Push(list, watch))
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	paramsMust(t, err)
	defer out.Close()
	paramsMust(t, store.DisplayInto(list, nil, out))
	value, err := out.String()
	paramsMust(t, err)
	if value != "42" {
		t.Fatal("external index display", value)
	}
}

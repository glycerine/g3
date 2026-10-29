package sat

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestSATClauseSetNativeCollections(t *testing.T) {
	a := satStore(t, nil)
	b := arena.NewBudget(1 << 20)
	s, e := NewSATClauseSet(a, b)
	satOK(t, e)
	defer s.Close()
	r1 := satMk(t, a, satLs(0, 2), false)
	r2 := satMk(t, a, satLs(2, 4), false)
	r3 := satMk(t, a, satLs(4, 6), false)
	satOK(t, s.Insert(r1))
	satOK(t, s.Insert(r2))
	satOK(t, s.Insert(r1))
	if s.Size() != 2 {
		t.Fatal(s.Size())
	}
	satOK(t, s.Erase(r1))
	got, e := s.Get(0)
	if got != r2 || e != nil {
		t.Fatal(got, e)
	}
	satOK(t, s.Insert(r1))
	// Native map/slice growth does not consume a simulated C++ vector budget.
	b.FailAfter(0)
	satOK(t, s.Insert(r3))
	if has, e := s.Contains(r3); !has || e != nil || s.Size() != 3 {
		t.Fatal(has, e, s.Size())
	}
	if ok, e := s.CheckInvariant(); !ok || e != nil {
		t.Fatal(ok, e)
	}
	satOK(t, s.Erase(r3))
	if has, e := s.Contains(r3); has || e != nil {
		t.Fatal(has, e)
	}
	p, e := s.Pop()
	if p != r1 || e != nil {
		t.Fatal(p, e)
	}
	satOK(t, s.Reset())
	satOK(t, s.Finalize())
	satOK(t, s.Insert(r2))
	s.Close()
	if b.Used() != 0 {
		t.Fatal(b.Used())
	}
}

func clauseSetAssertionRecord(line, condition string) string {
	return "ASSERTION VIOLATION\nFile: src/sat/sat_clause_set.cpp\nLine: " + line + "\n" + condition + "\n" +
		"(C)ontinue, (A)bort, (S)top, (T)hrow exception, Invoke (G)DB\n"
}

func TestSATClauseSetConditionalInvariantActionsPreserveMutation(t *testing.T) {
	a := satStore(t, nil)
	b := arena.NewBudget(1 << 20)
	s, err := NewSATClauseSet(a, b)
	satOK(t, err)
	defer func() { satOK(t, s.Close()) }()
	r1 := satMk(t, a, satLs(0, 2), false)
	r2 := satMk(t, a, satLs(2, 4), false)
	r3 := satMk(t, a, satLs(4, 6), false)
	satOK(t, s.Insert(r1))
	debug, input, outputFile := satIntegrityActionFixture(t, "CT")
	// The tag is initially absent, so ordinary source mutation does not run the
	// conditional invariant.
	satOK(t, s.Insert(r2))
	state, err := input.State()
	satOK(t, err)
	if state.Position != 0 {
		t.Fatal("disabled tag consumed action", state.Position)
	}
	id1, err := a.ID(r1)
	satOK(t, err)
	s.positions[id1] = 1
	satOK(t, debug.EnableDebug("clause_set"))
	satOK(t, debug.EnableAssertions(false))
	satOK(t, s.Insert(r3))
	satOK(t, s.Erase(r3))
	satOK(t, debug.EnableAssertions(true))
	beforeEpoch := s.epoch
	err = s.Insert(r3)
	if satStorageSourceDebug {
		var ex z3exception.DefaultException
		if !errors.As(err, &ex) {
			t.Fatal("conditional invariant T action", err)
		}
		if s.epoch != beforeEpoch+1 || s.Size() != 3 {
			t.Fatal("mutation not published before CASSERT", s.epoch, beforeEpoch, s.Size())
		}
		if has, e := s.Contains(r3); !has || e != nil {
			t.Fatal("inserted clause prefix", has, e)
		}
		state, err = input.State()
		satOK(t, err)
		if state.Position != 2 {
			t.Fatal("conditional action count", state.Position)
		}
		contents, e := os.ReadFile(outputFile.Name())
		satOK(t, e)
		want := clauseSetAssertionRecord("74", "m_id2pos[c.id()] == pos") + clauseSetAssertionRecord("85", "m_set[pos]->id() == id")
		if got := string(contents); got != want {
			t.Fatalf("conditional invariant records: got %q, want %q", got, want)
		}
	} else {
		satOK(t, err)
		contents, e := os.ReadFile(outputFile.Name())
		satOK(t, e)
		if len(contents) != 0 {
			t.Fatalf("release CASSERT emitted %q", contents)
		}
	}
}

func TestSATClauseSetPopAssertionPrefixesAndContinuation(t *testing.T) {
	a := satStore(t, nil)
	b := arena.NewBudget(1 << 20)
	s, err := NewSATClauseSet(a, b)
	satOK(t, err)
	defer func() { satOK(t, s.Close()) }()
	_, input, outputFile := satIntegrityActionFixture(t, "CCC")
	if _, err = s.Pop(); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("continued empty pop guard", err)
	}
	r := satMk(t, a, satLs(0, 2), false)
	satOK(t, s.Insert(r))
	id, err := a.ID(r)
	satOK(t, err)
	if satStorageSourceDebug {
		s.idExtent = 0
		if got, e := s.Pop(); got != r || !errors.Is(e, arena.ErrBounds) || s.Size() != 1 {
			t.Fatal("continued extent guard must stop unsafe index", got, e, s.Size())
		}
		s.idExtent = uint64(id) + 1
	}
	s.positions[id] = 1
	got, err := s.Pop()
	if err != nil || got != r || s.Size() != 0 {
		t.Fatal("continued pop publication", got, err, s.Size())
	}
	state, err := input.State()
	satOK(t, err)
	contents, err := os.ReadFile(outputFile.Name())
	satOK(t, err)
	if satStorageSourceDebug {
		if state.Position != 3 {
			t.Fatal("pop action count", state.Position)
		}
		want := clauseSetAssertionRecord("55", "!empty()") +
			clauseSetAssertionRecord("57", "c.id() < m_id2pos.size()") +
			clauseSetAssertionRecord("58", "m_id2pos[c.id()] == m_set.size()-1")
		if string(contents) != want {
			t.Fatalf("pop records: got %q, want %q", contents, want)
		}
	} else if state.Position != 0 || len(contents) != 0 {
		t.Fatalf("release pop actions: position=%d output=%q", state.Position, contents)
	}
	if strings.Count(string(contents), "ASSERTION VIOLATION") != map[bool]int{false: 0, true: 3}[satStorageSourceDebug] {
		t.Fatal("unexpected pop assertion count")
	}
}

func TestSATClauseSetConditionalInvariantExtentAndReverseBounds(t *testing.T) {
	for _, test := range []struct {
		name, line, condition string
		corrupt               func(*satClauseSet, uint32)
	}{
		{"extent", "73", "c.id() < m_id2pos.size()", func(s *satClauseSet, _ uint32) { s.idExtent = 0 }},
		{"reverse bounds", "84", "pos < m_set.size()", func(s *satClauseSet, id uint32) { s.positions[id+1000] = 99 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := satStore(t, nil)
			b := arena.NewBudget(1 << 20)
			s, err := NewSATClauseSet(a, b)
			satOK(t, err)
			defer func() { satOK(t, s.Close()) }()
			r := satMk(t, a, satLs(0, 2), false)
			satOK(t, s.Insert(r))
			id, err := a.ID(r)
			satOK(t, err)
			debug, input, outputFile := satIntegrityActionFixture(t, "C")
			satOK(t, debug.EnableDebug("clause_set"))
			test.corrupt(s, id)
			satOK(t, s.cassertInvariant())
			state, err := input.State()
			satOK(t, err)
			contents, err := os.ReadFile(outputFile.Name())
			satOK(t, err)
			if satStorageSourceDebug {
				if state.Position != 1 || string(contents) != clauseSetAssertionRecord(test.line, test.condition) {
					t.Fatalf("conditional source action: position=%d output=%q", state.Position, contents)
				}
			} else if state.Position != 0 || len(contents) != 0 {
				t.Fatalf("release source action: position=%d output=%q", state.Position, contents)
			}
		})
	}
}

func TestSATClauseSetCopyMoveAndHeaderFailure(t *testing.T) {
	a := satStore(t, nil)
	b := arena.NewBudget(1 << 20)
	src, e := NewSATClauseSet(a, b)
	satOK(t, e)
	defer src.Close()
	r := satMk(t, a, satLs(0, 2), false)
	satOK(t, src.Insert(r))
	dst, e := NewSATClauseSet(a, b)
	satOK(t, e)
	defer dst.Close()
	b.FailAfter(0)
	if _, e = src.Clone(b); !errors.Is(e, arena.ErrInjectedFailure) {
		t.Fatal("clone must charge its owner header", e)
	}
	// Assignment copies native collections and needs no extra source allocation.
	satOK(t, dst.Assign(src))
	b.FailAfter(-1)
	copy, e := src.Clone(b)
	satOK(t, e)
	defer copy.Close()
	satOK(t, dst.MoveAssign(copy))
	if copy.Size() != 1 || dst.Size() != 1 {
		t.Fatal("move assignment copies the set")
	}
	moved, e := copy.MoveClone(b)
	satOK(t, e)
	defer moved.Close()
	if copy.Size() != 0 || moved.Size() != 1 {
		t.Fatal("move constructor")
	}
	satOK(t, a.Close())
	raw, e := src.Clone(b)
	satOK(t, e)
	defer raw.Close()
	if got, e := raw.Get(0); got != r || e != nil {
		t.Fatal("copy must not dereference a dead borrowed clause", got, e)
	}
}

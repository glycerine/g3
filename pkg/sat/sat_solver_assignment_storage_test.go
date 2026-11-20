package sat

// Source: src/sat/sat_solver.h; src/sat/sat_solver.cpp:135-160,1765-1865.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func satRuntimeActualRefusal(r *umemory.Runtime, e error) bool {
	if errors.Is(e, arena.ErrInjectedFailure) {
		return true
	}
	var f umemory.Failure
	return errors.As(e, &f) && f.ErrorCode() == 101 && r.State().OutOfMemory
}

// Enumerate only declared owning roots; never infer ownership from live entries.
func satRuntimeAuditBlocks(t *testing.T, r *umemory.Runtime, external ...umemory.Block) {
	t.Helper()
	roots := external
	type identity struct{ slot, generation uint32 }
	expected := map[identity]bool{}
	rv := reflect.ValueOf(r).Elem()
	origin := testFieldBySourceName(rv, "origin").Uint()
	for _, h := range roots {
		hv := reflect.ValueOf(h)
		if testFieldBySourceName(hv, "origin").Uint() != origin {
			t.Fatal("foreign declared root")
		}
		id := identity{uint32(testFieldBySourceName(hv, "slot").Uint()), uint32(testFieldBySourceName(hv, "generation").Uint())}
		if expected[id] {
			t.Fatal("duplicate owning root", id)
		}
		expected[id] = true
		_, e := r.Size(h)
		satOK(t, e)
	}
	records := testFieldBySourceName(rv, "blocks")
	var physical, init uint64
	live := 0
	for i := 0; i < records.Len(); i++ {
		b := records.Index(i)
		if !testFieldBySourceName(b, "live").Bool() {
			continue
		}
		id := identity{uint32(i + 1), uint32(testFieldBySourceName(b, "generation").Uint())}
		if !expected[id] {
			t.Fatal("anonymous raw allocation", id)
		}
		delete(expected, id)
		live++
		physical += uint64(testFieldBySourceName(b, "data").Len())
		init += uint64(testFieldBySourceName(b, "initialized").Len()) * 8
	}
	st := r.Stats()
	if len(expected) != 0 || uint32(live) != st.LiveBlocks || physical != st.PhysicalBytes || init != st.InitializationBytes {
		t.Fatal("actual raw graph", expected, st, physical, init)
	}
}

func TestSATSolverNativeAssignmentCurrentBorrowAndOwnerGuards(t *testing.T) {
	v, _, b := satVariableStateFixture(t)
	_, e := v.mkVar(false, true)
	satOK(t, e)
	view := v.clauseState()
	before := b.Used()
	raw := v.memory.Stats()
	b.FailAfter(0)
	satOK(t, v.setAssignment(0, lbool.True))
	satOK(t, v.setAssignment(1, lbool.False))
	b.FailAfter(-1)
	if b.Used() != before || v.memory.Stats() != raw {
		t.Fatal("native assignment changed allocator state")
	}
	x, e := view.value(0)
	satOK(t, e)
	if x != lbool.True {
		t.Fatal("view lost current assignment", x)
	}
	// Force native backing growth through actual variable creation, then change
	// an old literal. The clause view must read the owner's current slice.
	for i := 0; i < 100; i++ {
		_, e = v.mkVar(false, true)
		satOK(t, e)
	}
	satOK(t, v.setAssignment(0, lbool.False))
	x, e = view.value(0)
	satOK(t, e)
	if x != lbool.False {
		t.Fatal("view retained old slice", x)
	}
	copied := *v
	if _, e = copied.valueLiteral(0); !errors.Is(e, ErrSATOwner) {
		t.Fatal("copied owner accepted", e)
	}
	_, e = v.valueLiteral(Literal(len(v.assignment)))
	want := arena.ErrBounds
	if satStorageSourceDebug {
		want = ErrSATAssertion
	}
	if !errors.Is(e, want) {
		t.Fatal("assignment bounds", e)
	}
	satScalarAuditRoots(t, v)
	satOK(t, v.Close())
	if _, e = view.value(0); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("closed borrow accepted", e)
	}
}
func TestSATSolverAssignmentRequiresSameActualCheckpointRuntime(t *testing.T) {
	s, v, b := satPropagationFixture(t, 1)
	foreign, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b})
	satOK(t, e)
	before := b.Used()
	got, e := newSATSolverPropagationState(v, s.clauses, s.converter, s.limit, foreign, s.config, b)
	if got != nil || !errors.Is(e, ErrSATOwner) || b.Used() != before {
		t.Fatal("foreign checkpoint domain", got, e, b.Used(), before)
	}
	satOK(t, foreign.Close())
}

func TestSATBothPolaritiesPreservesPartialWrite(t *testing.T) {
	v, _, _ := satVariableStateFixture(t)
	_, err := v.mkVar(false, true)
	satOK(t, err)
	full := v.assignment
	v.assignment = full[:1]
	defer func() { v.assignment = full }()
	want := error(arena.ErrBounds)
	if satStorageSourceDebug {
		want = ErrSATAssertion
	}
	if err = v.setBothPolarities(0, lbool.True); !errors.Is(err, want) || v.assignment[0] != lbool.True {
		t.Fatal("missing opposite slot", err, v.assignment)
	}
	if err = v.setBothPolarities(1, lbool.True); !errors.Is(err, want) || v.assignment[0] != lbool.True {
		t.Fatal("missing first slot", err, v.assignment)
	}
}

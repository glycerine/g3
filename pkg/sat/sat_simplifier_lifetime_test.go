package sat

// Source: src/sat/sat_simplifier.cpp register_clauses/scoped_finalize; Go owning abort.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestSATSimplifierClauseNativeStableSizeSortAndOwnership(t *testing.T) {
	s, l := satSimplifierFixture(t, 5)
	lengths := []uint32{3, 4, 3, 5}
	refs := make([]StoredClauseRef, len(lengths))
	for i, n := range lengths {
		lits := []Literal{0, 2, 4, 6, 8}[:n]
		c, e := l.MkClauseCore(n, lits, false)
		satOK(t, e)
		refs[i] = c
	}
	before := l.budget.Used()
	l.budget.FailAfter(0)
	satOK(t, s.sortClauses(false))
	l.budget.FailAfter(-1)
	if l.budget.Used() != before {
		t.Fatal("native clause sort charged arena workspace", before, l.budget.Used())
	}
	for i, want := range []StoredClauseRef{refs[3], refs[1], refs[0], refs[2]} {
		if got := l.Original[i]; got != want {
			t.Fatal("stable descending clause-size order", i, got, want)
		}
	}
	retained := append([]StoredClauseRef(nil), l.Original...)
	l.Original[2] = StoredClauseRef{}
	if e := s.sortClauses(false); e == nil {
		t.Fatal("invalid checked clause size accepted")
	}
	for i, want := range retained {
		if i == 2 {
			want = StoredClauseRef{}
		}
		if l.Original[i] != want {
			t.Fatal("failed size read reordered owning vector", i)
		}
	}
	l.Original[2] = retained[2]
	satSimplifierAudit(t, s)
}

func TestSATSimplifierDestructorActualExplicitFinalizeBeforeMembers(t *testing.T) {
	s, l := satSimplifierFixture(t, 4)
	todo := l.propagation.variables.elimTodo
	if todo.Empty() || todo.ValuesCapacity() == 0 {
		t.Fatal("real MkVar todo setup")
	}
	satOK(t, s.Close())
	if !todo.Empty() || todo.ValuesCapacity() != 0 || todo.MembershipCapacity() != 0 {
		t.Fatal("source destructor finalize must reach actual borrowed elim_todo before member teardown")
	}
}

func TestSATSimplifierColdCopyClosedAndWarmEmptyPhysicalPlateau(t *testing.T) {
	s, l := satSimplifierFixture(t, 0)
	copyOwner := *s
	if e := copyOwner.Run(false); !errors.Is(e, ErrSATOwner) {
		t.Fatal(e)
	}
	if e := copyOwner.Close(); !errors.Is(e, ErrSATOwner) {
		t.Fatal(e)
	}
	if e := copyOwner.Finalize(); !errors.Is(e, ErrSATOwner) {
		t.Fatal("copied finalize must not release actual member owners", e)
	}
	satOK(t, s.Run(false))
	satOK(t, s.Run(false))
	satOK(t, s.Run(false))
	before := l.budget.Used()
	rawBefore := l.propagation.memory.Stats()
	var runErr error
	allocs := testing.AllocsPerRun(20, func() { runErr = s.Run(false) })
	satOK(t, runErr)
	if allocs != 0 || l.budget.Used() != before || l.propagation.memory.Stats().TotalBytes != rawBefore.TotalBytes {
		t.Fatal("warmed empty actual source path", allocs, before, l.budget.Used())
	}
	satSimplifierAudit(t, s)
	satOK(t, s.Close())
	if e := s.Run(false); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
}

// Explicit checked dangling-member domain: the caller releases the actual
// descriptor behind the borrowed source field. This is not a fake free refusal.
func TestSATSimplifierLateFinalizeGuardDoesNotReplayOuterPhases(t *testing.T) {
	s, l := satSimplifierFixture(t, 0)
	satOK(t, s.initialize())
	s.scopeActive = true
	s.running = false
	debug, _, _ := satIntegrityActionFixture(t, "")
	satOK(t, debug.EnableDebug("sat_solver"))
	l.simplifierContinuation = s
	satOK(t, s.literalStore.Free(s.bsLS))
	before := l.propagation.limit.Count()
	e := s.scopedFinalize()
	if !errors.Is(e, arena.ErrInvalidSlot) || s.scopedStage != 4 || s.finalizeStage != 6 || !s.scopeActive {
		t.Fatal("real late guard and completed cursor", e, s.scopedStage, s.finalizeStage)
	}
	satSimplifierAudit(t, s)
	// Caller acknowledgment of its own external destruction, not reconstruction
	// or a repair of live source values. Owning Close now resumes only cleanup.
	s.bsLS = satSolverLiteralListRef{}
	satOK(t, s.Close())
	want := uint64(0)
	if satStorageSourceDebug {
		want = 1
	}
	if l.propagation.limit.Count()-before != want {
		t.Fatal("completed invariant was replayed after later Finalize guard")
	}
}

package sat

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Source: src/sat/sat_solver.cpp:1825. The trace observes the mark before
// the function mutates it, including an already-marked return.
func TestSATUnsatCoreAntecedentSourceTraceBeforeMark(t *testing.T) {
	s := satConflictFixture(t, 1)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("sat")
	s.traceTags = tags
	satOK(t, s.processAntecedentForUnsatCore(0))
	satOK(t, s.processAntecedentForUnsatCore(0))
	marked, e := s.analysis.marked(0)
	satOK(t, e)
	if !marked {
		t.Fatal("antecedent was not marked")
	}
	satOK(t, s.analysis.resetUnmark(0))
	want := ""
	if tags.ShouldTrace("sat") {
		want = satClauseTraceRecord("sat", "process_antecedent_for_unsat_core", "1825", "0 -\n") +
			satClauseTraceRecord("sat", "process_antecedent_for_unsat_core", "1825", "0 +\n")
	}
	if got := output.String(); got != want {
		t.Fatalf("antecedent trace = %q, want %q", got, want)
	}
}

// Source: src/sat/sat_solver.cpp:1885-1894,1940. The non-null conflicting
// literal precedes resolution, and the trail prints after clause publication.
func TestSATConflictInitNotLiteralSourceTrace(t *testing.T) {
	s := satConflictFixture(t, 2)
	p := s.analysis.propagation
	satConflictPush(t, s)
	satOK(t, p.assign(0, satJustification{}))
	satOK(t, p.assign(2, newSATBinaryJustification(1)))
	p.setConflict(newSATBinaryJustification(1), 2)
	s.conflictLevel = 1
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("sat")
	s.traceTags = tags
	ok, e := s.resolveConflictForInit()
	satOK(t, e)
	if !ok {
		t.Fatal("source init resolution did not complete")
	}
	want := ""
	if tags.ShouldTrace("sat") {
		want = satClauseTraceRecord("sat", "resolve_conflict_for_init", "1892", "not_l: 1\n") +
			satClauseTraceRecord("sat", "resolve_conflict_for_init", "1940", "Trail: -0\n")
	}
	if got := output.String(); got != want {
		t.Fatalf("init trace = %q, want %q", got, want)
	}
}

// Source: src/sat/sat_solver.cpp:1932,1940. The full solver emits the lemma,
// solver display, and pre-backtrack assignment before the reduced trail.
func TestSATConflictInitFullSolverLemmaAndAssignmentTrace(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(true, true)
		satOK(t, err)
	}
	p := s.propagation
	satOK(t, s.Lifecycle.push())
	satOK(t, p.assign(0, satJustification{}))
	satOK(t, p.assign(2, newSATBinaryJustification(1)))
	p.setConflict(newSATBinaryJustification(1), 2)
	s.conflict.conflictLevel = 1
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("sat")
	s.traceTags = tags
	s.conflict.traceTags = tags
	ok, err := s.conflict.resolveConflictForInit()
	satOK(t, err)
	if !ok {
		t.Fatal("source init conflict was not resolved")
	}
	want := ""
	if tags.ShouldTrace("sat") {
		want = satClauseTraceRecord("sat", "resolve_conflict_for_init", "1892", "not_l: 1\n") +
			satClauseTraceRecord("sat", "resolve_conflict_for_init", "1932", "lemma: -0\n(sat\n)\nassignment:\n0 1\n") +
			satClauseTraceRecord("sat", "resolve_conflict_for_init", "1940", "Trail: -0\n")
	}
	if got := output.String(); got != want {
		t.Fatalf("source conflict-init trace = %q, want %q", got, want)
	}
}

func TestSATCoreClauseReleasedDuringAntecedentTrace(t *testing.T) {
	for _, tc := range []struct {
		name       string
		consequent Literal
	}{
		{"conflict", DefaultLiteral()},
		{"second-watch", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := satConflictFixture(t, 4)
			a := s.analysis.propagation.clauses
			var c StoredClauseRef
			released := false
			writer := satWatchTraceWriter(func(b []byte) (int, error) {
				if !released && bytes.Contains(b, []byte("process_antecedent_for_unsat_core")) {
					released = true
					satOK(t, a.Delete(c))
				}
				return len(b), nil
			})
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("sat")
			if !tags.ShouldTrace("sat") {
				t.Skip("requires trace build")
			}
			s.traceTags = tags
			var err error
			c, err = a.MkClause([]Literal{0, 2, 4, 6}, false)
			satOK(t, err)
			off, err := a.Offset(c)
			satOK(t, err)
			err = s.processConsequent(tc.consequent, newSATClauseJustification(off), 2, nil)
			if !released || err != arena.ErrInvalidSlot {
				t.Fatalf("released = %t, next read = %v", released, err)
			}
			n, err := s.analysis.unmark.Size()
			satOK(t, err)
			if n != 1 {
				t.Fatalf("processed %d antecedents, want 1", n)
			}
			satOK(t, s.analysis.resetUnmark(0))
		})
	}
}

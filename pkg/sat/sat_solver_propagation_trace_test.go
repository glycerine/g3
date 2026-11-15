package sat

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type satPropagationTraceObserver struct {
	output                                    bytes.Buffer
	state                                     *satSolverPropagationState
	assignmentSeen, assignmentBeforeWrite     bool
	propagationSeen, propagationBeforeAdvance bool
}

func (w *satPropagationTraceObserver) Write(p []byte) (int, error) {
	text := string(p)
	if strings.HasPrefix(text, "0 binary 1 level: 0") {
		w.assignmentSeen = true
		v, err := w.state.variables.valueLiteral(0)
		w.assignmentBeforeWrite = err == nil && v == lbool.Undef
	}
	if strings.HasPrefix(text, "propagating: 0 none") {
		w.propagationSeen = true
		w.propagationBeforeAdvance = w.state.variables.qhead == 0
	}
	return w.output.Write(p)
}

// Source: src/sat/sat_solver.cpp:506,564. Assignment prints its incoming
// justification before level-zero erasure; propagation prints the stored one.
func TestSATAssignmentAndPropagationSourceTraceOrder(t *testing.T) {
	p, v, _ := satPropagationFixture(t, 3)
	w := &satPropagationTraceObserver{state: p}
	tags := diagnostic.NewTraceTagsWithWriter(w)
	tags.Enable("sat_assign_core")
	tags.Enable("sat_propagate")
	p.traceTags = tags
	satOK(t, p.assignCore(0, newSATBinaryJustification(2)))
	_, err := p.propagateCore(false)
	satOK(t, err)
	j, err := v.justification.read(0)
	satOK(t, err)
	if !j.None() || v.qhead != 1 {
		t.Fatal("source level-zero assignment and propagation")
	}
	v.scopeLevel = 2
	scoped := newSATTernaryJustification(0, 5)
	satOK(t, p.assignCore(2, scoped))
	_, err = p.propagateCore(false)
	satOK(t, err)
	j, err = v.justification.read(1)
	satOK(t, err)
	if j != scoped || v.qhead != 2 {
		t.Fatal("scoped justification was not retained")
	}
	if !tags.ShouldTrace("sat_assign_core") {
		if w.output.Len() != 0 || w.assignmentSeen || w.propagationSeen {
			t.Fatal("erased propagation TRACE emitted")
		}
		return
	}
	want := satClauseTraceRecord("sat_assign_core", "assign_core", "506", "0 binary 1 level: 0\n") +
		satClauseTraceRecord("sat_propagate", "propagate_core", "564", "propagating: 0 none\n") +
		satClauseTraceRecord("sat_assign_core", "assign_core", "506", "1 ternary 0 -2 level: 2\n") +
		satClauseTraceRecord("sat_propagate", "propagate_core", "564", "propagating: 1 ternary 0 -2\n")
	if got := w.output.String(); got != want || !w.assignmentSeen || !w.assignmentBeforeWrite || !w.propagationSeen || !w.propagationBeforeAdvance {
		t.Fatalf("propagation trace got %q, want %q; assign-before=%v propagation-before=%v", got, want, w.assignmentBeforeWrite, w.propagationBeforeAdvance)
	}
}

// Source: src/sat/sat_solver.cpp:622,632,635. The blocked path prints its
// clause without processing it; processing prints before the watch swap, and
// the conditional mismatch prints after it.
func TestSATWatchedClauseSourceTraceBranches(t *testing.T) {
	for _, route := range []struct {
		name, want string
	}{
		{"blocked", satClauseTraceRecord("propagate_clause_bug", "propagate_core", "622", "blocked literal 2\n(1 -0 2)+\n")},
		{"removed", satClauseTraceRecord("propagate_clause_bug", "propagate_core", "632", "processing... (1 -0 2)x+\nwas_removed: 1\n")},
		{"mismatch", satClauseTraceRecord("propagate_clause_bug", "propagate_core", "632", "processing... (1 -2 2)+\nwas_removed: 0\n") + satClauseTraceRecord("propagate_bug", "propagate_core", "635", "l: 0 (1 -2 2)+\n")},
		{"swap", satClauseTraceRecord("propagate_clause_bug", "propagate_core", "632", "processing... (-0 1 2)+\nwas_removed: 0\n")},
	} {
		t.Run(route.name, func(t *testing.T) {
			s, _, _ := satPropagationFixture(t, 3)
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			tags.Enable("propagate_clause_bug")
			tags.Enable("propagate_bug")
			s.traceTags = tags
			lits := []Literal{2, 1, 4}
			if route.name == "swap" {
				lits[0], lits[1] = lits[1], lits[0]
			}
			c, e := s.clauses.MkClause(lits, false)
			satOK(t, e)
			off, e := s.clauses.Offset(c)
			satOK(t, e)
			switch route.name {
			case "blocked":
				satOK(t, s.assign(4, satJustification{}))
			case "removed":
				satOK(t, s.clauses.SetRemoved(c, true))
			case "mismatch":
				satOK(t, s.clauses.SetLiteral(c, 1, 5))
			}
			w, e := newSATClauseWatch(4, off)
			satOK(t, e)
			satPropagationPush(t, s, 0, w)
			satOK(t, s.assign(0, satJustification{}))
			_, e = s.propagateCore(false)
			satOK(t, e)
			want := route.want
			if !tags.ShouldTrace("propagate_clause_bug") {
				want = ""
			}
			if got := output.String(); got != want {
				t.Fatalf("trace = %q, want %q", got, want)
			}
		})
	}
}

// Source: src/sat/sat_solver.h:259 (AUDIT.md sat_02-F3). solver::assign
// prints the literal and its previous value before the switch, for every
// outcome: undef assigns, true returns, false sets the conflict.
func TestSATAssignSourceTraceRecord(t *testing.T) {
	p, _, _ := satPropagationFixture(t, 2)
	var w bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&w)
	tags.Enable("sat_assign")
	p.traceTags = tags
	satOK(t, p.assign(0, satJustification{}))
	satOK(t, p.assign(0, satJustification{}))
	satOK(t, p.assign(1, satJustification{}))
	if !p.inconsistent {
		t.Fatal("assigning the negation of a true literal must set the conflict")
	}
	if !tags.ShouldTrace("sat_assign") {
		if w.Len() != 0 {
			t.Fatal("erased sat_assign TRACE emitted")
		}
		return
	}
	record := func(body string) string {
		return "-------- [sat_assign] assign src/sat/sat_solver.h:259 ---------\n" + body + "------------------------------------------------\n"
	}
	want := record("0 previous value: l_undef\n") + record("0 previous value: l_true\n") + record("-0 previous value: l_false\n")
	if got := w.String(); got != want {
		t.Fatalf("sat_assign trace got %q, want %q", got, want)
	}
}

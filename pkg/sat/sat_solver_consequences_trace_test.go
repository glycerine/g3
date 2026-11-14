package sat

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: src/sat/sat_solver.cpp:3285-3297. Each antecedent row is printed
// before union, and the final core follows index_set slot order.
func TestSATConsequenceCoreFixupSourceTraceOrder(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	satOK(t, s.conflict.core.Push(2))
	satOK(t, s.conflict.core.Push(4))
	var first, second satSolverIndexSet
	satOK(t, first.insert(0))
	satOK(t, first.insert(3))
	satOK(t, second.insert(1))
	satOK(t, s.antecedents.insert(1, &first))
	satOK(t, s.antecedents.insert(2, &second))
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("sat")
	s.traceTags = tags
	satOK(t, s.fixupConsequenceCore())
	if n, e := s.conflict.core.Size(); e != nil || n != 3 {
		t.Fatal(n, e)
	}
	for i, want := range []uint32{0, 1, 3} {
		got, e := s.conflict.core.Read(uint32(i))
		if e != nil || got != want {
			t.Fatal(i, got, want, e)
		}
	}
	want := ""
	if tags.ShouldTrace("sat") {
		want = satClauseTraceRecord("sat", "fixup_consequence_core", "3287", "1 2\n") +
			satClauseTraceRecord("sat", "fixup_consequence_core", "3289", "1: 0 -1 \n") +
			satClauseTraceRecord("sat", "fixup_consequence_core", "3289", "2: -0 \n") +
			satClauseTraceRecord("sat", "fixup_consequence_core", "3297", "0 -0 -1\n")
	}
	if got := output.String(); got != want {
		t.Fatalf("fixup trace = %q, want %q", got, want)
	}
}

// Source: src/sat/sat_solver.cpp:3516,3538,3585. Fixed-set scanning prints
// each literal before the level gate; reason extraction prints before and
// after its source justification switch.
func TestSATConsequenceExtractionSourceTrace(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 2; i++ {
		_, e := s.MkVar(true, true)
		satOK(t, e)
	}
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("sat")
	s.traceTags = tags
	satOK(t, s.Variables.setAssignment(0, lbool.False))
	var lits, assumptions, unfixed satMUSLiteralSet
	t.Cleanup(func() { lits.close(); assumptions.close(); unfixed.close() })
	satOK(t, lits.insert(0))
	satOK(t, lits.insert(2))
	rows, e := newSATSolverLiteralRows(s.runtime.memory, s.budget)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, rows.CloseChecked()) })
	satOK(t, s.extractFixedSet(&lits, &assumptions, &unfixed, rows))
	var indices satSolverIndexSet
	t.Cleanup(func() { satOK(t, indices.close()) })
	satOK(t, indices.insert(1))
	complete, e := s.extractConsequenceAssumptions(0, &indices)
	satOK(t, e)
	if !complete {
		t.Fatal("source NONE reason must finish")
	}
	want := ""
	if tags.ShouldTrace("sat") {
		want = satClauseTraceRecord("sat", "extract_fixed_consequences", "3516", "extract: 0 l_false 4294967295\n") +
			satClauseTraceRecord("sat", "extract_fixed_consequences", "3516", "extract: 1 l_undef 4294967295\n") +
			satClauseTraceRecord("sat", "extract_assumptions", "3538", "0 none\n") +
			satClauseTraceRecord("sat", "extract_assumptions", "3585", "0: -0 \n")
	}
	if got := output.String(); got != want {
		t.Fatalf("extraction trace = %q, want %q", got, want)
	}
}

// Source: src/sat/sat_solver.cpp:3307,3366. The same borrowed assumptions
// reach either the bounded or ordinary consequence body before map reset.
func TestSATConsequenceEntryAssumptionSourceTrace(t *testing.T) {
	for route := 0; route < 2; route++ {
		t.Run([]string{"ordinary", "bounded"}[route], func(t *testing.T) {
			s, out := satConsequenceFaultFixture(t, route)
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			tags.Enable("sat")
			s.traceTags = tags
			_, err := s.getConsequences([]Literal{0}, []BoolVar{1, 2, 3, 4}, out)
			satOK(t, err)
			line, function := "3366", "get_consequences"
			if route == 1 {
				line, function = "3307", "get_bounded_consequences"
			}
			record := satClauseTraceRecord("sat", function, line, "0\n")
			count := strings.Count(output.String(), record)
			if tags.ShouldTrace("sat") {
				if count != 1 {
					t.Fatalf("assumption record count = %d in %q", count, output.String())
				}
			} else if output.Len() != 0 {
				t.Fatalf("erased TRACE emitted %q", output.String())
			}
		})
	}
}

// Source: src/sat/sat_solver.cpp:3420,3450. These conditional records use the
// same live solver display as other SAT traces; the consequence search must
// still supply a separate fixture that reaches both branches end to end.
func TestSATConsequenceConditionalSourceTraceBodies(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	_, err := s.MkVar(true, true)
	satOK(t, err)
	var display bytes.Buffer
	satOK(t, s.traceUserDisplayInto(&display))
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("sat")
	s.traceTags = tags
	s.traceConsequenceInconsistent()
	s.traceConsequenceUnsat()
	want := ""
	if tags.ShouldTrace("sat") {
		want = satClauseTraceRecord("sat", "get_consequences", "3420", "inconsistent\n"+display.String()) +
			satClauseTraceRecord("sat", "get_consequences", "3450", "unsat\n")
	}
	if got := output.String(); got != want {
		t.Fatalf("consequence branch TRACE = %q, want %q", got, want)
	}
}

package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func lraTraceEvidenceFixture(t *testing.T, truth bool) (*lraFrontendFixture, []smtLRAEvidence, string) {
	t.Helper()
	f := newLRAFrontendFixture(t, true)
	x := f.variable("evidence-x")
	f.assert(f.app(smtArithOpLE, x, f.number(3)), truth)
	f.propagate(true)
	lit := tStateLiteralAt(t, f, 0)
	// The printer must consult the source constraint directory rather than the
	// copied literal field, which is not the authoritative source tag.
	ev := []smtLRAEvidence{{constraint: 0, literal: SMTNullLiteral, coefficient: lraQ(2, 1)}, {constraint: 0, literal: SMTNullLiteral, coefficient: lraQ(-3, 2)}}
	expr := "(<= evidence-x 3.0)"
	kind := "<="
	if !truth {
		expr = "(not " + expr + ")"
		kind = ">"
	}
	if lit.Sign() == truth {
		t.Fatal("fixture polarity", lit, truth)
	}
	want := expr + " l_true\n" + expr + " l_true\n2: v0 " + kind + " 3\n-3/2: v0 " + kind + " 3\n"
	return f, ev, want
}
func tStateLiteralAt(t *testing.T, f *lraFrontendFixture, index int) SMTLiteral {
	t.Helper()
	if len(f.a.numeric.inequalities) <= index {
		t.Fatal("missing numerical constraint")
	}
	return f.a.numeric.inequalities[index]
}

func TestSMTLRAEvidenceDisplaySignedLiteralsAndTwoPassOrder(t *testing.T) {
	for _, truth := range []bool{true, false} {
		f, ev, want := lraTraceEvidenceFixture(t, truth)
		var out bytes.Buffer
		astTestMust(t, f.a.displayEvidence(&out, ev))
		if out.String() != want {
			t.Fatal(truth, out.String(), want)
		}
	}
}

func TestSMTLRAEvidenceDisplayZeroSecondPassAndWriterPrefixes(t *testing.T) {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := d.AssertionsEnabled()
	astTestMust(t, err)
	astTestMust(t, d.EnableAssertions(false))
	defer func() { astTestMust(t, d.EnableAssertions(enabled)) }()
	f, ev, _ := lraTraceEvidenceFixture(t, true)
	ev = []smtLRAEvidence{{constraint: 0, literal: SMTNullLiteral, coefficient: lraQ(0, 1)}, ev[0]}
	var out bytes.Buffer
	astTestMust(t, f.a.displayEvidence(&out, ev))
	if out.String() != "(<= evidence-x 3.0) l_true\n0: v0 <= 3\n2: v0 <= 3\n" {
		t.Fatal(out.String())
	}
	_, _, want := lraTraceEvidenceFixture(t, false)
	for cut := 0; cut < len(want); cut++ {
		t.Run(fmt.Sprint(cut), func(t *testing.T) {
			f, ev, want := lraTraceEvidenceFixture(t, false)
			w := &smtLRAOutputPrefix{remaining: cut}
			if err := f.a.displayEvidence(w, ev); !errors.Is(err, io.ErrClosedPipe) || w.text.String() != want[:cut] {
				t.Fatal(cut, err, w.text.String(), want[:cut])
			}
		})
	}
}

func TestSMTLRAConflictTraceActualEvidenceAndBadStream(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		var out bytes.Buffer
		var writer io.Writer = &out
		if refuse {
			writer = &smtLRAOutputPrefix{remaining: 0}
		}
		tags := diagnostic.NewTraceTagsWithWriter(writer)
		tags.Enable("arith")
		f.a.trace = tags
		astTestMust(t, f.s.PushScope())
		x, y := f.variable("trace-x"), f.variable("trace-y")
		f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
		f.assert(f.app(smtArithOpGE, y, f.number(0)), true)
		f.assert(f.app(smtArithOpLE, f.app(smtArithOpAdd, x, y), f.number(-1)), true)
		f.propagate(false)
		if f.a.statistics.conflicts != 1 || f.a.conflictCount != 1 {
			t.Fatal("numerical conflict not reached", f.a.statistics.conflicts, f.a.conflictCount)
		}
		if tags.ShouldTrace("arith") && !refuse {
			text := out.String()
			first := strings.Index(text, "set_conflict src/smt/theory_lra.cpp:2222")
			second := strings.Index(text, "set_conflict src/smt/theory_lra.cpp:2223")
			if first < 0 || second <= first || !strings.Contains(text, "scope: 1\n") || strings.Count(text, " l_true\n") != 3 {
				t.Fatal(text)
			}
			if !strings.Contains(text, ", def: (+ trace-x trace-y)") {
				t.Fatal("state display missing", text)
			}
		} else if out.Len() != 0 {
			t.Fatal("erased/refused trace", out.String())
		}
		if tags.ShouldTrace("arith") && len(f.a.modelValues) != 3 {
			t.Fatal("later Display cache operands did not run", len(f.a.modelValues))
		}
		if !tags.ShouldTrace("arith") && len(f.a.modelValues) != 0 {
			t.Fatal("erased trace executed cache operands")
		}
		// A bad ostream must leave the conflict and later source operands intact.
		if f.a.numeric.stopped {
			t.Fatal("trace refusal stopped numerical owner")
		}
	}
}

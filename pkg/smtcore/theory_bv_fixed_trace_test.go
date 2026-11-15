package smtcore

// Source: src/smt/theory_bv.cpp:321,337-350,471-473.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSMTBVInitWatchAndFixedEqualitySourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, state, theory := bvOccurrenceFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	for _, tag := range []string{"init_bits", "find_wpos", "fixed_var_eh"} {
		tags.Enable(tag)
	}
	theory.trace = tags
	left, leftVariable := bvFixedSymbolic(t, m, state, theory, "trace-fixed-left", 2)
	right, rightVariable := bvFixedSymbolic(t, m, state, theory, "trace-fixed-right", 2)
	sum, err := m.MkBitVectorAdd(left.owner, right.owner)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(sum, false))

	bvFixedAssign(t, state, theory, leftVariable, 1)
	bvFixedAssign(t, state, theory, rightVariable, 1)
	astTestMust(t, theory.findWatchedPosition(leftVariable))
	astTestMust(t, theory.findWatchedPosition(rightVariable))

	if tags.ShouldTrace("init_bits") {
		trace := output.String()
		for _, want := range []string{
			"-------- [init_bits] init_bits src/smt/theory_bv.cpp:321 ---------\nbit 0 of #",
			"-------- [find_wpos] find_wpos src/smt/theory_bv.cpp:337 ---------\ncurr bit: ",
			"-------- [find_wpos] find_wpos src/smt/theory_bv.cpp:339 ---------\nmoved wpos of v",
			"-------- [find_wpos] find_wpos src/smt/theory_bv.cpp:350 ---------\nv0 is a fixed variable.\n",
			"-------- [fixed_var_eh] fixed_var_eh src/smt/theory_bv.cpp:471 ---------\ndetected equality: v1 = v0\n",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing BV fixed-bit source trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out BV fixed-bit source trace executed", output.String())
	}
}

// AUDIT.md smtcore_10-F10: init_bits prints the simplified bit with
// mk_ll_pp, so a negated bit is "#a := (bit2bool[i] x)\n(not #a)\n" followed
// by the TRACE's own newline.
func TestSMTBVInitBitsLowLevelTrace(t *testing.T) {
	var output bytes.Buffer
	m, state, theory := bvOccurrenceFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("init_bits")
	theory.trace = tags
	sort, err := m.BitVectorSort(2)
	astTestMust(t, err)
	word, err := m.MkConst(astTestSymbol(t, m, "trace-init-not"), sort)
	astTestMust(t, err)
	negated, err := m.MkBitVectorNot(word)
	astTestMust(t, err)
	astTestPin(t, m, negated)
	astTestMust(t, state.Internalize(negated, false))
	if !tags.ShouldTrace("init_bits") {
		if output.Len() != 0 {
			t.Fatal("compiled-out init_bits trace executed", output.String())
		}
		return
	}
	id, err := m.GetASTID(negated)
	astTestMust(t, err)
	header := fmt.Sprintf("-------- [init_bits] init_bits src/smt/theory_bv.cpp:321 ---------\nbit 0 of #%d\n", id)
	trace := output.String()
	at := strings.Index(trace, header)
	if at < 0 {
		t.Fatalf("missing init_bits record for #%d in %q", id, trace)
	}
	body := trace[at+len(header):]
	var bitID int
	if _, err := fmt.Sscanf(body, "#%d := (bit2bool[0] trace-init-not)\n", &bitID); err != nil {
		t.Fatalf("init_bits body is not mk_ll_pp: %q", body)
	}
	want := fmt.Sprintf("#%d := (bit2bool[0] trace-init-not)\n(not #%d)\n\n", bitID, bitID)
	if !strings.HasPrefix(body, want) {
		t.Fatalf("init_bits body %q, want prefix %q", body, want)
	}
}

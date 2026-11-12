package smtcore

// Source: src/smt/theory_bv.cpp:1203-1245,1269-1272,1424-1427.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSMTBVBitPropagationSourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, state, theory := bvOccurrenceFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	for _, tag := range []string{"bv_bit_prop", "assign_bit_bug"} {
		tags.Enable(tag)
	}
	theory.trace = tags

	p := smtRelevancyBool(t, m, "trace-propagate-p")
	q := smtRelevancyBool(t, m, "trace-propagate-q")
	left, leftVariable := bvNativeWord(t, m, state, theory, p)
	right, rightVariable := bvNativeWord(t, m, state, theory, q)
	astTestMust(t, theory.NewEquality(leftVariable, rightVariable))
	pLiteral, err := state.Literal(p)
	astTestMust(t, err)
	qLiteral, err := state.Literal(q)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(pLiteral, NewSMTBooleanAxiom()))
	astTestMust(t, theory.Assign(pLiteral.Var(), true))

	if tags.ShouldTrace("bv_bit_prop") {
		leftID, err := left.OwnerID()
		astTestMust(t, err)
		rightID, err := right.OwnerID()
		astTestMust(t, err)
		pID, err := m.GetASTID(p)
		astTestMust(t, err)
		qID, err := m.GetASTID(q)
		astTestMust(t, err)
		trace := output.String()
		for _, want := range []string{
			fmt.Sprintf("-------- [bv_bit_prop] propagate_bits src/smt/theory_bv.cpp:1203 ---------\npropagating #%d[0] = l_true\n", leftID),
			fmt.Sprintf("-------- [bv_bit_prop] propagate_bits src/smt/theory_bv.cpp:1214 ---------\npropagating #%d[0] = l_undef\n", rightID),
			fmt.Sprintf("-------- [bv_bit_prop] assign_bit src/smt/theory_bv.cpp:1242 ---------\nassigning: #%d using #%d #%d #%d idx: 0\npropagate_eqc: 0\n", qID, pID, leftID, rightID),
			"-------- [assign_bit_bug] assign_bit src/smt/theory_bv.cpp:1269 ---------\ncurr->m_var: v",
			"-------- [bv_bit_prop] propagate_bits src/smt/theory_bv.cpp:1232 ---------\ndone propagating\n",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing retained BV propagation trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out BV propagation trace executed", output.String())
	}
	value, err := state.Assignment(qLiteral)
	astTestMust(t, err)
	if value.String() != "l_true" {
		t.Fatal("trace fixture did not propagate", value)
	}
}

func TestSMTBVPropagationAndMergeConflictSourceTrace(t *testing.T) {
	t.Run("propagation", func(t *testing.T) {
		var output bytes.Buffer
		m, state, theory := bvOccurrenceFixture(t)
		tags := diagnostic.NewTraceTagsWithWriter(&output)
		tags.Enable("bv_bit_prop")
		theory.trace = tags
		p := smtRelevancyBool(t, m, "trace-propagate-conflict-p")
		q := smtRelevancyBool(t, m, "trace-propagate-conflict-q")
		_, left := bvNativeWord(t, m, state, theory, p)
		_, right := bvNativeWord(t, m, state, theory, q)
		astTestMust(t, theory.NewEquality(left, right))
		pLiteral, err := state.Literal(p)
		astTestMust(t, err)
		qLiteral, err := state.Literal(q)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(pLiteral, NewSMTBooleanAxiom()))
		astTestMust(t, state.AssignLiteral(qLiteral.Negated(), NewSMTBooleanAxiom()))
		astTestMust(t, theory.Assign(pLiteral.Var(), true))
		if tags.ShouldTrace("bv_bit_prop") && !strings.Contains(output.String(), "-------- [bv_bit_prop] propagate_bits src/smt/theory_bv.cpp:1223 ---------\ninconsistent ") {
			t.Fatal("missing source propagation conflict trace", output.String())
		}
	})

	t.Run("merge", func(t *testing.T) {
		var output bytes.Buffer
		m, state, theory := bvOccurrenceFixture(t)
		tags := diagnostic.NewTraceTagsWithWriter(&output)
		tags.Enable("bv")
		theory.trace = tags
		p := smtRelevancyBool(t, m, "trace-merge-conflict-p")
		q := smtRelevancyBool(t, m, "trace-merge-conflict-q")
		_, left := bvNativeWord(t, m, state, theory, p)
		_, right := bvNativeWord(t, m, state, theory, q)
		pLiteral, err := state.Literal(p)
		astTestMust(t, err)
		qLiteral, err := state.Literal(q)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(pLiteral, NewSMTBooleanAxiom()))
		astTestMust(t, state.AssignLiteral(qLiteral.Negated(), NewSMTBooleanAxiom()))
		astTestMust(t, theory.NewEquality(left, right))
		if tags.ShouldTrace("bv") && !strings.Contains(output.String(), "-------- [bv] merge_eh src/smt/theory_bv.cpp:1424 ---------\ninconsistent ") {
			t.Fatal("missing source merge conflict trace", output.String())
		}
	})
}

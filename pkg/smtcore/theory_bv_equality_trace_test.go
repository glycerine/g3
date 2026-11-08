package smtcore

// Source: src/smt/theory_bv.cpp:227,1106-1109,1164,1181,1384-1397.

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSMTBVEqualityAssignmentAndMergeSourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, state, theory := bvOccurrenceFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	for _, tag := range []string{"bv", "bv_eq", "bv_bit_prop", "bv_diseq_axiom"} {
		tags.Enable(tag)
	}
	theory.trace = tags

	p := smtRelevancyBool(t, m, "trace-equality-p")
	q := smtRelevancyBool(t, m, "trace-equality-q")
	left, leftVariable := bvNativeWord(t, m, state, theory, p)
	right, rightVariable := bvNativeWord(t, m, state, theory, q)
	pVariable, ok := state.BoolVar(p)
	if !ok {
		t.Fatal("missing source Boolean variable")
	}
	astTestMust(t, theory.Assign(pVariable, true))
	astTestMust(t, theory.NewEquality(leftVariable, rightVariable))

	notP, err := m.MkNot(p)
	astTestMust(t, err)
	_, _ = bvNativeWord(t, m, state, theory, notP)

	if tags.ShouldTrace("bv") {
		leftID, err := left.OwnerID()
		astTestMust(t, err)
		rightID, err := right.OwnerID()
		astTestMust(t, err)
		pID, err := m.GetASTID(p)
		astTestMust(t, err)
		trace := output.String()
		for _, want := range []string{
			fmt.Sprintf("-------- [bv] assign_eh src/smt/theory_bv.cpp:1164 ---------\nassert: v%d #%d is_true: 1\n", pVariable, pID), /* ostream bool */
			"-------- [bv] assign_eh src/smt/theory_bv.cpp:1181 ---------\n1\n",
			"-------- [bv_eq] new_eq_eh src/smt/theory_bv.cpp:1106 ---------\nnew_eq: ",
			fmt.Sprintf("-------- [bv] new_eq_eh src/smt/theory_bv.cpp:1107 ---------\nnew_eq_eh v%d = v%d relevant1: 0 relevant2: 0\n", leftVariable, rightVariable),
			fmt.Sprintf("-------- [bv] merge_eh src/smt/theory_bv.cpp:1384 ---------\nmerging: #%d #%d\n", rightID, leftID),
			fmt.Sprintf("-------- [bv_bit_prop] merge_eh src/smt/theory_bv.cpp:1385 ---------\nmerging: #%d #%d\n", rightID, leftID),
			"-------- [bv] merge_eh src/smt/theory_bv.cpp:1397 ---------\nbits size: 1\n",
			"-------- [bv_diseq_axiom] mk_new_diseq_axiom src/smt/theory_bv.cpp:227 ---------\nfound new diseq axiom\n",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing retained BV equality trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out BV equality trace executed", output.String())
	}
}

func TestSMTBVEqualityTraceTagGatesOperands(t *testing.T) {
	_, _, theory := bvOccurrenceFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(&bytes.Buffer{})
	theory.trace = tags
	astTestMust(t, theory.traceBVStaticDisequality(SMTNullTheoryVar, SMTNullTheoryVar))
	astTestMust(t, theory.traceBVNewEquality(SMTNullTheoryVar, SMTNullTheoryVar))
	astTestMust(t, theory.traceBVAssignment(-1, false))
	astTestMust(t, theory.traceBVMerge(SMTNullTheoryVar, SMTNullTheoryVar, "bv"))

	tags.Enable("bv")
	if tags.ShouldTrace("bv") {
		if err := theory.traceBVAssignment(-1, false); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("enabled trace did not evaluate its source operand", err)
		}
	}
}

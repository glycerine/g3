package smtcore

// Source: src/smt/theory_bv.cpp:436-439,557,574,611-615,662-679,
// 859,1052,1087-1088,1285,1325.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSMTBVConversionSourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, solver, theory := bvConversionFixture(t, 0)
	state := solver.kernel.internalizer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("bv")
	tags.Enable("bv2int_bug")
	theory.trace = tags

	zero := bvConversionInt(t, m, "0")
	intSort, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "trace-conversion-x"), intSort)
	astTestMust(t, err)
	converted, err := m.MkIntToBitVector(2, x)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(converted, false))

	bvSort, err := m.BitVectorSort(2)
	astTestMust(t, err)
	w, err := m.MkConst(astTestSymbol(t, m, "trace-conversion-w"), bvSort)
	astTestMust(t, err)
	bvToInt, err := m.MkBitVectorToInt(w)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(bvToInt, false))

	if tags.ShouldTrace("bv") {
		trace := output.String()
		for _, want := range []string{
			"-------- [bv] internalize_term src/smt/theory_bv.cpp:859 ---------\ninternalizing term: ((_ int2bv 2) trace-conversion-x)\n",
			"-------- [bv] assert_int2bv_axiom src/smt/theory_bv.cpp:662 ---------\n",
			"-------- [bv] assert_int2bv_axiom src/smt/theory_bv.cpp:678 ---------\n",
			"-------- [bv] internalize_bv2int src/smt/theory_bv.cpp:557 ---------\n(bv2int trace-conversion-w)\n",
			"-------- [bv2int_bug] assert_bv2int_axiom src/smt/theory_bv.cpp:574 ---------\nbv2int:\n(bv2int trace-conversion-w)\n",
			"-------- [bv] assert_bv2int_axiom src/smt/theory_bv.cpp:611 ---------\n(bv2int trace-conversion-w)\n",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing retained BV conversion trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out BV conversion trace executed", output.String())
	}
}

func TestSMTBVFrontendFixedRelevantAndScopeSourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, state, theory := bvOccurrenceFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("bv")
	theory.trace = tags

	p := smtRelevancyBool(t, m, "trace-control-p")
	q := smtRelevancyBool(t, m, "trace-control-q")
	left, leftVariable := bvNativeWord(t, m, state, theory, p)
	right, rightVariable := bvNativeWord(t, m, state, theory, q)
	astTestMust(t, theory.addFixedEquality(leftVariable, rightVariable))

	comparison, err := m.MkBitVectorULE(left.owner, right.owner)
	astTestMust(t, err)
	_, err = theory.InternalizeAtom(comparison, true)
	astTestMust(t, err)

	relevancy, err := NewSMTRelevancyPropagator(state.context, true)
	astTestMust(t, err)
	state.relevancy = relevancy
	astTestMust(t, theory.Relevant(left.owner))

	astTestMust(t, state.PushScope())
	astTestMust(t, state.PopScopes(1))

	wideSort, err := m.BitVectorSort(3)
	astTestMust(t, err)
	wide, err := m.MkConst(astTestSymbol(t, m, "trace-control-wide"), wideSort)
	astTestMust(t, err)
	theory.config.BlastMaxSize = 2
	_, err = theory.approximateTerm(wide)
	astTestMust(t, err)

	if tags.ShouldTrace("bv") {
		trace := output.String()
		for _, want := range []string{
			"-------- [bv] add_fixed_eq src/smt/theory_bv.cpp:436 ---------\n(mkbv trace-control-p) = (mkbv trace-control-q) 0\n",
			"-------- [bv] internalize_atom src/smt/theory_bv.cpp:1052 ---------\ninternalizing atom: (bvule (mkbv trace-control-p) (mkbv trace-control-q))\n",
			"-------- [bv] relevant_eh src/smt/theory_bv.cpp:1285 ---------\nrelevant: (mkbv trace-control-p)\n",
			"-------- [bv] pop_scope_eh src/smt/theory_bv.cpp:1325 ---------\n1\n",
			"-------- [bv] approximate_term src/smt/theory_bv.cpp:1087 ---------\nfound large size bit-vector:\ntrace-control-wide\n",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing retained BV frontend trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out BV frontend trace executed", output.String())
	}
}

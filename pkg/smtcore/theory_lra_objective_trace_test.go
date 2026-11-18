package smtcore

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func lraObjectiveTraceRecord(tag, function string, line int, body string) string {
	return fmt.Sprintf("-------- [%s] %s src/smt/theory_lra.cpp:%d ---------\n%s------------------------------------------------\n", tag, function, line, body)
}

func TestSMTLRAObjectiveTraceBlockerPairFormatAndLiveReturn(t *testing.T) {
	for _, tc := range []struct {
		integer             bool
		r                   string
		eps                 int64
		rounded, expression string
	}{
		{false, "2", -1, "2", "(>= trace-objective-x 2.0)"},
		{false, "2", 0, "2", "(> trace-objective-x 2.0)"},
		{true, "2", -1, "3", "(>= trace-objective-x 3)"},
		{true, "3/2", 0, "2", "(>= trace-objective-x 2)"},
		{true, "-3/2", 1, "-1", "(>= trace-objective-x (- 1))"},
	} {
		for _, refuse := range []bool{false, true} {
			f := newLRAFrontendFixture(t, true)
			if tc.integer {
				var err error
				f.sort, err = f.m.MkFamilySort(f.a.FamilyID(), 1, nil)
				astTestMust(t, err)
			}
			x := f.variable("trace-objective-x")
			v, err := f.a.AddObjective(x)
			astTestMust(t, err)
			writer := &lraLifecycleTraceWriter{owner: f.a, refuse: refuse}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("opt")
			f.a.trace = tags
			r, ok := new(big.Rat).SetString(tc.r)
			if !ok {
				t.Fatal(tc.r)
			}
			value := testNI(r, big.NewRat(tc.eps, 1))
			// Pin only after the callback returns, exercising the actual API's
			// unrooted return rather than an extra test-only reference.
			h := f.pin(f.a.objectiveBlocker(v, &value))
			rounded, ok := new(big.Rat).SetString(tc.rounded)
			if !ok {
				t.Fatal(tc.rounded)
			}
			op := smtArithOpGT
			if tc.integer || tc.eps < 0 {
				op = smtArithOpGE
			}
			lraObjectiveBlockerShape(t, f, h, v, op, rounded)
			if tags.ShouldTrace("opt") && !refuse {
				want := lraObjectiveTraceRecord("opt", "mk_gt", 2392, fmt.Sprintf("v%d (%s, %d) %s %s\n", v, tc.r, tc.eps, tc.rounded, tc.expression))
				if writer.output.String() != want {
					t.Fatal(writer.output.String(), want)
				}
			} else if writer.output.Len() != 0 {
				t.Fatal(writer.output.String())
			}
		}
	}
}

func TestSMTLRAObjectiveTraceMappedAndUnmappedInfinity(t *testing.T) {
	for _, mapped := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x := f.variable("trace-ray-x")
		if mapped {
			f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
			f.propagate(true)
		}
		v, err := f.a.AddObjective(x)
		astTestMust(t, err)
		var out bytes.Buffer
		tags := diagnostic.NewTraceTagsWithWriter(&out)
		tags.Enable("arith")
		f.a.trace = tags
		value, h, shared, err := f.a.MaximizeObjective(v)
		astTestMust(t, err)
		if !value.Unbounded || shared || !f.m.IsFalse(h) {
			t.Fatal(value, h, shared)
		}
		want := ""
		if mapped && tags.ShouldTrace("arith") {
			want = lraObjectiveTraceRecord("arith", "maximize", 2361, fmt.Sprintf("Unbounded %d\n", v))
		}
		if out.String() != want {
			t.Fatal(mapped, out.String(), want)
		}
	}
}

func TestSMTLRAObjectiveTraceLowerBoundRegistrationReuseAndPolarity(t *testing.T) {
	for _, strict := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x := f.variable("trace-lower-x")
		f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
		f.propagate(true)
		v, err := f.a.AddObjective(x)
		astTestMust(t, err)
		var out bytes.Buffer
		tags := diagnostic.NewTraceTagsWithWriter(&out)
		tags.Enable("arith")
		f.a.trace = tags
		eps := int64(0)
		if strict {
			eps = 1
		}
		point := testNI(big.NewRat(5, 1), big.NewRat(eps, 1))
		filter := objectiveBoundFilter(t, f.m)
		base := "(>= trace-lower-x 5.0)"
		final := base
		if strict {
			base = "(<= trace-lower-x 5.0)"
			final = "(not " + base + ")"
		}
		for attempt := 0; attempt < 2; attempt++ {
			out.Reset()
			before := f.a.unassigned[v]
			h := f.pin(f.a.MakeObjectiveLowerBound(filter, v, point))
			var printed bytes.Buffer
			astTestMust(t, f.a.displayLRAExpression(&printed, h))
			if printed.String() != final {
				t.Fatal(printed.String(), final)
			}
			want := ""
			if tags.ShouldTrace("arith") {
				if attempt == 0 {
					want += lraObjectiveTraceRecord("arith", "updt_unassigned_bounds", 595, fmt.Sprintf("v%d %d += 1\n", v, before))
					want += lraObjectiveTraceRecord("arith", "mk_ge", 2444, base+"\n")
				}
				want += lraObjectiveTraceRecord("arith", "mk_ge", 2449, final+"\n")
			}
			if out.String() != want {
				t.Fatal(strict, attempt, out.String(), want)
			}
			if strings.Count(out.String(), "src/smt/theory_lra.cpp:2444") > 1 {
				t.Fatal(out.String())
			}
		}
	}
}

func TestSMTLRAObjectiveTraceActualStrictMaximizeAndBadbit(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x, y := f.variable("trace-max-x"), f.variable("trace-max-y")
		f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
		f.assert(f.app(smtArithOpGE, x, f.number(3)), false)
		f.assert(f.app(smtArithOpGE, y, f.number(0)), true)
		f.assert(f.app(smtArithOpLE, y, f.number(2)), true)
		objective := f.app(smtArithOpAdd, f.number(3), f.app(smtArithOpMul, f.number(2), x), f.app(smtArithOpUMinus, y))
		v, err := f.a.AddObjective(objective)
		astTestMust(t, err)
		f.propagate(true)
		writer := &lraLifecycleTraceWriter{owner: f.a, refuse: refuse}
		tags := diagnostic.NewTraceTagsWithWriter(writer)
		tags.Enable("opt")
		f.a.trace = tags
		value, h, shared, err := f.a.MaximizeObjective(v)
		astTestMust(t, err)
		h = f.pin(h, nil)
		if value.Unbounded || shared {
			t.Fatal(value, shared)
		}
		checkInfiniteArithmetic(t, value.Finite, 0, 9, -2)
		lraObjectiveBlockerShape(t, f, h, v, smtArithOpGE, big.NewRat(9, 1))
		if tags.ShouldTrace("opt") && !refuse {
			prefix := fmt.Sprintf("v%d (9, -2) 9 ", v)
			if !strings.Contains(writer.output.String(), "mk_gt src/smt/theory_lra.cpp:2392") || !strings.Contains(writer.output.String(), prefix) {
				t.Fatal(writer.output.String())
			}
		} else if writer.output.Len() != 0 {
			t.Fatal(writer.output.String())
		}
		if f.a.numeric.stopped {
			t.Fatal("trace badbit stopped maximizer")
		}
	}
	checkNativeLRANumeric(t)
}

func TestSMTLRAObjectiveTraceLowerBoundBadbit(t *testing.T) {
	for _, strict := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x := f.variable("trace-badbit-lower-x")
		f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
		f.propagate(true)
		v, err := f.a.AddObjective(x)
		astTestMust(t, err)
		writer := &lraLifecycleTraceWriter{owner: f.a, refuse: true}
		tags := diagnostic.NewTraceTagsWithWriter(writer)
		tags.Enable("arith")
		f.a.trace = tags
		eps := int64(0)
		if strict {
			eps = 1
		}
		h := f.pin(f.a.MakeObjectiveLowerBound(objectiveBoundFilter(t, f.m), v, testNI(big.NewRat(5, 1), big.NewRat(eps, 1))))
		negated, base, err := f.m.IsNot(h)
		astTestMust(t, err)
		if negated != strict {
			t.Fatal("badbit changed strictness", strict, negated)
		}
		if !strict {
			base = h
		}
		bv, found := f.s.BoolVar(base)
		if !found || f.a.boundByBoolean[bv] == nil || f.a.boundByBoolean[bv].variable != v || f.a.numeric.stopped {
			t.Fatal("badbit damaged registration", found, bv)
		}
	}
}

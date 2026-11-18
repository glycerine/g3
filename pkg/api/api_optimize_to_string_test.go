package api

import "testing"

// AUDIT.md cmd_context_02-F9: Z3_optimize_to_string is opt::context::
// to_string: declarations, asserts, objectives (weights by
// display_decimal(3, true), quoted ids), opt parameters, (check-sat). The
// expected text is z3 4.7.1's C API output for the same calls.
func TestAPIOptimizeToStringMatchesSource(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	o, err := f.c.NewOptimize()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, o.Close()) })
	boolean := f.pin(f.c.BoolSort())
	integer := f.pin(f.c.IntegerSort())
	p := f.pin(f.c.Constant("api-object-opt-p", boolean))
	x := f.pin(f.c.Constant("x", integer))
	zero := f.pin(f.c.Numeral("0", integer))
	astTestMust(t, o.Assert(f.pin(f.m.MkArithmeticGE(x, zero))))
	_, err = o.Minimize(x)
	astTestMust(t, err)
	_, err = o.AssertSoft(p, softRat(2, 1), "preferences")
	astTestMust(t, err)
	_, err = o.AssertSoft(f.pin(f.c.Not(p)), softRat(1, 3), "preferences")
	astTestMust(t, err)
	_, err = o.AssertSoft(p, softRat(1, 1), "a b")
	astTestMust(t, err)
	_, err = o.Maximize(f.pin(f.m.MkArithmeticAdd(x, x)))
	astTestMust(t, err)
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	astTestMust(t, parameters.SetSymbol("priority", "box"))
	astTestMust(t, o.SetParameters(parameters))
	text, err := o.SMT2()
	astTestMust(t, err)
	want := "(declare-fun x () Int)\n(declare-fun api-object-opt-p () Bool)\n(assert (>= x 0))\n(minimize x)\n" +
		"(assert-soft api-object-opt-p :weight 2 :id preferences)\n" +
		"(assert-soft (not api-object-opt-p) :weight 0.333 :id preferences)\n" +
		"(assert-soft api-object-opt-p :weight 1 :id |a b|)\n" +
		"(maximize (+ x x))\n(set-option :opt.priority box)\n(check-sat)\n"
	if text != want {
		t.Fatalf("to_string:\n%s\nwant:\n%s", text, want)
	}
}

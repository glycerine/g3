package smtcore

// Source: src/smt/theory_arith_eq.h retained diagnostics and their live
// fixed-variable, offset-row, and equal-definition propagation paths.

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func enableArithmeticEqualityTrace(tags *diagnostic.TraceTags) {
	for _, tag := range []string{"arith", "arith_bug", "arith_eq", "arith_eq_propagation", "arith_eq_verbose"} {
		tags.Enable(tag)
	}
}

func arithmeticEqualityTraceFixture(t *testing.T) (*arithmeticObjectiveFixture, *bytes.Buffer) {
	t.Helper()
	f := newArithmeticObjectiveFixture(t, false)
	output := &bytes.Buffer{}
	tags := diagnostic.NewTraceTagsWithWriter(output)
	enableArithmeticEqualityTrace(tags)
	f.a.equalityAdapter.trace = tags
	return f, output
}

func requireArithmeticEqualityTrace(t *testing.T, output string, sites ...string) {
	t.Helper()
	gate := diagnostic.NewTraceTagsWithWriter(io.Discard)
	gate.Enable("arith_eq")
	if !gate.ShouldTrace("arith_eq") {
		if output != "" {
			t.Fatalf("erased arithmetic equality TRACE emitted output:\n%s", output)
		}
		return
	}
	for _, site := range sites {
		if !strings.Contains(output, site) {
			t.Fatalf("missing %q in arithmetic equality trace:\n%s", site, output)
		}
	}
}

func TestSMTArithmeticEqualityRetainedDiagnostics(t *testing.T) {
	t.Run("zero-offset-row", func(t *testing.T) {
		f, output := arithmeticEqualityTraceFixture(t)
		x, y := f.variable("eq-trace-x"), f.variable("eq-trace-y")
		f.assert(f.app(smtArithOpLE, x, y))
		f.assert(f.app(smtArithOpGE, x, y))
		propagated, err := f.a.propagateImpliedEqualities()
		astTestMust(t, err)
		if !propagated {
			t.Fatal("zero-offset row did not propagate")
		}
		requireArithmeticEqualityTrace(t, output.String(),
			"[arith_eq_verbose] propagate_cheap_eq src/smt/theory_arith_eq.h:220",
			"[arith_eq] propagate_cheap_eq src/smt/theory_arith_eq.h:263",
			"[arith_eq] propagate_eq_to_core src/smt/theory_arith_eq.h:351",
			"[arith_eq_propagation] propagate_eq_to_core src/smt/theory_arith_eq.h:354")
	})

	t.Run("fixed-variables", func(t *testing.T) {
		f, output := arithmeticEqualityTraceFixture(t)
		x, y, one := f.variable("fixed-trace-x"), f.variable("fixed-trace-y"), f.number("1")
		for _, variable := range []ast.ASTHandle{x, y} {
			f.assert(f.app(smtArithOpLE, variable, one))
			f.assert(f.app(smtArithOpGE, variable, one))
		}
		propagated, err := f.a.propagateImpliedEqualities()
		astTestMust(t, err)
		if !propagated {
			t.Fatal("fixed variables did not propagate")
		}
		requireArithmeticEqualityTrace(t, output.String(),
			"[arith_eq_propagation] propagate_cheap_eq src/smt/theory_arith_eq.h:251",
			"fixed\n")
	})

	t.Run("fixed-monomial-implies-fixed-variable", func(t *testing.T) {
		f, output := arithmeticEqualityTraceFixture(t)
		x := f.variable("implied-fixed-trace-x")
		z := f.variable("implied-fixed-trace-z")
		w := f.variable("implied-fixed-trace-w")
		one, minusOne, zero := f.number("1"), f.number("-1"), f.number("0")
		for _, bound := range [][3]ast.ASTHandle{{z, one, one}, {w, minusOne, minusOne}} {
			f.assert(f.app(smtArithOpLE, bound[0], bound[1]))
			f.assert(f.app(smtArithOpGE, bound[0], bound[2]))
		}
		xPlusZ := f.app(smtArithOpAdd, x, z)
		f.assert(f.app(smtArithOpLE, xPlusZ, zero))
		f.assert(f.app(smtArithOpGE, xPlusZ, zero))
		propagated, err := f.a.propagateImpliedEqualities()
		astTestMust(t, err)
		if !propagated {
			t.Fatal("fixed monomial row did not propagate its implied fixed variable")
		}
		_, err = f.s.PropagateCore()
		astTestMust(t, err)
		if f.s.context.enodes.get(x).Root() != f.s.context.enodes.get(w).Root() {
			t.Fatal("implied fixed variable did not merge with the fixed-value peer")
		}
		requireArithmeticEqualityTrace(t, output.String(),
			"[arith_eq_propagation] propagate_cheap_eq src/smt/theory_arith_eq.h:251",
			"[arith_eq] propagate_eq_to_core src/smt/theory_arith_eq.h:351")
	})

	t.Run("equal-definition-rows", func(t *testing.T) {
		f, output := arithmeticEqualityTraceFixture(t)
		x, y := f.variable("definition-trace-x"), f.variable("definition-trace-y")
		first := f.app(smtArithOpAdd, x, y)
		second := f.app(smtArithOpAdd, y, x)
		astTestMust(t, f.s.Internalize(first, false))
		astTestMust(t, f.s.Internalize(second, false))
		propagated, err := f.a.propagateImpliedEqualities()
		astTestMust(t, err)
		if !propagated {
			t.Fatal("equal definition rows did not propagate")
		}
		requireArithmeticEqualityTrace(t, output.String(),
			"[arith_eq] propagate_cheap_eq src/smt/theory_arith_eq.h:304",
			"propagate eq two rows:\n")
	})

	t.Run("fixed-var-event", func(t *testing.T) {
		theory, solver, first, second := smtFixedEqualityGuardFixture(t)
		var output bytes.Buffer
		tags := diagnostic.NewTraceTagsWithWriter(&output)
		enableArithmeticEqualityTrace(tags)
		theory.equalityAdapter.trace = tags
		propagated, err := theory.propagateFixedEquality(solver, first, second)
		astTestMust(t, err)
		if !propagated {
			t.Fatal("fixed-var event did not propagate")
		}
		requireArithmeticEqualityTrace(t, output.String(),
			"[arith_eq] fixed_var_eh src/smt/theory_arith_eq.h:47",
			"[arith_eq] fixed_var_eh src/smt/theory_arith_eq.h:68")
	})
}

func TestSMTArithmeticEqualityDiagnosticBadbitAndAssertionGate(t *testing.T) {
	theory, solver, first, second := smtFixedEqualityGuardFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(&arithmeticAdapterTraceWriter{refuse: true})
	enableArithmeticEqualityTrace(tags)
	theory.equalityAdapter.trace = tags
	propagated, err := theory.propagateFixedEquality(solver, first, second)
	astTestMust(t, err)
	if !propagated {
		t.Fatal("trace badbit stopped fixed equality propagation")
	}

	d, _ := newLRASourceAssertionContext(t)
	for _, enabled := range []bool{false, true} {
		astTestMust(t, d.EnableAssertions(enabled))
		for _, assertion := range []struct {
			line      int32
			condition string
		}{{40, "is_fixed(v)"}, {44, "lower_bound(v).is_rational()"}, {170, "is_fixed(v)"}, {193, "x != null_theory_var"}, {300, "y == y2 && k == k2"}} {
			err = arithmeticEqualityAssertion(assertion.line, assertion.condition, false)
			checkLRASourceAssertionResult(t, err, enabled, nil)
		}
	}
}

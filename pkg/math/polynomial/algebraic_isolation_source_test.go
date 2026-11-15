package polynomial

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestAlgebraicIsolationSourceTraceBranches(t *testing.T) {
	for _, factor := range []bool{false, true} {
		c, up, _ := algebraicControlledFixture(t)
		c.factor = factor
		p := algebraicPolynomial(t, up, -1, 0, 1)
		roots, e := c.NewVector()
		serviceMust(t, e)
		var output strings.Builder
		tags := diagnostic.NewTraceTagsWithWriter(&output)
		tags.Enable("algebraic")
		serviceMust(t, c.config.Rationals.BindTrace(tags))
		serviceMust(t, c.IsolateRoots(mustUPolyView(t, up, p), roots))
		serviceMust(t, c.config.Rationals.BindTrace(nil))
		if len(roots.values) != 2 {
			t.Fatal("root count", roots.values)
		}
		algebraicExpectBetween(t, c, &roots.values[0], -2, 0)
		algebraicExpectBetween(t, c, &roots.values[1], 0, 2)
		if tags.ShouldTrace("algebraic") {
			entry := "-------- [algebraic] isolate_roots src/math/polynomial/algebraic_numbers.cpp:568 ---------\nx^2 - 1\n------------------------------------------------\n"
			if !strings.HasPrefix(output.String(), entry) || strings.Contains(output.String(), ":655 ---------") {
				t.Fatal("isolation entry/selection frontier", output.String())
			}
			if factor {
				if strings.Count(output.String(), "linear polynomial...\n") != 2 || !strings.Contains(output.String(), "factor 0 degree: 1\n") || !strings.Contains(output.String(), "factor 1 degree: 1\n") {
					t.Fatal("linear factor source records", output.String())
				}
			} else {
				for _, record := range []string{
					"-------- [algebraic] factor src/math/polynomial/algebraic_numbers.cpp:549 ---------\nx^2 - 1\n------------------------------------------------\n",
					"factor 0 degree: 2\n", "isolated roots: 0\n",
				} {
					if !strings.Contains(output.String(), record) {
						t.Fatal("square-free isolation source record", record, output.String())
					}
				}
			}
		} else if output.Len() != 0 {
			t.Fatal("erased isolation TRACE emitted output")
		}
		serviceMust(t, roots.Close())
	}
}

func TestAlgebraicIsolationEmptyTraceBeforeLazyMembers(t *testing.T) {
	c, _, _ := algebraicControlledFixture(t)
	roots, e := c.NewVector()
	serviceMust(t, e)
	factors, scratch := c.factors, c.roots
	var output strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("algebraic")
	serviceMust(t, c.config.Rationals.BindTrace(tags))
	serviceMust(t, c.IsolateRoots(UPolynomialView{}, roots))
	if c.factors != factors || c.roots != scratch || len(roots.values) != 0 || c.univariateIsolationRunning || c.univariateIsolationOutput != nil {
		t.Fatal("empty input acquired isolation members or retained its lifetime span")
	}
	want := ""
	if tags.ShouldTrace("algebraic") {
		want = "-------- [algebraic] isolate_roots src/math/polynomial/algebraic_numbers.cpp:568 ---------\n0\n------------------------------------------------\n"
	}
	if output.String() != want {
		t.Fatal("empty input source trace", output.String())
	}
	serviceMust(t, c.config.Rationals.BindTrace(nil))
	serviceMust(t, roots.Close())
}

func TestAlgebraicIsolationPolynomialZeroTraceBeforeConversion(t *testing.T) {
	c, _, _ := algebraicControlledFixture(t)
	pm, _, _ := polynomialEvalFixtureWithRationals(t, c.config.Rationals, []polynomialEvalTerm{{1, [3]uint32{}}})
	roots, e := c.NewVector()
	serviceMust(t, e)
	var output strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("algebraic")
	serviceMust(t, c.config.Rationals.BindTrace(tags))
	serviceMust(t, c.IsolatePolynomialRoots(pm, pm.core.zero, roots))
	want := ""
	if tags.ShouldTrace("algebraic") {
		want = "-------- [algebraic] isolate_roots src/math/polynomial/algebraic_numbers.cpp:637 ---------\nisolating roots of: 0\n------------------------------------------------\n"
	}
	if output.String() != want || c.isolateTmp1.slot != 0 || c.isolationConversion.active {
		t.Fatal("zero polynomial trace/conversion frontier", output.String())
	}
	serviceMust(t, c.config.Rationals.BindTrace(nil))
	serviceMust(t, roots.Close())
}

func TestAlgebraicSquareFreeFactorResetBeforeInputRead(t *testing.T) {
	c, up, _ := algebraicControlledFixture(t)
	c.factor = false
	serviceMust(t, c.ensureIsolation())
	var e error
	c.tmp3, e = up.NewPolynomial()
	serviceMust(t, e)
	serviceMust(t, up.SetInts(c.tmp3, []unumeral.Int{unumeral.NewInt(9), unumeral.NewInt(1)}))
	// A Go owner-read refusal exposes the source reset frontier, without
	// assigning mathematical meaning to an invalid source view.
	_, body := c.factorPolynomialInto(UPolynomialView{count: 2}, c.factors)
	if body == nil {
		t.Fatal("invalid input unexpectedly completed square-free factoring")
	}
	n, e := up.Size(c.tmp3)
	serviceMust(t, e)
	if n != 0 {
		t.Fatal("source square-free scratch reset occurred after the input read", n)
	}
}

func TestAlgebraicIsolationDisabledScratchAssertionsDoNotRead(t *testing.T) {
	c, _, _ := algebraicControlledFixture(t)
	if polynomialSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		old, e := d.AssertionsEnabled()
		serviceMust(t, e)
		serviceMust(t, d.EnableAssertions(false))
		defer func() { serviceMust(t, d.EnableAssertions(old)) }()
	}
	if c.roots != nil || c.lowers != nil || c.uppers != nil {
		t.Fatal("expected cold selected-core vector members")
	}
	serviceMust(t, c.assertIsolationScratchEmpty())
	serviceMust(t, c.assertIsolationBoundsSizes())
}

func TestAlgebraicIsolationScratchAssertionActions(t *testing.T) {
	if !polynomialSourceDebug {
		c, _, _ := algebraicControlledFixture(t)
		serviceMust(t, c.assertIsolationScratchEmpty())
		serviceMust(t, c.assertIsolationBoundsSizes())
		return
	}
	for _, action := range []string{"T", "C"} {
		t.Run(action, func(t *testing.T) {
			c, _, _ := algebraicControlledFixture(t)
			serviceMust(t, c.ensureIsolation())
			one := unumeral.NewBinaryInt(1)
			serviceMust(t, c.roots.Resize(1))
			serviceMust(t, c.roots.Set(0, &one))
			serviceMust(t, c.lowers.Resize(1))
			serviceMust(t, c.lowers.Set(0, &one))
			d := diagnostic.BorrowProcessDebugContext()
			old, e := d.AssertionsEnabled()
			serviceMust(t, e)
			serviceMust(t, d.EnableAssertions(true))
			defer func() { serviceMust(t, d.EnableAssertions(old)) }()
			polynomialAssertionActionsFixture(t, action+action)
			for i, body := range []error{c.assertIsolationScratchEmpty(), c.assertIsolationBoundsSizes()} {
				if action == "T" {
					var exception z3exception.DefaultException
					if !errors.As(body, &exception) {
						t.Fatal("source scratch assertion did not use Throw", i, body)
					}
				} else {
					serviceMust(t, body)
				}
			}
			if c.roots.state.size != 1 || c.lowers.state.size != 1 || c.uppers.state.size != 0 {
				t.Fatal("assertion action repaired source scratch")
			}
		})
	}
}

func TestAlgebraicIsolationCooperationKeepsExecutingOwners(t *testing.T) {
	c, up, _ := algebraicControlledFixture(t)
	c.factor = false
	p := algebraicPolynomial(t, up, -1, 0, 1)
	view := mustUPolyView(t, up, p)
	out, e := c.NewVector()
	serviceMust(t, e)
	cooperate := up.config.Cooperation
	probes := 0
	up.config.Cooperation = func() error {
		probes++
		count, callers := len(out.values), c.callers
		var zero Algebraic
		for i, action := range []func() error{c.Close, c.CleanupRetained, c.CleanupIsolation, out.Close, out.Reset, func() error { return out.PushCopy(&zero) }, func() error { return c.IsolateRoots(view, out) }} {
			if e := action(); !errors.Is(e, ErrAlgebraicPending) {
				t.Fatalf("isolation cooperation action %d: %v", i, e)
			}
		}
		if c.closed || c.closing || out.closed || out.closing || len(out.values) != count || c.callers != callers {
			t.Fatal("cooperation changed executing isolation owners")
		}
		return cooperate()
	}
	serviceMust(t, c.IsolateRoots(view, out))
	up.config.Cooperation = cooperate
	if probes == 0 || len(out.values) != 2 || c.univariateIsolationRunning || c.univariateIsolationOutput != nil {
		t.Fatal("isolation did not reach cooperation or end its lifetime span", probes)
	}
	serviceMust(t, out.Close())
}

// TestAlgebraicIsolationSortComparisonOrder: sort_roots is std::sort with
// lt_proc (algebraic_numbers.cpp:564), so the compare records (argument
// order included) and the refinements they make follow libstdc++'s
// insertion sort (AUDIT.md math_polynomial_01-F1). Expected lines come
// from build_trace/z3 -tr:algebraic on (check-sat-using qfnra-nlsat) with
// (> (- (* x x) 2) 0) and (> (+ (* x x x x) (* (- 10) x x) 1) 0); the
// first isolate_roots of each run sorts exactly these roots.
func TestAlgebraicIsolationSortComparisonOrder(t *testing.T) {
	for _, tc := range []struct {
		coeff []int32
		want  []string
	}{
		{[]int32{-2, 0, 1}, []string{"comparing: (-4, 0) (0, 4)"}},
		{[]int32{1, 0, -10, 0, 1}, []string{
			"comparing: (0, 2) (2, 4)",
			"comparing: (-4, -2) (0, 2)",
			"comparing: (-2, 0) (-4, -2)",
			"comparing: (-2, 0) (2, 4)",
			"comparing: (-2, 0) (0, 2)",
			"comparing: (-2, 0) (-4, -2)",
		}},
	} {
		c, up, _ := algebraicControlledFixture(t)
		p := algebraicPolynomial(t, up, tc.coeff...)
		roots, e := c.NewVector()
		serviceMust(t, e)
		var output strings.Builder
		tags := diagnostic.NewTraceTagsWithWriter(&output)
		tags.Enable("algebraic")
		serviceMust(t, c.config.Rationals.BindTrace(tags))
		serviceMust(t, c.IsolateRoots(mustUPolyView(t, up, p), roots))
		serviceMust(t, c.config.Rationals.BindTrace(nil))
		if len(roots.values) != len(tc.coeff)-1 {
			t.Fatal("root count", len(roots.values))
		}
		if tags.ShouldTrace("algebraic") {
			if !strings.Contains(output.String(), "-------- [algebraic] compare src/math/polynomial/algebraic_numbers.cpp:1861 ---------\n"+tc.want[0]+"\n") {
				t.Fatal("first compare record", output.String())
			}
			var got []string
			for _, line := range strings.Split(output.String(), "\n") {
				if strings.HasPrefix(line, "comparing: ") {
					got = append(got, line)
				}
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("compare sequence\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		}
		serviceMust(t, roots.Close())
	}
}

package arith

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestAddBoundsConstructorAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 512; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			base, m := fixDLFixture(t, false)
			m.Budget().FailAfter(stop)
			r, err := NewAddBoundsTactic(m, base.context, base.globals, nil)
			m.Budget().FailAfter(-1)
			if r != nil {
				equationMust(t, r.Close())
			}
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				equationMust(t, failure.Cleanup())
			}
			var memout memory.Failure
			if err == nil {
				success = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &memout) && memout.Kind == memory.FailureMemout) {
				t.Fatal(err)
			}
		})
		if success {
			break
		}
	}
	if !success {
		t.Fatal("constructor did not reach a successful allocation frontier")
	}
}

type addBoundsClosedWriter struct{}

func (addBoundsClosedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestAddBoundsPrecisionTraceAndCleanupRefusal(t *testing.T) {
	base, m := fixDLFixture(t, false)
	r, err := NewAddBoundsTactic(m, base.context, base.globals, nil)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	x := fixDLConst(t, m, true, "x")
	bound := fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "10"))
	goal, err := tactic.NewGoal(m, false, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.Assert(bound))
	equationMust(t, goal.SetPrecision(tactic.GoalOver))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("add_bounds")
	equationMust(t, r.BindTrace(tags))
	equationMust(t, r.Apply(goal, &result))
	if goal.SatPreserved() || goal.UnsatPreserved() {
		t.Fatal("OVER union UNDER did not become UNDER_OVER")
	}
	count, err := goal.Size()
	equationMust(t, err)
	if count != 3 {
		t.Fatal("pre-existing lower bound unexpectedly suppressed insertion", count)
	}
	if tags.ShouldTrace("add_bounds") {
		if !strings.Contains(output.String(), "src/tactic/arith/add_bounds_tactic.cpp:132") || !strings.Contains(output.String(), "(<= x 2)") {
			t.Fatal("missing source goal trace", output.String())
		}
	} else if output.Len() != 0 {
		t.Fatal("erased trace emitted")
	}
	old := r.current
	m.Budget().FailAfter(0)
	err = r.Cleanup()
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || r.current != old {
		t.Fatal("failed cleanup replaced the current bounds", err)
	}
	equationMust(t, r.Cleanup())
	closed := diagnostic.NewTraceTagsWithWriter(addBoundsClosedWriter{})
	closed.Enable("add_bounds")
	equationMust(t, r.BindTrace(closed))
	equationMust(t, r.Apply(goal, &result))
}

func TestAddBoundsTranslationAndRationalParameterSnapshot(t *testing.T) {
	base, m := fixDLFixture(t, false)
	c := base.context
	settings, err := c.NewValue()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, c.Del(&settings)) })
	set := func(name, text string) {
		n, e := c.Numerals().NewValue()
		equationMust(t, e)
		defer func() { equationMust(t, c.Numerals().Del(&n)) }()
		equationMust(t, c.Numerals().SetString(&n, text))
		equationMust(t, c.SetRatName(&settings, &name, &n))
	}
	set("add_bound_lower", "-123456789012345678901234567890")
	set("add_bound_upper", "123456789012345678901234567890")
	r, err := NewAddBoundsTactic(m, c, base.globals, &settings)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, target.Close()) })
	equationMust(t, target.RegisterDeclarationPlugins())
	translated, err := r.Translate(target)
	equationMust(t, err)
	equationMust(t, translated.IncRef())
	t.Cleanup(func() { equationMust(t, translated.DecRef()) })
	copy := translated.(*AddBoundsTactic)
	set("add_bound_lower", "-8")
	set("add_bound_upper", "7")
	equationMust(t, r.UpdateParams(&settings))
	equationMust(t, r.Close())
	if ratText(m, &copy.current.lower) != "-123456789012345678901234567890" || ratText(m, &copy.current.upper) != "123456789012345678901234567890" {
		t.Fatal("source updates changed translated policy")
	}
	equationMust(t, translated.Cleanup())
	if ratText(m, &copy.current.lower) != "-123456789012345678901234567890" {
		t.Fatal("cleanup lost copied parameter snapshot")
	}
	equationMust(t, translated.UpdateParams(&settings))
	equationMust(t, translated.Cleanup())
	if ratText(m, &copy.current.lower) != "-8" || ratText(m, &copy.current.upper) != "7" {
		t.Fatal("updated policy lost on rebuild")
	}
}

func TestArithmeticFragmentProbeNativeContracts(t *testing.T) {
	base, m := fixDLFixture(t, false)
	c, g := base.context, base.globals
	x := fixDLConst(t, m, true, "x")
	two := fixDLNumber(t, m, true, "2")
	zero := fixDLNumber(t, m, true, "0")
	for _, tc := range []struct {
		name string
		term ast.ASTHandle
		kind ArithmeticFragment
		want bool
	}{
		{"linear-left", fixDLApp(t, m, 2, fixDLApp(t, m, 9, two, x), zero), FragmentILP, true},
		{"linear-right-refused", fixDLApp(t, m, 2, fixDLApp(t, m, 9, x, two), zero), FragmentILP, false},
		{"broad-linear-right-refused", fixDLApp(t, m, 2, fixDLApp(t, m, 9, x, two), zero), FragmentLIA, false},
		{"nonlinear-accepts-right", fixDLApp(t, m, 2, fixDLApp(t, m, 9, x, two), zero), FragmentQFNIA, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goal, err := tactic.NewGoal(m, false, true, false)
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, goal.Close()) })
			equationMust(t, goal.Assert(tc.term))
			p, err := NewArithmeticFragmentProbe(m.Budget(), c, g, tc.kind)
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, p.Close()) })
			value, err := p.Eval(goal)
			equationMust(t, err)
			if (value != 0) != tc.want {
				t.Fatal(value, tc.want)
			}
			var text bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&text)
			tags.Enable("probe")
			equationMust(t, p.BindTrace(tags))
			_, err = p.Eval(goal)
			equationMust(t, err)
			if !tc.want && tc.kind == FragmentLIA && tags.ShouldTrace("probe") && !strings.Contains(text.String(), "probe_arith.cpp:396") {
				t.Fatal("missing rejection diagnostic", text.String())
			}
			closed := diagnostic.NewTraceTagsWithWriter(addBoundsClosedWriter{})
			closed.Enable("probe")
			equationMust(t, p.BindTrace(closed))
			_, err = p.Eval(goal)
			equationMust(t, err)
		})
	}
	// Probe classes are born uncounted and participate in the existing typed
	// composition ownership contract.
	p, err := NewArithmeticFragmentProbe(m.Budget(), c, g, FragmentILP)
	equationMust(t, err)
	neg, err := tactic.NewNotProbe(m.Budget(), p)
	equationMust(t, err)
	equationMust(t, neg.Close())
	if p.Check() == nil {
		t.Fatal("composition did not release its probe child")
	}
	_, err = NewArithmeticFragmentProbe(m.Budget(), c, g, ArithmeticFragment(255))
	if !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal(err)
	}
}

func TestArithmeticFragmentProbeAcrossManagerDomains(t *testing.T) {
	first, m1 := fixDLFixture(t, false)
	p, err := NewArithmeticFragmentProbe(m1.Budget(), first.context, first.globals, FragmentLIA)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, p.Close()) })
	_, m2 := fixDLFixture(t, false)
	x := fixDLConst(t, m2, false, "other-domain-x")
	zero := fixDLNumber(t, m2, false, "0")
	goal, err := tactic.NewGoal(m2, false, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.Assert(fixDLApp(t, m2, 2, x, zero)))
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("probe")
	equationMust(t, p.BindTrace(tags))
	value, err := p.Eval(goal)
	equationMust(t, err)
	if value != 0 {
		t.Fatal("real goal accepted as LIA")
	}
	if tags.ShouldTrace("probe") && !strings.Contains(output.String(), "other-domain-x") {
		t.Fatal("probe did not use the goal's compatible printer domain", output.String())
	}
}

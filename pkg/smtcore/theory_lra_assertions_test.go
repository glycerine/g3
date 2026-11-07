package smtcore

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func newLRASourceAssertionContext(t *testing.T) (*diagnostic.DebugContext, string) {
	t.Helper()
	b := arena.NewBudget(1 << 24)
	region, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: b}})
	astTestMust(t, err)
	d, err := diagnostic.NewDebugContext(region, b)
	astTestMust(t, err)
	inputFile, err := os.CreateTemp(t.TempDir(), "lra-source-assert-input-*")
	astTestMust(t, err)
	_, err = inputFile.WriteString(strings.Repeat("T", 64))
	astTestMust(t, err)
	_, err = inputFile.Seek(0, 0)
	astTestMust(t, err)
	report, err := os.CreateTemp(t.TempDir(), "lra-source-assert-report-*")
	astTestMust(t, err)
	streams, err := diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: b, Stdout: report, Stderr: report})
	astTestMust(t, err)
	input, err := diagnostic.NewDebuggerCharInput(streams, inputFile)
	astTestMust(t, err)
	actions, err := diagnostic.NewDebugActionContext(diagnostic.DebugActionConfig{Budget: b, Streams: streams, Input: input})
	astTestMust(t, err)
	astTestMust(t, d.BindAssertionStreams(streams))
	astTestMust(t, d.BindAssertionActions(actions))
	previous, err := diagnostic.BindProcessDebugContext(d)
	astTestMust(t, err)
	t.Cleanup(func() {
		_, err := diagnostic.BindProcessDebugContext(previous)
		astTestMust(t, err)
		astTestMust(t, actions.Close())
		astTestMust(t, input.Close())
		astTestMust(t, streams.Close())
		astTestMust(t, inputFile.Close())
		astTestMust(t, report.Close())
		astTestMust(t, d.Close())
		astTestMust(t, region.Close())
		if b.Used() != 0 {
			t.Fatal("assertion owner leak", b.Used())
		}
	})
	return d, report.Name()
}

func checkLRASourceAssertionResult(t *testing.T, err error, enabled bool, fallback error) {
	t.Helper()
	if enabled && diagnostic.DebugBuild {
		var ex z3exception.DefaultException
		if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
			t.Fatal("expected checked source assertion", err)
		}
	} else if !errors.Is(err, fallback) {
		t.Fatal("native/disabled guard changed", err, fallback)
	}
}
func TestSMTLRASourceAssertionAtomAndRawValueGates(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	for _, enabled := range []bool{false, true} {
		astTestMust(t, d.EnableAssertions(enabled))
		f := newLRAFrontendFixture(t, true)
		x := f.variable("assert-source-x")
		atom := f.app(smtArithOpGE, x, f.number(0))
		astTestMust(t, f.s.Internalize(atom, true))
		before := len(f.a.boundTrail)
		_, err := f.a.InternalizeAtom(atom, false)
		checkLRASourceAssertionResult(t, err, enabled, ast.ErrASTDomain)
		if enabled && diagnostic.DebugBuild && len(f.a.boundTrail) != before {
			t.Fatal("duplicate atom mutated before assertion")
		}
		// A separate unit objective has no LP mapping yet. Enabled checked reads
		// fail at source get_ivalue; native/disabled reads keep the Go domain guard.
		y := f.variable("assert-source-unmapped-y")
		v, err := f.a.AddObjective(y)
		astTestMust(t, err)
		_, err = f.a.ObjectiveValue(v)
		checkLRASourceAssertionResult(t, err, enabled, ast.ErrASTDomain)
		if f.a.numeric.stopped {
			t.Fatal("precondition assertion stopped numerical owner")
		}
	}
	text, err := os.ReadFile(report)
	astTestMust(t, err)
	if diagnostic.DebugBuild {
		for _, part := range []string{"689", "!ctx().b_internalized(atom)", "1027", "can_get_ivalue(v)"} {
			if !strings.Contains(string(text), part) {
				t.Fatal(part, string(text))
			}
		}
	} else if len(text) != 0 {
		t.Fatal("native assertions printed", string(text))
	}
}

func TestSMTLRASourceAssertionBoundAndEvidenceGates(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	for _, enabled := range []bool{false, true} {
		astTestMust(t, d.EnableAssertions(enabled))
		f := newLRAFrontendFixture(t, true)
		x, y := f.variable("assert-bound-x"), f.variable("assert-bound-y")
		xh, yh := f.app(smtArithOpGE, x, f.number(0)), f.app(smtArithOpLE, y, f.number(1))
		astTestMust(t, f.s.Internalize(xh, true))
		astTestMust(t, f.s.Internalize(yh, true))
		xb, _ := f.s.BoolVar(xh)
		yb, _ := f.s.BoolVar(yh)
		a, b := f.a.boundByBoolean[xb], f.a.boundByBoolean[yb]
		err := f.a.mkBoundAxiom(a, b)
		checkLRASourceAssertionResult(t, err, enabled, ast.ErrASTDomain)
		saved := f.a.bounds[a.variable]
		f.a.bounds[a.variable] = nil
		err = f.a.propagateBasicBound(a, true)
		f.a.bounds[a.variable] = saved
		checkLRASourceAssertionResult(t, err, enabled, nil)
		ci, err := f.a.numeric.assertBound(a.variable, a.boolean, &a.value, a.upper, true)
		astTestMust(t, err)
		old := f.a.numeric.inequalities[ci]
		f.a.numeric.inequalities[ci] = SMTNullLiteral
		hook := smtLRABoundPropagation{owner: f.a}
		err = hook.Consume(nil, ci)
		f.a.numeric.inequalities[ci] = old
		checkLRASourceAssertionResult(t, err, enabled, ast.ErrASTDomain)
		if len(hook.core) != 0 {
			t.Fatal("null evidence published before assertion")
		}
		astTestMust(t, hook.Consume(nil, smtLRANoIndex))
	}
	text, err := os.ReadFile(report)
	astTestMust(t, err)
	if diagnostic.DebugBuild {
		for _, part := range []string{"1553", "v == b2.get_var()", "1758", "!bounds.empty()", "2187", "lit != null_literal"} {
			if !strings.Contains(string(text), part) {
				t.Fatal(part, string(text))
			}
		}
	} else if len(text) != 0 {
		t.Fatal(string(text))
	}
}

func TestSMTLRASourceAssertionReusedVariableBounds(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	for _, enabled := range []bool{false, true} {
		astTestMust(t, d.EnableAssertions(enabled))
		f := newLRAFrontendFixture(t, true)
		x := f.variable("assert-reuse-x")
		h := f.app(smtArithOpGE, x, f.number(0))
		astTestMust(t, f.s.Internalize(h, true))
		bv, _ := f.s.BoolVar(h)
		next := f.a.NumVariables()
		// Simulate the stale slot that the native assertion protects when a
		// theory variable identifier is reused. The borrowed bound remains owned
		// by its original registered slot and trail throughout the test.
		f.a.bounds = append(f.a.bounds, []*smtLRABound{f.a.boundByBoolean[bv]})
		f.a.unassigned = append(f.a.unassigned, 0)
		y := f.variable("assert-reuse-y")
		v, err := f.a.mkVar(y)
		f.a.bounds[next] = nil
		checkLRASourceAssertionResult(t, err, enabled, nil)
		if enabled && diagnostic.DebugBuild && (f.a.NumVariables() != next+1 || f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID()) != SMTNullTheoryVar) {
			t.Fatal("source invariant must follow variable allocation and precede attachment")
		}
		if !(enabled && diagnostic.DebugBuild) && v != SMTTheoryVar(next) {
			t.Fatal(v, next)
		}
	}
	text, err := os.ReadFile(report)
	astTestMust(t, err)
	if diagnostic.DebugBuild {
		if !strings.Contains(string(text), "476") || !strings.Contains(string(text), "m_bounds[v].empty()") {
			t.Fatal(string(text))
		}
	} else if len(text) != 0 {
		t.Fatal(string(text))
	}
}

func TestSMTLRASourceAssertionsActualLPAndScopePath(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	astTestMust(t, d.EnableAssertions(true))
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("assert-live-x"), f.variable("assert-live-y")
	target := f.app(smtArithOpGE, x, f.number(3))
	astTestMust(t, f.s.Internalize(target, true))
	astTestMust(t, f.a.Propagate())
	astTestMust(t, f.s.PushScope())
	sum := f.app(smtArithOpAdd, x, y)
	f.assert(f.app(smtArithOpGE, sum, f.number(5)), true)
	f.assert(f.app(smtArithOpLE, y, f.number(2)), true)
	f.propagate(true)
	literal, err := f.s.Literal(target)
	astTestMust(t, err)
	value, err := f.s.Assignment(literal)
	astTestMust(t, err)
	if value != lbool.True || f.a.lpPropagations != 1 {
		t.Fatal("checked LP implication lost", value, f.a.lpPropagations)
	}
	objective, err := f.a.AddObjective(sum)
	astTestMust(t, err)
	_, err = f.a.ObjectiveValue(objective)
	astTestMust(t, err)
	astTestMust(t, f.s.PopScopes(1))
	// Reuse the popped term variable identifiers with different arithmetic.
	z := f.variable("assert-live-z")
	term := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number(2), z), f.number(7))
	_, err = f.a.AddObjective(term)
	astTestMust(t, err)
	text, err := os.ReadFile(report)
	astTestMust(t, err)
	if len(text) != 0 {
		t.Fatal("valid algorithm triggered source assertion", string(text))
	}
	checkNativeLRANumeric(t)
}

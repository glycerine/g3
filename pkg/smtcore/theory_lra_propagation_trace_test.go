package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestSMTContextLiteralDisplaysForLRAEvidence(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("display-evidence-x")
	h := f.app(smtArithOpGE, x, f.number(2))
	astTestMust(t, f.s.Internalize(h, true))
	literal, err := f.s.Literal(h)
	astTestMust(t, err)
	var out bytes.Buffer
	astTestMust(t, f.s.DisplayLiteralsVerbose(&out, []SMTLiteral{SMTTrueLiteral, literal, literal.Negated(), SMTFalseLiteral, SMTNullLiteral}))
	want := "true\n(>= display-evidence-x 2.0)\n(not (>= display-evidence-x 2.0))\nfalse\nnull"
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	out.Reset()
	astTestMust(t, f.s.DisplayDetailedLiteral(&out, literal.Negated()))
	if out.String() != "(not (>= display-evidence-x 2.0))" {
		t.Fatal(out.String())
	}
	out.Reset()
	astTestMust(t, f.s.DisplayLiteralVerbose(&out, literal))
	if out.String() != "(>= display-evidence-x 2.0)" {
		t.Fatal(out.String())
	}
	if err = f.s.DisplayLiteralsVerbose(nil, nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if err = f.s.DisplayDetailedLiteral(&out, NewSMTLiteral(99999, false)); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if err = f.s.DisplayLiteralVerbose(smtLRAShortOutput{}, literal); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	for _, literals := range [][]SMTLiteral{{SMTTrueLiteral}, {SMTFalseLiteral}, {SMTNullLiteral}, {literal}, {literal.Negated()}, {literal, SMTTrueLiteral}} {
		if err := f.s.DisplayLiteralsVerbose(smtLRAShortOutput{}, literals); !errors.Is(err, io.ErrShortWrite) {
			t.Fatal("short literal output", literals, err)
		}
	}
	out.Reset()
	astTestMust(t, f.s.DisplayLiteralsVerbose(&out, nil))
	if out.Len() != 0 {
		t.Fatal(out.String())
	}
}

func TestSMTLRAPropagationTraceNumericalBoundAndRefusedOutput(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x, y := f.variable("trace-implied-x"), f.variable("trace-implied-y")
		target := f.app(smtArithOpGE, x, f.number(3))
		astTestMust(t, f.s.Internalize(target, true))
		astTestMust(t, f.a.Propagate())
		sum := f.app(smtArithOpAdd, x, y)
		premise := f.app(smtArithOpGE, sum, f.number(5))
		other := f.app(smtArithOpLE, y, f.number(2))
		f.assert(premise, true)
		f.assert(other, true)
		w, tags := lraFrontendTrace(f, refuse)
		f.propagate(true)
		literal, err := f.s.Literal(target)
		astTestMust(t, err)
		assignment, err := f.s.Assignment(literal)
		astTestMust(t, err)
		if assignment != lbool.True || f.a.lpPropagations != 1 || f.a.propagator != nil || f.a.numeric.stopped {
			t.Fatal("actual propagation changed", assignment, f.a.lpPropagations)
		}
		if tags.ShouldTrace("arith") && !refuse {
			text := w.output.String()
			variable := f.a.boundByBoolean[literal.Var()].variable
			want := lraObjectiveTraceRecord("arith", "propagate_lp_solver_bound", 1383, fmt.Sprintf("v%d >= 3\n", variable))
			if !strings.Contains(text, want) {
				t.Fatal("missing exact implied point", text)
			}
			start := strings.Index(text, "propagate_lp_solver_bound src/smt/theory_lra.cpp:1415")
			if start < 0 {
				t.Fatal(text)
			}
			evidence := text[start:]
			// mk_pp uses SMT2 formatting; a single-use sum is printed inline.
			for _, part := range []string{"(>= (+ trace-implied-x trace-implied-y) 5.0)", "(<= trace-implied-y 2.0)", "\n --> (>= trace-implied-x 3.0)\n", "implied bound\n", " >= 3\nend of implied bound\n"} {
				if !strings.Contains(evidence, part) {
					t.Fatal(part, evidence)
				}
			}
			counter := strings.LastIndex(text, "updt_unassigned_bounds src/smt/theory_lra.cpp:595")
			if counter < 0 || counter > start {
				t.Fatal("evidence before decrement", text)
			}
		} else {
			checkLRAFrontendOutput(t, w, tags, "")
		}
	}
}

func TestSMTLRAPropagationTraceInterestAndConflict(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("trace-interest-x")
	v, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	index, err := f.a.numeric.getVarIndex(v)
	astTestMust(t, err)
	w, tags := lraFrontendTrace(f, false)
	hook := smtLRABoundPropagation{owner: f.a}
	// The source early return prints without evaluating the numerical operand.
	interesting, err := hook.BoundIsInteresting(index, 0, nil)
	astTestMust(t, err)
	if interesting {
		t.Fatal("empty bound list is interesting")
	}
	checkLRAFrontendOutput(t, w, tags, lraObjectiveTraceRecord("arith", "bound_is_interesting", 1339, "return\n"))
	for _, refuse := range []bool{false, true} {
		g := newLRAFrontendFixture(t, true)
		a, b := g.variable("trace-conflict-a"), g.variable("trace-conflict-b")
		g.assert(g.app(smtArithOpGE, a, g.number(0)), true)
		g.assert(g.app(smtArithOpGE, b, g.number(0)), true)
		g.assert(g.app(smtArithOpLE, g.app(smtArithOpAdd, a, b), g.number(-1)), true)
		w, tags = lraFrontendTrace(g, refuse)
		g.propagate(false)
		if g.a.conflictCount != 1 {
			t.Fatal("conflict lost")
		}
		if tags.ShouldTrace("arith") && !refuse {
			text := w.output.String()
			first := strings.Index(text, lraObjectiveTraceRecord("arith", "propagate", 1290, "propagation conflict\n"))
			second := strings.Index(text, "set_conflict src/smt/theory_lra.cpp:2222")
			if first < 0 || second < first {
				t.Fatal("conflict trace out of order", text)
			}
		} else {
			checkLRAFrontendOutput(t, w, tags, "")
		}
	}
}

func TestSMTLRAFixedEqualityTraceNestedQueryAndEvidence(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x := f.variable("trace-fixed-x")
		constant := f.number(7)
		cv, err := f.a.internalizeDef(constant)
		astTestMust(t, err)
		index, _ := f.a.numeric.index(cv)
		lraFix(t, f, x, 7)
		w, tags := lraFrontendTrace(f, refuse)
		f.propagate(true)
		if f.s.context.enodes.get(x).Root() != f.s.context.enodes.get(constant).Root() || f.a.fixedEqualities != 1 {
			t.Fatal("fixed numeral merge lost")
		}
		if tags.ShouldTrace("arith") && !refuse {
			text := w.output.String()
			nested := lraObjectiveTraceRecord("arith", "has_bound", 2057, fmt.Sprintf("%d %d\n", index, cv))
			record := lraObjectiveTraceRecord("arith", "fixed_var_eh", 2098, "fixed: trace-fixed-x 7.0 7 "+nested+"1\n")
			if !strings.Contains(text, record) || strings.Count(text, nested) != 3 {
				t.Fatal("trace-only query omitted, added to native work, or reordered", text)
			}
			evidence := lraObjectiveTraceRecord("arith", "fixed_var_eh", 2116, "(>= trace-fixed-x 7.0)\n(<= trace-fixed-x 7.0)\n ==> trace-fixed-x = 7.0\n")
			if !strings.Contains(text, evidence) || strings.Index(text, evidence) < strings.Index(text, record) {
				t.Fatal("fixed evidence", text)
			}
		} else {
			checkLRAFrontendOutput(t, w, tags, "")
		}
		_, found, err := f.a.hasFixedBound(index, big.NewRat(7, 1), true)
		astTestMust(t, err)
		if !found {
			t.Fatal("trace damaged implicit numeral bound")
		}
	}
}

func TestSMTLRAPropagationCoreSourceAssertionGate(t *testing.T) {
	b := arena.NewBudget(1 << 24)
	region, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: b}})
	astTestMust(t, err)
	d, err := diagnostic.NewDebugContext(region, b)
	astTestMust(t, err)
	inputFile, err := os.CreateTemp(t.TempDir(), "propagation-assert-input-*")
	astTestMust(t, err)
	_, err = inputFile.WriteString("T")
	astTestMust(t, err)
	_, err = inputFile.Seek(0, 0)
	astTestMust(t, err)
	report, err := os.CreateTemp(t.TempDir(), "propagation-assert-report-*")
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
	defer func() { _, err := diagnostic.BindProcessDebugContext(previous); astTestMust(t, err) }()
	t.Cleanup(func() {
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
	f := newLRAFrontendFixture(t, true)
	astTestMust(t, d.EnableAssertions(false))
	astTestMust(t, f.a.checkLRAPropagationCore([]SMTLiteral{SMTNullLiteral}))
	astTestMust(t, d.EnableAssertions(true))
	astTestMust(t, f.a.checkLRAPropagationCore([]SMTLiteral{SMTTrueLiteral}))
	err = f.a.checkLRAPropagationCore([]SMTLiteral{SMTFalseLiteral})
	if diagnostic.DebugBuild {
		var ex z3exception.DefaultException
		if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
			t.Fatal(err)
		}
	} else {
		astTestMust(t, err)
		astTestMust(t, f.a.checkLRAPropagationCore([]SMTLiteral{SMTNullLiteral}))
	}
	text, err := os.ReadFile(report.Name())
	astTestMust(t, err)
	if diagnostic.DebugBuild {
		for _, part := range []string{"src/smt/theory_lra.cpp", "1425", "ctx().get_assignment(lit) == l_true"} {
			if !strings.Contains(string(text), part) {
				t.Fatal(part, string(text))
			}
		}
	} else if len(text) != 0 {
		t.Fatal(string(text))
	}
}

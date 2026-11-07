package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func datalogTraceRecord(function string, line int, body string) string {
	return fmt.Sprintf("-------- [theory_dl] %s src/smt/theory_dl.cpp:%d ---------\n%s\n------------------------------------------------\n", function, line, body)
}

func TestSMTDatalogSourceTraceEntryAndAttachmentOrder(t *testing.T) {
	f := newDLModelFixture(t, 8)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("theory_dl")
	f.dl.trace = tags
	term, err := f.m.MkConst(astTestSymbol(t, f.m, "dl_trace_term"), f.sort)
	astTestMust(t, err)
	astTestPin(t, f.m, term)
	_, err = f.dl.InternalizeTerm(term)
	astTestMust(t, err)
	want := datalogTraceRecord("internalize_term", 140, "dl_trace_term") + datalogTraceRecord("mk_rep", 234, "dl_trace_term")
	if tags.ShouldTrace("theory_dl") {
		if output.String() != want {
			t.Fatal("cold term trace/attachment order", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native trace not erased")
	}
	if !f.dl.IsAttachedToVariable(f.s.context.enodes.get(term)) {
		t.Fatal("trace lost native attachment")
	}
	output.Reset()
	_, err = f.dl.InternalizeTerm(term)
	astTestMust(t, err)
	if tags.ShouldTrace("theory_dl") && output.String() != datalogTraceRecord("internalize_term", 140, "dl_trace_term") {
		t.Fatal("attached term must keep entry trace and skip mk_rep trace", output.String())
	}
	truth, err := f.m.MkTrue()
	astTestMust(t, err)
	output.Reset()
	handled, err := f.dl.InternalizeTerm(truth)
	astTestMust(t, err)
	if handled {
		t.Fatal("nonfinite term handled")
	}
	if tags.ShouldTrace("theory_dl") && output.String() != datalogTraceRecord("internalize_term", 140, "true") {
		t.Fatal("term trace must precede sort guard", output.String())
	}
	// A previously Boolean-internalized expression still emits entry TRACE.
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	atom, err := f.m.MkConst(astTestSymbol(t, f.m, "dl_trace_atom"), boolean)
	astTestMust(t, err)
	_, err = f.s.MkBoolVar(atom)
	astTestMust(t, err)
	output.Reset()
	handled, err = f.dl.InternalizeAtom(atom, true)
	astTestMust(t, err)
	if !handled {
		t.Fatal("existing Boolean variable guard")
	}
	if tags.ShouldTrace("theory_dl") && output.String() != datalogTraceRecord("internalize_atom", 117, "dl_trace_atom") {
		t.Fatal("atom trace must precede internalized guard", output.String())
	}
	output.Reset()
	astTestMust(t, f.dl.assertConstraint(truth))
	if tags.ShouldTrace("theory_dl") && output.String() != datalogTraceRecord("assert_cnstr", 272, "true") {
		t.Fatal("constraint trace", output.String())
	}
	if f.dl.Name() != "datalog" {
		t.Fatal("source theory name")
	}
	output.Reset()
	astTestMust(t, f.dl.Display(&output))
	astTestMust(t, f.dl.Display(nil))
	if output.Len() != 0 {
		t.Fatal("source empty display emitted text")
	}
}

type datalogClosedTraceWriter struct{}

func (datalogClosedTraceWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestSMTDatalogSourceTraceErasureAndOstreamFailure(t *testing.T) {
	f := newDLModelFixture(t, 8)
	tags := diagnostic.NewTraceTagsWithWriter(datalogClosedTraceWriter{})
	f.dl.trace = tags
	// Disabled TRACE must not read the invalid expression operand.
	astTestMust(t, f.dl.traceDatalogExpression("internalize_atom", 117, ast.ASTHandle{}))
	tags.Enable("theory_dl")
	truth, err := f.m.MkTrue()
	astTestMust(t, err)
	// Source ostream badbit does not turn a successfully computed body into a
	// theory failure; later operands still execute even after the bad header.
	astTestMust(t, f.dl.traceDatalogExpression("internalize_atom", 117, truth))
	err = f.dl.traceDatalogExpression("internalize_atom", 117, ast.ASTHandle{})
	if tags.ShouldTrace("theory_dl") {
		if err == nil || errors.Is(err, io.ErrClosedPipe) {
			t.Fatal("badbit suppressed the real operand refusal", err)
		}
	} else {
		astTestMust(t, err)
	}
}

func TestSMTDatalogSourceDeclarationAssertionGateAndCache(t *testing.T) {
	// Construct the real service owners before binding the process context.
	// Cache hit/miss probes alone do not distinguish this gate from every
	// downstream assertion-owner read; its error-return coverage is separate.
	f := newDLModelFixture(t, 8)
	previous := diagnostic.BorrowProcessDebugContext()
	d, _ := newLRASourceAssertionContext(t)
	defer func() { _, err := diagnostic.BindProcessDebugContext(previous); astTestMust(t, err) }()
	_, err := f.dl.declarationsFor(f.sort)
	astTestMust(t, err)
	cold, err := f.m.MkDatalogFiniteSort(astTestSymbol(t, f.m, "cold-declaration-domain"), 9)
	astTestMust(t, err)
	astTestPin(t, f.m, cold)
	astTestMust(t, d.EnableAssertions(false))
	calls := 0
	astTestMust(t, datalogAssert(205, "!m_reps.contains(s)", func() bool { calls++; return false }))
	if calls != 0 {
		t.Fatal("disabled source assertion evaluated operand")
	}
	astTestMust(t, d.Close())
	// Source cached get_rep has no assertion-owner access.
	_, err = f.dl.declarationsFor(f.sort)
	astTestMust(t, err)
	before := f.m.Stats()
	_, err = f.dl.declarationsFor(cold)
	if diagnostic.DebugBuild {
		if !errors.Is(err, arena.ErrClosed) || f.m.Stats() != before {
			t.Fatal("line205 must consult owner before building declarations", err)
		}
	} else {
		astTestMust(t, err)
	}
	_, err = f.dl.datalogMaximum(f.sort)
	if diagnostic.DebugBuild {
		if !errors.Is(err, arena.ErrClosed) {
			t.Fatal("valid max_value reads assertion owner at247", err)
		}
	} else {
		astTestMust(t, err)
	}
}

func TestSMTDatalogSourceMaximumVerificationAndActions(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, action := range []string{"T", "C"} {
			t.Run(fmt.Sprintf("%t/%s", enabled, action), func(t *testing.T) {
				d, _ := newLRASourceAssertionContext(t)
				report := bvDebugActionInput(t, d, action)
				f := newDLModelFixture(t, 8)
				nonfinite, err := f.m.MkUninterpretedSort(astTestSymbol(t, f.m, "unbounded-max-domain"), nil)
				astTestMust(t, err)
				astTestPin(t, f.m, nonfinite)
				astTestMust(t, d.EnableAssertions(enabled))
				_, err = f.dl.datalogMaximum(nonfinite)
				if diagnostic.DebugBuild && action == "T" {
					var ex z3exception.DefaultException
					if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
						t.Fatal("VERIFY throw ignores assertion switch", err)
					}
				} else if diagnostic.DebugBuild {
					if !errors.Is(err, ast.ErrASTDomain) {
						t.Fatal("continued VERIFY must stop at uninitialized source sz", err)
					}
				} else if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
					t.Fatal("release VERIFY must evaluate and report before native unreachable guard", err)
				}
				data, e := os.ReadFile(report)
				astTestMust(t, e)
				if !strings.Contains(string(data), "Failed to verify: u().try_get_size(s, sz)\n") {
					t.Fatal("source VERIFY message", string(data))
				}
				if diagnostic.DebugBuild && !strings.Contains(string(data), "File: src/smt/theory_dl.cpp\nLine: 246\nUNREACHABLE CODE WAS REACHED.\n") {
					t.Fatal("actual unreachable notification", string(data))
				}
			})
		}
	}
}

func TestSMTDatalogSourceZeroMaximumAssertionActions(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, action := range []string{"T", "C"} {
			t.Run(fmt.Sprintf("%t/%s", enabled, action), func(t *testing.T) {
				d, _ := newLRASourceAssertionContext(t)
				report := bvDebugActionInput(t, d, action)
				f := newDLModelFixture(t, 8)
				info, err := f.m.NewSortInfo(ast.NullFamilyID, 41, nil, ast.FiniteSortSize(0), false)
				astTestMust(t, err)
				defer func() { astTestMust(t, f.m.DelSortInfo(&info)) }()
				zero, err := f.m.MkSort(astTestSymbol(t, f.m, "zero-max-sort"), info)
				astTestMust(t, err)
				astTestPin(t, f.m, zero)
				astTestMust(t, d.EnableAssertions(enabled))
				value, err := f.dl.datalogMaximum(zero)
				if diagnostic.DebugBuild && enabled && action == "T" {
					var ex z3exception.DefaultException
					if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
						t.Fatal("zero maximum assertion throw", err)
					}
				} else {
					astTestMust(t, err)
					text, matched, e := f.m.BitVectorNumeralText(value)
					astTestMust(t, e)
					if !matched || text != "18446744073709551615" {
						t.Fatal("source unsigned sz-1 continuation", text, matched)
					}
				}
				data, e := os.ReadFile(report)
				astTestMust(t, e)
				want := 0
				if diagnostic.DebugBuild && enabled {
					want = 1
				}
				if strings.Count(string(data), "File: src/smt/theory_dl.cpp\nLine: 247\nsz > 0\n") != want {
					t.Fatal("source maximum assertion notification", string(data))
				}
			})
		}
	}
}

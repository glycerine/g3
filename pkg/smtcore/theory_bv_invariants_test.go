package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func bvInvariantFailure(t *testing.T, err error, report string, line int, condition string) {
	t.Helper()
	if !diagnostic.DebugBuild {
		astTestMust(t, err)
		text, e := os.ReadFile(report)
		astTestMust(t, e)
		if len(text) != 0 {
			t.Fatal("native invariant notification was not erased", string(text))
		}
		return
	}
	var ex z3exception.DefaultException
	if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
		t.Fatal("source invariant did not throw through its bound debugger", err)
	}
	text, e := os.ReadFile(report)
	astTestMust(t, e)
	if !strings.Contains(string(text), fmt.Sprintf("File: src/smt/theory_bv.cpp\nLine: %d\n%s\n", line, condition)) {
		t.Fatal("wrong source invariant site", string(text))
	}
}

func TestSMTBVInvariantFinalCheckAssertionGateAndPriority(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	m, s, bv := bvOccurrenceFixture(t)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	_, v := bvNativeWord(t, m, s, bv, truth)
	bv.zeroOneBits[v] = nil
	bv.unsupported = true
	before := m.Stats()
	astTestMust(t, d.EnableAssertions(false))
	original := bv.variables[v]
	bv.variables[v] = nil
	status, err := bv.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckGiveUp || m.Stats() != before {
		t.Fatal("disabled invariant evaluated before giveup")
	}
	bv.variables[v] = original
	astTestMust(t, d.EnableAssertions(true))
	status, err = bv.FinalCheck()
	bvInvariantFailure(t, err, report, 1740, "_bits.size() == num_bits")
	if !diagnostic.DebugBuild && status != SMTFinalCheckGiveUp {
		t.Fatal("native giveup changed")
	}
	if m.Stats() != before {
		t.Fatal("read-side invariant changed AST owners")
	}
}

func TestSMTBVInvariantAssignmentGuardsAndSourceSites(t *testing.T) {
	for _, mode := range []string{"nonroot", "irrelevant-root", "width", "value", "member-relevance", "conflict-still-checks"} {
		t.Run(mode, func(t *testing.T) {
			_, report := newLRASourceAssertionContext(t)
			m, s, bv := bvOccurrenceFixture(t)
			left, lv := bvNativeWord(t, m, s, bv, smtRelevancyBool(t, m, "invariant-p"))
			right, rv := bvNativeWord(t, m, s, bv, smtRelevancyBool(t, m, "invariant-q"))
			astTestMust(t, bv.NewEquality(lv, rv))
			astTestMust(t, s.context.SetRelevant(left.owner, true))
			astTestMust(t, s.context.SetRelevant(right.owner, true))
			root, err := bv.privateRoot(lv)
			astTestMust(t, err)
			child := lv
			if child == root {
				child = rv
			}
			target := root
			line := 1697
			condition := "val1 == val2"
			switch mode {
			case "nonroot":
				target = child
				bv.bitLiterals[child] = []SMTLiteral{SMTNullLiteral}
			case "irrelevant-root":
				node, err := bv.Enode(root)
				astTestMust(t, err)
				astTestMust(t, s.context.SetRelevant(node.owner, false))
				bv.bitLiterals[child] = []SMTLiteral{SMTNullLiteral}
			case "width":
				bv.bitLiterals[child] = nil
				line = 1684
				condition = "bits1.size() == bits2.size()"
			case "value", "conflict-still-checks":
				astTestMust(t, s.AssignLiteral(bv.bitLiterals[child][0], NewSMTBooleanAxiom()))
				if mode == "conflict-still-checks" {
					s.inconsistent = true
				}
			case "member-relevance":
				node, err := bv.Enode(child)
				astTestMust(t, err)
				astTestMust(t, s.context.SetRelevant(node.owner, false))
				line = 1699
				condition = "ctx.is_relevant(get_enode(v1))"
			}
			err = bv.checkBVAssignment(target)
			if mode == "nonroot" || mode == "irrelevant-root" {
				astTestMust(t, err)
			} else {
				bvInvariantFailure(t, err, report, line, condition)
			}
		})
	}
}

func TestSMTBVInvariantConstantSummarySourceGuardsAndSites(t *testing.T) {
	for _, mode := range []string{"complement", "missing", "owner", "polarity", "duplicate", "nonroot", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			_, report := newLRASourceAssertionContext(t)
			m, s, bv := bvOccurrenceFixture(t)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			falsity, err := m.MkFalse()
			astTestMust(t, err)
			_, lv := bvNativeWord(t, m, s, bv, truth, falsity, smtRelevancyBool(t, m, "summary-p"))
			_, rv := bvNativeWord(t, m, s, bv, truth, falsity, smtRelevancyBool(t, m, "summary-q"))
			_, other := bvNativeWord(t, m, s, bv, falsity, falsity, smtRelevancyBool(t, m, "summary-r"))
			astTestMust(t, bv.NewEquality(lv, rv))
			root, err := bv.privateRoot(lv)
			astTestMust(t, err)
			child := lv
			if root == child {
				child = rv
			}
			target := root
			line := 1740
			condition := "_bits.size() == num_bits"
			switch mode {
			case "complement":
				bv.bitLiterals[child][0] = SMTFalseLiteral
				line = 1728
				condition = "!bits[!is_true][i]"
			case "missing":
				bv.zeroOneBits[root] = bv.zeroOneBits[root][:1]
			case "owner":
				bv.zeroOneBits[root][0].variable = other
				line = 1746
				condition = "find(it->m_owner) == v"
			case "polarity":
				bv.zeroOneBits[root][0].isTrue = false
				line = 1747
				condition = "bits[it->m_is_true][it->m_idx]"
			case "duplicate":
				bv.zeroOneBits[root][1] = bv.zeroOneBits[root][0]
				line = 1748
				condition = "!already_found[it->m_idx]"
			case "nonroot":
				target = child
				bv.zeroOneBits[child] = nil
			case "conflict":
				s.inconsistent = true
				target = SMTNullTheoryVar
			}
			err = bv.checkBVZeroOneBits(target)
			if mode == "nonroot" || mode == "conflict" {
				astTestMust(t, err)
			} else {
				bvInvariantFailure(t, err, report, line, condition)
			}
		})
	}
}

func TestSMTBVInvariantAfterMergeCommitsAndTrailsBeforeAssertion(t *testing.T) {
	_, report := newLRASourceAssertionContext(t)
	m, s, bv := bvOccurrenceFixture(t)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	_, lv := bvNativeWord(t, m, s, bv, truth, smtRelevancyBool(t, m, "merge-check-p"))
	_, rv := bvNativeWord(t, m, s, bv, truth, smtRelevancyBool(t, m, "merge-check-q"))
	bv.zeroOneBits[rv][0].index = 1
	astTestMust(t, s.PushScope())
	before := len(bv.privateTrail)
	err = bv.NewEquality(lv, rv)
	bvInvariantFailure(t, err, report, 1740, "_bits.size() == num_bits")
	root, e := bv.privateRoot(lv)
	astTestMust(t, e)
	if root != rv || len(bv.privateTrail) != before+1 || bv.privateNext[lv] != rv || bv.privateNext[rv] != lv {
		t.Fatal("source after_merge assertion ran before the trailed link")
	}
	astTestMust(t, s.PopScopes(1))
	root, e = bv.privateRoot(lv)
	astTestMust(t, e)
	if root != lv || bv.privateNext[lv] != lv || bv.privateNext[rv] != rv {
		t.Fatal("asserted merge could not be undone")
	}
}

func TestSMTBVInvariantMismatchTraceExactRecordAndErasure(t *testing.T) {
	_, report := newLRASourceAssertionContext(t)
	m, s, bv := bvOccurrenceFixture(t)
	left, lv := bvNativeWord(t, m, s, bv, smtRelevancyBool(t, m, "trace-invariant-p"))
	right, rv := bvNativeWord(t, m, s, bv, smtRelevancyBool(t, m, "trace-invariant-q"))
	astTestMust(t, bv.NewEquality(lv, rv))
	astTestMust(t, s.context.SetRelevant(left.owner, true))
	astTestMust(t, s.context.SetRelevant(right.owner, true))
	astTestMust(t, s.PushScope())
	p, q := bv.bitLiterals[lv][0], bv.bitLiterals[rv][0]
	astTestMust(t, s.AssignLiteral(p, NewSMTBooleanAxiom()))
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("bv_bug")
	bv.trace = tags
	err := bv.checkBVAssignment(rv)
	bvInvariantFailure(t, err, report, 1697, "val1 == val2")
	var body bytes.Buffer
	fmt.Fprintln(&body, "equivalence class is inconsistent, i: 0")
	astTestMust(t, bv.DisplayVariable(&body, lv))
	astTestMust(t, bv.DisplayVariable(&body, rv))
	fmt.Fprintf(&body, "val1: l_true lvl: 1 bit p%d\nval2: l_undef lvl: 0 bit p%d\n", p.Var(), q.Var())
	want := fmt.Sprintf("-------- [bv_bug] check_assignment src/smt/theory_bv.cpp:1691 ---------\n%s------------------------------------------------\n", body.String())
	if diagnostic.DebugBuild && tags.ShouldTrace("bv_bug") {
		if output.String() != want {
			t.Fatal("source CTRACE record", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("erased invariant trace executed")
	}
	// Tag and condition gates precede the malformed display operands.
	tags.Disable("bv_bug")
	astTestMust(t, bv.traceBVAssignmentMismatch(-1, -1, 0, p, q, lbool.True, lbool.Undef))
	tags.Enable("bv_bug")
	astTestMust(t, bv.traceBVAssignmentMismatch(-1, -1, 0, p, q, lbool.True, lbool.True))
	if diagnostic.DebugBuild && tags.ShouldTrace("bv_bug") {
		before := m.Stats()
		for cut := 0; cut < len(want); cut++ {
			prefix := &smtLRAOutputPrefix{remaining: cut}
			failed := diagnostic.NewTraceTagsWithWriter(prefix)
			failed.Enable("bv_bug")
			bv.trace = failed
			astTestMust(t, bv.traceBVAssignmentMismatch(lv, rv, 0, p, q, lbool.True, lbool.Undef))
			if prefix.text.String() != want[:cut] {
				t.Fatal("ostream badbit prefix", cut, prefix.text.String(), want[:cut])
			}
		}
		if m.Stats() != before {
			t.Fatal("invariant trace changed AST owners")
		}
		prefix := &smtLRAOutputPrefix{remaining: 0}
		failed := diagnostic.NewTraceTagsWithWriter(prefix)
		failed.Enable("bv_bug")
		bv.trace = failed
		// ostream failure is sticky but does not prevent source operand reads.
		if e := bv.traceBVAssignmentMismatch(-1, -1, 0, p, q, lbool.True, lbool.Undef); !errors.Is(e, ast.ErrASTDomain) {
			t.Fatal("trace stream refusal hid the later operand error", e)
		}
	}
}

func TestSMTBVInvariantNonBVSummaryAndNativeErasure(t *testing.T) {
	m, solver, bv := bvConversionFixture(t, 2)
	s := solver.kernel.internalizer
	node, _ := bvFixedSymbolic(t, m, s, bv, "invariant-int-owner", 1)
	term, err := m.MkBitVectorToInt(node.owner)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(term, false))
	v := s.context.enodes.get(term).TheoryVar(bv.FamilyID())
	bv.zeroOneBits[v] = []smtBVZeroOneBit{{variable: SMTNullTheoryVar, index: 99, isTrue: true}}
	astTestMust(t, bv.checkBVZeroOneBits(v))
	if !diagnostic.DebugBuild {
		var erased SMTBitVectorTheory
		astTestMust(t, erased.checkBVAssignment(SMTNullTheoryVar))
		astTestMust(t, erased.checkBVZeroOneBits(SMTNullTheoryVar))
		astTestMust(t, erased.checkBVInvariant())
		astTestMust(t, erased.checkBVAfterMerge(SMTNullTheoryVar))
	}
}

func TestSMTBVInvariantContinueActionPreservesSourceReturn(t *testing.T) {
	d, _ := newLRASourceAssertionContext(t)
	reportName := bvDebugActionInput(t, d, "CCCCCCCC")
	m, s, bv := bvOccurrenceFixture(t)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	_, v := bvNativeWord(t, m, s, bv, truth)
	bv.zeroOneBits[v][0].isTrue = false
	status, err := bv.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone {
		t.Fatal("debugger continue changed source final-check return", status)
	}
	text, err := os.ReadFile(reportName)
	astTestMust(t, err)
	if diagnostic.DebugBuild {
		if !strings.Contains(string(text), "Line: 1747\nbits[it->m_is_true][it->m_idx]\n") || strings.Count(string(text), "ASSERTION VIOLATION") != 1 {
			t.Fatal("source C action notification", string(text))
		}
	} else if len(text) != 0 {
		t.Fatal("native C-action path was not erased", string(text))
	}
}

// Real stream/input/action owners are shared by the invariant and model tests.
func bvDebugActionInput(t *testing.T, d *diagnostic.DebugContext, commands string) string {
	t.Helper()
	inputFile, err := os.CreateTemp(t.TempDir(), "bv-invariant-continue-input-*")
	astTestMust(t, err)
	_, err = inputFile.WriteString(commands)
	astTestMust(t, err)
	_, err = inputFile.Seek(0, 0)
	astTestMust(t, err)
	report, err := os.CreateTemp(t.TempDir(), "bv-invariant-continue-report-*")
	astTestMust(t, err)
	streams, err := diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: d.Budget(), Stdout: report, Stderr: report})
	astTestMust(t, err)
	input, err := diagnostic.NewDebuggerCharInput(streams, inputFile)
	astTestMust(t, err)
	actions, err := diagnostic.NewDebugActionContext(diagnostic.DebugActionConfig{Budget: d.Budget(), Streams: streams, Input: input})
	astTestMust(t, err)
	astTestMust(t, d.BindAssertionActions(nil))
	astTestMust(t, d.BindAssertionStreams(streams))
	astTestMust(t, d.BindAssertionActions(actions))
	t.Cleanup(func() {
		astTestMust(t, d.BindAssertionActions(nil))
		astTestMust(t, d.BindAssertionStreams(nil))
		astTestMust(t, actions.Close())
		astTestMust(t, input.Close())
		astTestMust(t, streams.Close())
		astTestMust(t, inputFile.Close())
		astTestMust(t, report.Close())
	})
	return report.Name()
}

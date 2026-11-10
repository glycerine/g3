package smtcore

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSMTPBResolutionTraceNativeCutPreservesSourceOrder(t *testing.T) {
	m, s, pb, a, inputs := smtPBWatchFixture(t, []int64{4, 3, 2}, 6, false)
	x, err := s.BoolExpression(inputs[0].Var())
	astTestMust(t, err)
	notX, err := m.MkNot(x)
	astTestMust(t, err)
	h := smtPBResolveExpression(t, m, pb.FamilyID(), []ast.ASTHandle{notX, smtRelevancyBool(t, m, "trace-cut-z"), smtRelevancyBool(t, m, "trace-cut-w")}, []int32{3, 2, 1}, 4)
	astTestMust(t, s.Internalize(h, true))
	b, err := s.Literal(h)
	astTestMust(t, err)
	pb.controls.ConflictFrequency = 0
	astTestMust(t, s.PushScope())
	for _, l := range []SMTLiteral{a, b} {
		smtPBResolutionRoot(t, s, l, false)
		s.booleanData[l.Var()].Assumption = true
	}
	_, err = s.PropagateCore()
	astTestMust(t, err)
	astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
	s.searchUserBaseLevel = 0
	defer func() { astTestMust(t, s.endSearchPersistence()) }()
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("pb")
	tags.Enable("pb_verbose")
	pb.trace = tags
	astTestMust(t, s.PushScope())
	smtPBWatchAssign(t, s, inputs[1].Negated())
	conflict, ok := s.Conflict()
	if !ok {
		t.Fatal("native weighted cut did not conflict")
	}
	clause, ok := conflict.Clause()
	if !ok || clause == nil {
		t.Fatal("cut did not produce a clause")
	}
	// mk_clause sorts CLS_AUX_LEMMA literals (simplify_aux_lemma_literals).
	expectSMTRawCore(t, clause.literals, []SMTLiteral{a.Negated(), b.Negated()})
	if tags.ShouldTrace("pb") {
		last := -1
		for _, site := range []string{
			"[pb] resolve_conflict src/smt/theory_pb.cpp:1595",
			"[pb] process_antecedent src/smt/theory_pb.cpp:1529",
			"[pb_verbose] process_antecedent src/smt/theory_pb.cpp:1535",
			"[pb_verbose] resolve_conflict src/smt/theory_pb.cpp:1624",
			"[pb] resolve_conflict src/smt/theory_pb.cpp:1635",
			"[pb] resolve_conflict src/smt/theory_pb.cpp:1664",
			"[pb] resolve_conflict src/smt/theory_pb.cpp:1729",
			"[pb] resolve_conflict src/smt/theory_pb.cpp:1745",
			"[pb] resolve_conflict src/smt/theory_pb.cpp:1776",
		} {
			index := strings.Index(output.String(), site)
			if index <= last {
				t.Fatal("missing or reordered source cutting-plane trace", site, output.String())
			}
			last = index
		}
		want := smtPBTraceRecord("resolve_conflict", 1776, fmt.Sprintf("(not p%d) (not p%d)\n", b.Var(), a.Var()))
		if !strings.Contains(output.String(), want) {
			t.Fatal("guard negation trace", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native resolution trace was not erased")
	}
}

func TestSMTPBResolutionVerboseTagIndependentAndMarkPrefix(t *testing.T) {
	_, s, pb, _, inputs := smtPBWatchFixture(t, []int64{4, 3, 2}, 6, false)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(inputs[0].Negated(), NewSMTBooleanAxiom()))
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("pb_verbose")
	pb.trace = tags
	r := smtPBResolution{theory: pb, base: 0, level: 1, marks: make(map[int32]int)}
	astTestMust(t, r.antecedent(inputs[0], big.NewRat(3, 1)))
	if tags.ShouldTrace("pb_verbose") {
		want := fmt.Sprintf("-------- [pb_verbose] process_antecedent src/smt/theory_pb.cpp:1535 ---------\nante: p%d*3 1\n------------------------------------------------\n", inputs[0].Var())
		if output.String() != want {
			t.Fatal("independent pb_verbose record", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native verbose trace was not erased")
	}
	if r.numMarks != 1 || len(r.marks) != 1 {
		t.Fatal("tracing changed marks")
	}
}

func TestSMTPBResolvedLemmaDisplayAndVerboseGate(t *testing.T) {
	_, s, pb, _, inputs := smtPBWatchFixture(t, []int64{4, 3, 2}, 6, false)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(inputs[0].Negated(), NewSMTBooleanAxiom()))
	r := smtPBResolution{theory: pb, base: 0, level: 1, marks: map[int32]int{inputs[0].Var(): 0}, numMarks: 1}
	r.lemma.terms = []smtPBTerm{{literal: inputs[0], coeff: *big.NewRat(3, 1)}}
	r.lemma.bound.SetInt64(2)
	var output bytes.Buffer
	astTestMust(t, r.displayResolvedLemma(&output))
	want := fmt.Sprintf("num marks: 1\nconflict level: 1\n(not p%d)@ 1 m\nskipped: (not p%d):0\n3*p%d@(l_false:1) ~ 2\np%d@1 m l_false\nnum conflicts: 1\n", inputs[0].Var(), inputs[0].Var(), inputs[0].Var(), inputs[0].Var())
	if output.String() != want {
		t.Fatal("source resolved-lemma display", output.String(), want)
	}
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	astTestMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	astTestMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: arena.NewBudget(math.MaxUint64)})
	astTestMust(t, err)
	defer func() {
		astTestMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		astTestMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		astTestMust(t, stream.Close())
	}()
	astTestMust(t, diagnostic.SetProcessVerboseStream(stream))
	astTestMust(t, diagnostic.SetProcessVerbosityLevel(11))
	called := false
	astTestMust(t, smtPBVerbose(12, func(out io.Writer) error { called = true; return r.displayResolvedLemma(out) }))
	if called {
		t.Fatal("disabled IF_VERBOSE evaluated its body")
	}
	astTestMust(t, diagnostic.SetProcessVerbosityLevel(12))
	astTestMust(t, smtPBVerbose(12, r.displayResolvedLemma))
	got, err := stream.String()
	astTestMust(t, err)
	if got != want {
		t.Fatal("verbose source output", got, want)
	}
}

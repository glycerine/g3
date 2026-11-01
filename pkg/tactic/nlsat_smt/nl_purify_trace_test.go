package nlsatsmt

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/text"
)

type nlRefusedTrace struct{}

func (nlRefusedTrace) Write([]byte) (int, error) { return 0, errors.New("trace refused") }

func TestNLPurifySourceTraceOnNonlinearUnsat(t *testing.T) {
	m := astTestManager(t)
	_, _, config := nlModelEvaluatorFixture(t, m)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("nlsat_smt")
	config.Trace = tags
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x := analysisConst(t, m, real, "trace-nonlinear-x")
	h, err := m.ArithmeticRealNumeralText("0")
	zero := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticLE(x, zero)
	formula := analysisKeep(t, m, h, err)
	h, err = m.ArithmeticRealNumeralText("1")
	one := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticGE(x, one)
	opposite := analysisKeep(t, m, h, err)
	engine, err := NewNLPurifyTactic(m, config.Context, config.Globals, nil, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, engine.Close()) })
	run := func() {
		goal, err := tactic.NewGoal(m, false, true, false)
		astTestMust(t, err)
		defer func() { astTestMust(t, goal.Close()) }()
		astTestMust(t, goal.Assert(formula))
		astTestMust(t, goal.Assert(opposite))
		var result tactic.TacticResult
		defer func() { astTestMust(t, result.Close()) }()
		astTestMust(t, engine.Apply(goal, &result))
		if !result.DecidedUnsat() {
			t.Fatal("expected nonlinear contradiction")
		}
	}
	run()
	if tags.ShouldTrace("nlsat_smt") {
		for _, want := range []string{
			"operator() src/tactic/nlsat_smt/nl_purify_tactic.cpp:754",
			"solve src/tactic/nlsat_smt/nl_purify_tactic.cpp:337",
			"unsat\n",
			"solve src/tactic/nlsat_smt/nl_purify_tactic.cpp:411",
			"goal\n",
		} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("trace missing %q:\n%s", want, output.String())
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("disabled trace wrote output")
	}
	astTestMust(t, engine.Cleanup())
	refused := diagnostic.NewTraceTagsWithWriter(nlRefusedTrace{})
	refused.EnableAll(true)
	engine.services.Trace = refused
	run()
}

func TestNLPurifyReportsProofRefusal(t *testing.T) {
	m := astTestManager(t)
	_, _, config := nlModelEvaluatorFixture(t, m)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	goal, err := tactic.NewGoal(m, true, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	engine, err := NewNLPurifyTactic(m, config.Context, config.Globals, nil, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, engine.Close()) })
	stream, err := text.NewNumericTextStream(text.NumericTextStreamConfig{Budget: m.Budget()})
	astTestMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	astTestMust(t, err)
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	astTestMust(t, err)
	astTestMust(t, diagnostic.SetProcessVerboseStream(stream))
	astTestMust(t, diagnostic.SetProcessVerbosityLevel(10))
	t.Cleanup(func() {
		astTestMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		astTestMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		astTestMust(t, stream.Close())
	})
	var result tactic.TacticResult
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	err = engine.Apply(goal, &result)
	var exception tactic.TacticException
	if !errors.As(err, &exception) || !strings.Contains(exception.Message(), "does not support proof generation") {
		t.Fatal("expected source proof refusal", err)
	}
	got, err := stream.String()
	astTestMust(t, err)
	if !strings.Contains(got, "(qfufnl-purify :num-exprs ") {
		t.Fatal("missing tactic report", got)
	}
}

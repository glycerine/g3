package core

import (
	"bytes"
	"fmt"
	"github.com/glycerine/g3/pkg/util/statistics"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSolveEquationsExactTraceAndVerbose(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	p, err := c.NewValue()
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, c.Del(&p)) })
	key := "solve_eqs_max_occs"
	simplifyMust(t, c.SetUIntName(&p, &key, 5))
	runner, err := NewSolveEquationsTactic(m, c, g, &p, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	a, b, _ := propagateAtoms(t, m)
	eq, err := m.MkEq(a, b)
	simplifyMust(t, err)
	simplifyPin(t, m, eq)
	or, err := m.MkOr(a, b)
	simplifyMust(t, err)
	simplifyPin(t, m, or)
	goal, err := tactic.NewGoal(m, false, true, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(eq))
	simplifyMust(t, goal.Assert(b))
	simplifyMust(t, goal.Assert(or))
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	for _, tag := range []string{"solve_eqs", "solve_eqs_check_occs", "solve_eqs_bug2", "solve_eqs_subst", "solve_eqs_round", "gaussian_leak"} {
		tags.Enable(tag)
	}
	simplifyMust(t, runner.BindTrace(tags))
	verbose := diagnostic.BorrowProcessVerboseContext()
	previousLevel, err := verbose.VerbosityLevel()
	simplifyMust(t, err)
	previousStream, err := verbose.VerboseStream()
	simplifyMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	simplifyMust(t, err)
	t.Cleanup(func() {
		simplifyMust(t, verbose.SetVerbosityLevel(previousLevel))
		simplifyMust(t, verbose.SetVerboseStream(previousStream))
		simplifyMust(t, stream.Close())
	})
	simplifyMust(t, verbose.SetVerboseStream(stream))
	simplifyMust(t, verbose.SetVerbosityLevel(100))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	if !goal.IsDecidedSat() || statistics.Sums(runner.CollectStatistics)["eliminated vars"] != 2 {
		t.Fatal("tracing changed elimination")
	}
	output, err := stream.String()
	simplifyMust(t, err)
	if !strings.Contains(output, "num. eliminated vars: 2\n") {
		t.Fatalf("missing source verbose diagnostic: %q", output)
	}
	if !tags.ShouldTrace("solve_eqs") {
		if trace.Len() != 0 {
			t.Fatal("erased traces wrote output")
		}
		return
	}
	record := func(tag, function string, line int, body string) string {
		return fmt.Sprintf("-------- [%s] %s src/tactic/core/solve_eqs_tactic.cpp:%d ---------\n%s------------------------------------------------\n", tag, function, line, body)
	}
	empty := "(goal)\n"
	want := record("solve_eqs_check_occs", "check_occs", 90, "a num_occs: 2 max: 5\n") +
		record("solve_eqs_bug2", "solve", 369, "eliminating: b\n") +
		record("solve_eqs", "collect", 428, "candidate vars:\na b \n") +
		record("solve_eqs", "sort_vars", 569, "ordered vars:\nb a \n") +
		record("solve_eqs", "normalize", 606, "after normalizing variables\nb\n----->\ntrue\n\na\n----->\ntrue\n\n") +
		record("gaussian_leak", "substitute", 642, "processing:\n(= a b)\n") +
		record("gaussian_leak", "substitute", 642, "processing:\nb\n") +
		record("gaussian_leak", "substitute", 642, "processing:\n(or a b)\n") +
		record("solve_eqs_subst", "substitute", 654, "(or a b)\n--->\ntrue\n") +
		record("solve_eqs", "substitute", 666, "after applying substitution\n"+empty) +
		record("solve_eqs_round", "operator()", 775, empty+"(extension-model-converter\n  (b true)\n  (a true))\n") +
		record("solve_eqs", "collect", 428, "candidate vars:\n\n") +
		record("solve_eqs", "operator()", 780, empty)
	if trace.String() != want {
		t.Fatalf("source trace mismatch\nGo: %s\nwant: %s", trace.String(), want)
	}
}

func TestSolveEquationsSortTraceAndStreamRefusal(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	runner, err := NewSolveEquationsTactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	a, b, _ := propagateAtoms(t, m)
	eq, err := m.MkEq(a, b)
	simplifyMust(t, err)
	simplifyPin(t, m, eq)
	goal, err := tactic.NewGoal(m, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, goal.Assert(eq))
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("solve_eqs_bug")
	simplifyMust(t, runner.BindTrace(tags))
	simplifyMust(t, runner.Apply(goal, &result))
	if tags.ShouldTrace("solve_eqs_bug") {
		const sorting = "-------- [solve_eqs_bug] sort_vars src/tactic/core/solve_eqs_tactic.cpp:438 ---------\nsorting vars...\n------------------------------------------------\n"
		const processing = "-------- [solve_eqs_bug] sort_vars src/tactic/core/solve_eqs_tactic.cpp:466 ---------\nprocessing:\n%s\n------------------------------------------------\n"
		want := sorting + fmt.Sprintf(processing, "a") + fmt.Sprintf(processing, "b") + fmt.Sprintf(processing, "a")
		if trace.String() != want {
			t.Fatalf("source sort traversal differs: %q, want %q", trace.String(), want)
		}
	}
	simplifyMust(t, runner.Cleanup())
	simplifyMust(t, goal.Assert(eq))
	refused := diagnostic.NewTraceTagsWithWriter(simplifyRefusedWriter{})
	refused.EnableAll(true)
	simplifyMust(t, runner.BindTrace(refused))
	simplifyMust(t, runner.Apply(goal, &result))
	if !goal.IsDecidedSat() {
		t.Fatal("trace stream refusal changed the result")
	}
}

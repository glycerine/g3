package model_test

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Source: src/model/model_evaluator.cpp:149-166,566. The basic family
// returns m_b_rw.mk_app_core's status directly: no line-200 result record
// (AUDIT.md model_01-F6; build_trace/z3 writes none for (eval (and ...))).
func TestModelEvaluatorBooleanReductionSourceStatusTrace(t *testing.T) {
	m, _, ev, _, _, _ := evaluatorFixture(t)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	term, e := m.MkFamilyApp(0, ast.ASTOpAnd, nil, []ast.ASTHandle{yes, no})
	astTestMust(t, e)
	astTestPin(t, m, term)
	out := astTestPin(t, m, ast.ASTHandle{})
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	astTestMust(t, ev.BindTrace(tags))
	tags.Enable("model_evaluator")
	astTestMust(t, ev.Evaluate(term, out))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != no {
		t.Fatal("Boolean reduction", got)
	}
	if !tags.ShouldTrace("model_evaluator") {
		if output.Len() != 0 {
			t.Fatal("erased trace", output.String())
		}
		return
	}
	want := "-------- [model_evaluator] operator() src/model/model_evaluator.cpp:566 ---------\n" +
		"(and true false)\n" +
		"------------------------------------------------\n"
	if got := output.String(); got != want {
		t.Fatalf("source Boolean status trace: got %q, want %q", got, want)
	}
}

// Source: src/model/model_evaluator.cpp:180-202. Label elimination also
// reaches the conditional BR_DONE result record after application relookup.
func TestModelEvaluatorLabelReductionSourceStatusTrace(t *testing.T) {
	m, _, ev, _, _, _ := evaluatorFixture(t)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	term, e := m.MkLabel(true, []usymbol.Symbol{astTestSymbol(t, m, "trace-label")}, yes)
	astTestMust(t, e)
	astTestPin(t, m, term)
	out := astTestPin(t, m, ast.ASTHandle{})
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	astTestMust(t, ev.BindTrace(tags))
	tags.Enable("model_evaluator")
	astTestMust(t, ev.Evaluate(term, out))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != yes {
		t.Fatal("label reduction", got)
	}
	if !tags.ShouldTrace("model_evaluator") {
		if output.Len() != 0 {
			t.Fatal("erased trace", output.String())
		}
		return
	}
	want := "-------- [model_evaluator] operator() src/model/model_evaluator.cpp:566 ---------\n" +
		"(! true :lblpos trace-label)\n" +
		"------------------------------------------------\n" +
		"-------- [model_evaluator] reduce_app src/model/model_evaluator.cpp:200 ---------\n" +
		"true\n" +
		"------------------------------------------------\n"
	if got := output.String(); got != want {
		t.Fatalf("label status trace: got %q, want %q", got, want)
	}
}

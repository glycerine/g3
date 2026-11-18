package model_test

// Source: src/model/model_evaluator.cpp:565-577; src/solver/solver.cpp:87-94.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestModelEvaluatorValueOverloadOwnsResultAndTracesBothEntries(t *testing.T) {
	m, _, ev, _, _, _ := evaluatorFixture(t)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	term, e := m.MkFamilyApp(0, ast.ASTOpAnd, nil, []ast.ASTHandle{yes, no})
	astTestMust(t, e)
	astTestPin(t, m, term)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	astTestMust(t, ev.BindTrace(tags))
	tags.Enable("model_evaluator")
	value, e := ev.EvaluateValue(term)
	astTestMust(t, e)
	got, e := m.RefGet(value)
	astTestMust(t, e)
	if got != no {
		t.Fatal("value overload result", got)
	}
	if tags.ShouldTrace("model_evaluator") {
		want := "-------- [model_evaluator] operator() src/model/model_evaluator.cpp:572 ---------\n" +
			"(and true false)\n" +
			"------------------------------------------------\n" +
			"-------- [model_evaluator] operator() src/model/model_evaluator.cpp:566 ---------\n" +
			"(and true false)\n" +
			"------------------------------------------------\n"
		if output.String() != want {
			t.Fatalf("value overload source trace: got %q, want %q", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("erased value-overload trace", output.String())
	}
	astTestMust(t, m.DelRef(&value))
	if _, e := m.RefGet(value); e == nil {
		t.Fatal("released value reference stayed usable")
	}
}

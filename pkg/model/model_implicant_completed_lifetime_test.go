package model

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// model::eval constructs a new evaluator for every call, including recursive
// normalization calls inside one model_implicant::eval operation.
func TestModelImplicantCompletedEvaluationHasFreshLifetime(t *testing.T) {
	m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
	var pins []ast.ASTRef
	defer func() {
		for n := range pins {
			arrayTableMust(t, m.DelRef(&pins[n]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		arrayTableMust(t, err)
		ref, err := m.NewRef(h)
		arrayTableMust(t, err)
		pins = append(pins, ref)
		return h
	}
	boolean := own(m.MkBoolSort())
	no := own(m.MkFalse())
	yes := own(m.MkTrue())
	symbol, err := m.Symbols.Intern("completed-lifetime-f")
	arrayTableMust(t, err)
	function := own(m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	bound := own(m.MkVar(0, boolean))
	body := own(m.MkNot(bound))
	graph, err := NewFunctionInterpretation(m, 1)
	arrayTableMust(t, err)
	arrayTableMust(t, graph.SetElse(body))
	arrayTableMust(t, md.Core().RegisterFunction(function, graph))
	expression := own(m.MkApp(function, []ast.ASTHandle{no}))
	arrayTableMust(t, implicant.setupModel(md))
	defer func() { arrayTableMust(t, implicant.reset()) }()
	before := m.Stats()
	completion := implicant.evaluator.cfg.completion
	m.Budget().FailAfter(0)
	failed, err := implicant.evalCompletedExpression(expression)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || failed != (ast.ASTRef{}) {
		t.Fatal("failed evaluation did not return its construction failure", failed, err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		value, err := implicant.evalCompletedExpression(expression)
		arrayTableMust(t, err)
		got, err := m.RefGet(value)
		arrayTableMust(t, err)
		if got != yes {
			t.Fatal("retry produced wrong completed value", got, yes)
		}
		arrayTableMust(t, m.DelRef(&value))
		after := m.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
			t.Fatal("completed evaluation leaked ownership", after, before)
		}
		if implicant.evaluator.cfg.completion != completion {
			t.Fatal("local completion changed primary evaluator")
		}
	}
}

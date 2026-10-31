package fp_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestFixedpointExpressionAnswerSurvivesQueryAndInvalidates(t *testing.T) {
	f := newQueryExpressionFixture(t)
	sort := f.pin(f.manager.MkBoolSort())
	decl, _ := f.predicate("expression-answer", sort)
	truth := f.pin(f.manager.MkTrue())
	queryMust(t, f.engine.AddFact(decl, []ast.ASTHandle{truth}))
	variable := f.pin(f.manager.MkVar(0, sort))
	query := f.pin(f.manager.MkApp(decl, []ast.ASTHandle{variable}))
	status, err := f.engine.QueryExpression(query)
	queryMust(t, err)
	if status != lbool.True {
		t.Fatal("query", status)
	}
	answer, err := f.engine.ExpressionAnswer()
	queryMust(t, err)
	defer func() { queryMust(t, f.manager.DelRef(&answer)) }()
	h, err := f.manager.RefGet(answer)
	queryMust(t, err)
	expected := f.pin(f.manager.MkEq(variable, truth))
	if h != expected {
		t.Fatal("answer formula", h, expected)
	}
	no := f.pin(f.manager.MkFalse())
	queryMust(t, f.engine.AddFact(decl, []ast.ASTHandle{no}))
	if _, err = f.engine.ExpressionAnswer(); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("stale answer", err)
	}
	status, err = f.engine.QueryExpression(query)
	queryMust(t, err)
	if status != lbool.True {
		t.Fatal("second query", status)
	}
	next, err := f.engine.ExpressionAnswer()
	queryMust(t, err)
	queryMust(t, f.manager.DelRef(&next))
	if _, err = f.manager.RefGet(answer); err != nil {
		t.Fatal("earlier owned answer invalidated", err)
	}
	status, err = f.engine.QueryPredicate(decl)
	queryMust(t, err)
	if status != lbool.True {
		t.Fatal("relation query", status)
	}
	if _, err = f.engine.ExpressionAnswer(); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("expression answer survived relation query", err)
	}
}

func TestFixedpointExpressionAnswerForEmptyRelation(t *testing.T) {
	f := newQueryExpressionFixture(t)
	_, p := f.predicate("expression-empty")
	status, err := f.engine.QueryExpression(p)
	queryMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	answer, err := f.engine.ExpressionAnswer()
	queryMust(t, err)
	defer func() { queryMust(t, f.manager.DelRef(&answer)) }()
	h, err := f.manager.RefGet(answer)
	queryMust(t, err)
	if !f.manager.IsFalse(h) {
		t.Fatal("empty answer", h)
	}
}

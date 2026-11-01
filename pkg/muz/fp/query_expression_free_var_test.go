package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestFixedpointQueryExpressionFreeVariableMatchesAnyTuple(t *testing.T) {
	f := newQueryExpressionFixture(t)
	boolean := f.pin(f.manager.MkBoolSort())
	relation, _ := f.predicate("query-free-R", boolean)
	truth := f.pin(f.manager.MkTrue())
	queryMust(t, f.engine.AddFact(relation, []ast.ASTHandle{truth}))
	variable := f.pin(f.manager.MkVar(0, boolean))
	query := f.pin(f.manager.MkApp(relation, []ast.ASTHandle{variable}))
	result, err := f.engine.Query(query)
	queryMust(t, err)
	if result != lbool.True {
		t.Fatal("free-variable query", result)
	}
}

func TestFixedpointQueryExpressionExistential(t *testing.T) {
	f := newQueryExpressionFixture(t)
	boolean := f.pin(f.manager.MkBoolSort())
	relation, _ := f.predicate("query-exists-R", boolean)
	truth := f.pin(f.manager.MkTrue())
	queryMust(t, f.engine.AddFact(relation, []ast.ASTHandle{truth}))
	x := f.pin(f.manager.MkVar(0, boolean))
	body := f.pin(f.manager.MkApp(relation, []ast.ASTHandle{x}))
	query := f.pin(f.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	result, err := f.engine.QueryExpression(query)
	queryMust(t, err)
	if result != lbool.True {
		t.Fatal("existential query", result)
	}
}

func TestFixedpointQueryExpressionNestedExistential(t *testing.T) {
	f := newQueryExpressionFixture(t)
	boolean := f.pin(f.manager.MkBoolSort())
	relation, _ := f.predicate("query-nested-exists-R", boolean)
	readyDecl, ready := f.predicate("query-nested-exists-ready")
	_, missing := f.predicate("query-nested-exists-missing")
	truth := f.pin(f.manager.MkTrue())
	queryMust(t, f.engine.AddFact(relation, []ast.ASTHandle{truth}))
	queryMust(t, f.engine.AddFact(readyDecl, nil))
	x := f.pin(f.manager.MkVar(0, boolean))
	body := f.pin(f.manager.MkApp(relation, []ast.ASTHandle{x}))
	exists := f.pin(f.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	for _, example := range []struct {
		guard ast.ASTHandle
		want  lbool.LBool
	}{{ready, lbool.True}, {missing, lbool.False}} {
		query := f.pin(f.manager.MkAndArgs([]ast.ASTHandle{exists, example.guard}))
		result, err := f.engine.QueryExpression(query)
		queryMust(t, err)
		if result != example.want {
			t.Fatalf("nested existential query: got %v, want %v", result, example.want)
		}
	}
}

func TestFixedpointQueryExpressionExistentialWithSparseFreeVariable(t *testing.T) {
	f := newQueryExpressionFixture(t)
	boolean := f.pin(f.manager.MkBoolSort())
	relation, _ := f.predicate("query-exists-sparse-R", boolean, boolean)
	trueRelation, _ := f.predicate("query-exists-sparse-true", boolean)
	falseRelation, _ := f.predicate("query-exists-sparse-false", boolean)
	truth := f.pin(f.manager.MkTrue())
	falsity := f.pin(f.manager.MkFalse())
	queryMust(t, f.engine.AddFact(relation, []ast.ASTHandle{truth, falsity}))
	queryMust(t, f.engine.AddFact(trueRelation, []ast.ASTHandle{truth}))
	queryMust(t, f.engine.AddFact(falseRelation, []ast.ASTHandle{falsity}))
	x := f.pin(f.manager.MkVar(0, boolean))
	y := f.pin(f.manager.MkVar(3, boolean))
	body := f.pin(f.manager.MkApp(relation, []ast.ASTHandle{x, y}))
	for _, example := range []struct {
		predicate ast.ASTHandle
		want      lbool.LBool
	}{{trueRelation, lbool.False}, {falseRelation, lbool.True}} {
		guard := f.pin(f.manager.MkApp(example.predicate, []ast.ASTHandle{y}))
		guarded := f.pin(f.manager.MkAndArgs([]ast.ASTHandle{body, guard}))
		query := f.pin(f.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: guarded}))
		result, err := f.engine.QueryExpression(query)
		queryMust(t, err)
		if result != example.want {
			t.Fatalf("existential and sparse free variable: got %v, want %v", result, example.want)
		}
	}
}

func TestFixedpointQueryExpressionCompactsVariableGaps(t *testing.T) {
	f := newQueryExpressionFixture(t)
	boolean := f.pin(f.manager.MkBoolSort())
	relation, _ := f.predicate("query-gaps-R", boolean, boolean)
	truth := f.pin(f.manager.MkTrue())
	falsity := f.pin(f.manager.MkFalse())
	queryMust(t, f.engine.AddFact(relation, []ast.ASTHandle{truth, falsity}))
	x := f.pin(f.manager.MkVar(2, boolean))
	y := f.pin(f.manager.MkVar(5, boolean))
	query := f.pin(f.manager.MkApp(relation, []ast.ASTHandle{x, y}))
	result, err := f.engine.QueryExpression(query)
	queryMust(t, err)
	if result != lbool.True {
		t.Fatal("gapped query", result)
	}
	repeated := f.pin(f.manager.MkApp(relation, []ast.ASTHandle{x, x}))
	result, err = f.engine.QueryExpression(repeated)
	queryMust(t, err)
	if result != lbool.False {
		t.Fatal("gapped repeated-variable query", result)
	}
}

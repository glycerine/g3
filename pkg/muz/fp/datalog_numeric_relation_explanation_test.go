package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogIntervalRelationExplanationCompilesNumericConstantsAndVariables(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", "interval_relation"))
	must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		must(cause)
		predicate, cause := m.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(predicate))
		return predicate
	}
	p, q := declare("numeric-explanation-p"), declare("numeric-explanation-q")
	number := func(text string) ast.ASTHandle {
		value, cause := m.ArithmeticIntegerNumeralText(text)
		must(cause)
		return value
	}
	app := func(predicate, arg ast.ASTHandle) ast.ASTHandle {
		value, cause := m.MkApp(predicate, []ast.ASTHandle{arg})
		must(cause)
		return value
	}
	x, err := m.MkVar(0, integer)
	must(err)
	must(e.addRuleEager(app(p, number("3")), "numeric-explanation-fact"))
	implication, err := m.MkImplies(app(p, x), app(q, x))
	must(err)
	must(e.addRuleEager(implication, "numeric-explanation-step"))
	cycle, err := m.MkImplies(app(q, x), app(q, x))
	must(err)
	must(e.addRuleEager(cycle, "numeric-explanation-cycle"))

	query := app(q, number("3"))
	status, err := e.QueryExpression(query)
	if err != nil {
		t.Fatalf("query error: %+v", err)
	}
	if status != lbool.True {
		t.Fatal("numeric relation-level explanation query", status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	formula, err := m.RefGet(answer)
	must(err)
	if m.IsFalse(formula) {
		t.Fatal("numeric relation-level query lost its proof formula")
	}
}

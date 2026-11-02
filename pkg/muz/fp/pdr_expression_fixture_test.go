package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type coveredPDRExpressionFixture struct {
	*pdrLifecycleFixture
	declaration, property, query, fact ast.ASTHandle
}

func newCoveredPDRExpressionFixture(t *testing.T) *coveredPDRExpressionFixture {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("expression owner fixture leaked bytes", budget.Used())
		}
	})
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	budget = m.Budget()
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	parameters, err := e.params.NewValue()
	must(err)
	t.Cleanup(func() { must(e.params.Del(&parameters)) })
	for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
		must(e.params.SetBoolName(&parameters, &name, false))
	}
	must(e.SetQueryParameters(&parameters))
	arithmetic, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("failed-expression-integer")
	must(err)
	declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(declaration))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	fact, err := m.MkApp(declaration, []ast.ASTHandle{zero})
	must(err)
	must(e.AddRule(fact, "failed-expression-zero"))
	variable, err := m.MkVar(0, integer)
	must(err)
	property, err := m.MkEq(variable, zero)
	must(err)
	propertyRef, err := m.NewRef(property)
	must(err)
	t.Cleanup(func() { must(m.DelRef(&propertyRef)) })
	query, err := m.MkApp(declaration, []ast.ASTHandle{variable})
	must(err)
	queryRef, err := m.NewRef(query)
	must(err)
	t.Cleanup(func() { must(m.DelRef(&queryRef)) })
	must(e.AddCover(2, declaration, property))
	return &coveredPDRExpressionFixture{f, declaration, property, query, fact}
}

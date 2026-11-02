package fp

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

type spacerGeneralizerExpressionFixture struct {
	*pdrLifecycleFixture
	query, variable, current, integer, zero ast.ASTHandle
}

func newSpacerGeneralizerExpressionFixture(t *testing.T) *spacerGeneralizerExpressionFixture {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("generalizer expression fixture leaked bytes", budget.Used())
		}
	})
	f := newPDRLifecycleFixture(t)
	budget = f.manager.Budget()
	e := f.engine
	fragmentMust(t, e.SelectEngine("spacer"))
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	paramContext := e.params
	parameters, err := paramContext.NewValue()
	must(err)
	t.Cleanup(func() { must(paramContext.Del(&parameters)) })
	for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
		must(e.params.SetBoolName(&parameters, &name, false))
	}
	must(e.SetQueryParameters(&parameters))
	m := f.manager
	arithmetic, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("statistics-integer-chain")
	must(err)
	declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(declaration))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("-1")
	must(err)
	fact, err := m.MkApp(declaration, []ast.ASTHandle{zero})
	must(err)
	must(e.AddRule(fact, "statistics-zero"))
	variable, err := m.MkVar(0, integer)
	must(err)
	current, err := m.MkApp(declaration, []ast.ASTHandle{variable})
	must(err)
	nextValue, err := m.MkArithmeticAdd(variable, one)
	must(err)
	next, err := m.MkApp(declaration, []ast.ASTHandle{nextValue})
	must(err)
	edge, err := m.MkImplies(current, next)
	must(err)
	must(e.AddRule(edge, "statistics-successor"))
	query, err := m.MkApp(declaration, []ast.ASTHandle{three})
	must(err)
	queryRef, err := m.NewRef(query)
	must(err)
	t.Cleanup(func() { must(m.DelRef(&queryRef)) })
	return &spacerGeneralizerExpressionFixture{f, query, variable, current, integer, zero}
}

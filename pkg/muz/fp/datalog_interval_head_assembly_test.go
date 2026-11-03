package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogConfiguredIntervalHeadAssemblyAndUpdates(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", "interval_relation"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, arity int) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		must(cause)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = integer
		}
		declaration, cause := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("configured-interval-assembly-p", 1)
	q := declare("configured-interval-assembly-q", 4)
	number := func(text string) ast.ASTHandle {
		value, cause := m.ArithmeticIntegerNumeralText(text)
		must(cause)
		return value
	}
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, arguments)
		must(cause)
		return result
	}
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	seven := number("7")
	rule, err := m.MkImplies(app(p, x), app(q, seven, x, x, y))
	must(err)
	must(e.AddRule(rule, "interval constant repeated and head-only columns"))
	for _, text := range []string{"2", "4"} {
		must(e.AddRule(app(p, number(text)), "interval seed"))
	}

	query := func() {
		t.Helper()
		status, cause := e.QueryPredicate(q)
		must(cause)
		if status != lbool.Undef {
			t.Fatal("nonempty interval head was not unknown", status)
		}
	}
	contains := func(values ...ast.ASTHandle) bool {
		t.Helper()
		relation, cause := e.intervalDatalog.relation(q)
		must(cause)
		found, cause := relation.ContainsFactExpressions(m, values)
		must(cause)
		return found
	}
	query()
	minusHundred, two, four := number("-100"), number("2"), number("4")
	if !contains(seven, two, four, minusHundred) {
		t.Fatal("source vector copy did not expose the coarse repeated-column hull")
	}
	if contains(number("8"), two, four, minusHundred) {
		t.Fatal("constant head column lost its singleton")
	}
	// A repeated public query rebuilds the evaluator without changing the
	// source vector abstraction.
	query()
	if !contains(seven, four, two, number("100")) {
		t.Fatal("repeated query changed assembled head columns")
	}

	must(e.AddRule(app(p, number("10")), "late interval seed"))
	query()
	if !contains(seven, two, number("10"), number("0")) {
		t.Fatal("late rule update did not flow through assembled interval columns")
	}
}

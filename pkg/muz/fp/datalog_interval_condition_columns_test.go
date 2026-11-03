package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestIntervalDatalogConditionOnlyColumnProjectionAndUpdates(t *testing.T) {
	f := newDirectFactFixture(t)
	m, engine := f.manager, f.engine
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	declare := func(name string, arity int) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern("interval-condition-column-" + name)
		fragmentMust(t, cause)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = integer
		}
		declaration, cause := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("P", 1)
	ordered := declare("Ordered", 2)
	projected := declare("Projected", 1)
	numeral := func(text string) ast.ASTHandle {
		value, cause := m.ArithmeticIntegerNumeralText(text)
		fragmentMust(t, cause)
		return value
	}
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, arguments)
		fragmentMust(t, cause)
		return result
	}
	five := numeral("5")
	fragmentMust(t, engine.AddRule(app(p, five), "interval condition-column fact"))
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := m.MkVar(1, integer)
	fragmentMust(t, err)
	less, err := m.MkArithmeticLT(y, x)
	fragmentMust(t, err)
	body, err := m.MkAnd(app(p, x), less)
	fragmentMust(t, err)
	for _, head := range []ast.ASTHandle{app(ordered, y, x), app(projected, x)} {
		rule, cause := m.MkImplies(body, head)
		fragmentMust(t, cause)
		fragmentMust(t, engine.AddRule(rule, "interval condition-only full column"))
	}
	evaluator, err := NewIntervalDatalogEvaluator(m, engine.set, []ast.ASTHandle{p, ordered, projected}, nil)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, evaluator.Close()) }()

	contains := func(declaration ast.ASTHandle, values ...ast.ASTHandle) bool {
		t.Helper()
		relation, cause := evaluator.Relation(declaration)
		fragmentMust(t, cause)
		found, cause := relation.ContainsFactExpressions(m, values)
		fragmentMust(t, cause)
		return found
	}
	checkInitial := func() {
		t.Helper()
		fragmentMust(t, evaluator.SaturatePositive(false))
		status, cause := evaluator.Query([]ast.ASTHandle{ordered, projected})
		fragmentMust(t, cause)
		if status != lbool.Undef {
			t.Fatal("nonempty condition-column interval result status", status)
		}
		if inside, outside := contains(ordered, numeral("4"), five), contains(ordered, numeral("6"), five); !inside || outside {
			t.Fatal("condition-only interval column did not preserve strict order", inside, outside)
		}
		if !contains(projected, five) {
			t.Fatal("projecting the condition-only column lost the predicate-tail value")
		}
		source, cause := evaluator.Relation(p)
		fragmentMust(t, cause)
		arity, cause := source.Arity()
		fragmentMust(t, cause)
		if arity != 1 || !contains(p, five) {
			t.Fatal("condition-column lowering mutated its interval source", arity)
		}
	}
	checkInitial()
	checkInitial()

	nine := numeral("9")
	fragmentMust(t, engine.AddRule(app(p, nine), "late interval condition-column fact"))
	fragmentMust(t, evaluator.SaturatePositive(false))
	if !contains(ordered, numeral("8"), nine) || contains(ordered, numeral("10"), nine) {
		t.Fatal("late interval update did not receive the condition-only full column")
	}
	if !contains(projected, nine) || !contains(p, nine) {
		t.Fatal("late interval update was lost during condition-column projection")
	}
}

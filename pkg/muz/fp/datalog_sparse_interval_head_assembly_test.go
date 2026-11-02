package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSparseIntervalDatalogFullConditionAndHeadAssembly(t *testing.T) {
	f := newDirectFactFixture(t)
	m, engine := f.manager, f.engine
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	declare := func(text string, sorts ...ast.ASTHandle) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("sparse-interval-assembly-p", boolean, integer)
	ordered := declare("sparse-interval-assembly-ordered", boolean, integer, integer)
	projected := declare("sparse-interval-assembly-projected", boolean, integer)
	constant := declare("sparse-interval-assembly-constant", boolean, integer)
	repeated := declare("sparse-interval-assembly-repeated", boolean, boolean, integer, integer)
	unbound := declare("sparse-interval-assembly-unbound", boolean, integer)
	unboundRepeated := declare("sparse-interval-assembly-unbound-repeated", boolean, boolean, integer, integer)
	declarations := []ast.ASTHandle{p, ordered, projected, constant, repeated, unbound, unboundRepeated}
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, arguments)
		fragmentMust(t, cause)
		return result
	}
	b, err := m.MkVar(0, boolean)
	fragmentMust(t, err)
	x, err := m.MkVar(1, integer)
	fragmentMust(t, err)
	y, err := m.MkVar(2, integer)
	fragmentMust(t, err)
	c, err := m.MkVar(3, boolean)
	fragmentMust(t, err)
	condition, err := m.MkArithmeticLT(y, x)
	fragmentMust(t, err)
	conditionBody, err := m.MkAnd(app(p, b, x), condition)
	fragmentMust(t, err)
	for _, head := range []ast.ASTHandle{app(ordered, b, y, x), app(projected, b, x)} {
		rule, cause := m.MkImplies(conditionBody, head)
		fragmentMust(t, cause)
		fragmentMust(t, engine.AddRule(rule, "mixed condition-only full column"))
	}
	truth, err := m.MkTrue()
	fragmentMust(t, err)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	fragmentMust(t, err)
	rule, err := m.MkImplies(app(p, b, x), app(constant, truth, seven))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "mixed constant head"))
	rule, err = m.MkImplies(app(p, b, x), app(repeated, b, b, x, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "mixed repeated head"))
	rule, err = m.MkImplies(app(p, b, x), app(unbound, c, y))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "mixed unbound head"))
	rule, err = m.MkImplies(app(p, b, x), app(unboundRepeated, c, c, y, y))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "mixed repeated unbound head"))

	store, err := NewSparseIntervalDatalogStore(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, store.Close()) }()
	for _, declaration := range declarations {
		fragmentMust(t, store.Register(declaration))
	}
	falsity, err := m.MkFalse()
	fragmentMust(t, err)
	five, err := m.ArithmeticIntegerNumeralText("5")
	fragmentMust(t, err)
	fragmentMust(t, store.AddFact(p, []ast.ASTHandle{falsity, five}))
	evaluator, err := store.Evaluator(engine.set, engine.guard)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, evaluator.Close()) }()
	fragmentMust(t, evaluator.SaturatePositive(false))
	status, err := evaluator.Query(declarations[1:])
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("nonempty mixed overapproximation was not unknown", status)
	}
	contains := func(declaration ast.ASTHandle, values ...ast.ASTHandle) bool {
		t.Helper()
		result, cause := evaluator.Relation(declaration)
		fragmentMust(t, cause)
		defer func() { fragmentMust(t, result.Close()) }()
		found, cause := result.ContainsFact(values)
		fragmentMust(t, cause)
		return found
	}
	four, err := m.ArithmeticIntegerNumeralText("4")
	fragmentMust(t, err)
	six, err := m.ArithmeticIntegerNumeralText("6")
	fragmentMust(t, err)
	inside, outside := contains(ordered, falsity, four, five), contains(ordered, falsity, six, five)
	if !inside || outside {
		t.Fatal("condition-only arithmetic column did not receive full-domain filtering", inside, outside)
	}
	if !contains(projected, falsity, five) || contains(projected, truth, five) {
		t.Fatal("condition-only projection changed retained finite/interval columns")
	}
	eight, err := m.ArithmeticIntegerNumeralText("8")
	fragmentMust(t, err)
	if !contains(constant, truth, seven) || contains(constant, falsity, seven) || contains(constant, truth, eight) {
		t.Fatal("constant head column was not an exact singleton")
	}
	if !contains(repeated, falsity, falsity, five, five) || contains(repeated, falsity, truth, five, five) || contains(repeated, falsity, falsity, five, six) {
		t.Fatal("repeated mixed head columns lost equality")
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	hundred, err := m.ArithmeticIntegerNumeralText("100")
	fragmentMust(t, err)
	if !contains(unbound, truth, zero) || !contains(unbound, falsity, hundred) {
		t.Fatal("unbound mixed head columns were not full")
	}
	equalTuple := contains(unboundRepeated, truth, truth, zero, zero)
	finiteMismatch := contains(unboundRepeated, truth, falsity, zero, zero)
	intervalMismatch := contains(unboundRepeated, truth, truth, zero, hundred)
	// Source vector_relation::copy publishes representative interval values
	// into fresh target equality classes. Two duplicated full interval columns
	// therefore remain independently full after the first union, while the
	// concrete sparse row still preserves its repeated finite value.
	if !equalTuple || finiteMismatch || !intervalMismatch {
		t.Fatal("repeated unbound mixed head columns diverged from source component copying", equalTuple, finiteMismatch, intervalMismatch)
	}
}

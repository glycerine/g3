package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundDatalogConditionAndHeadOnlyColumnAssembly(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	declare := func(text string, arity int) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = integer
		}
		declaration, cause := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("bound-assembly-p", 1)
	ordered := declare("bound-assembly-ordered", 2)
	projected := declare("bound-assembly-projected", 1)
	repeated := declare("bound-assembly-repeated", 2)
	constant := declare("bound-assembly-constant", 2)
	unbound := declare("bound-assembly-unbound", 2)
	unboundRepeated := declare("bound-assembly-unbound-repeated", 2)
	declarations := []ast.ASTHandle{p, ordered, projected, repeated, constant, unbound, unboundRepeated}
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, arguments)
		fragmentMust(t, cause)
		return result
	}
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := m.MkVar(1, integer)
	fragmentMust(t, err)
	condition, err := m.MkArithmeticLT(y, x)
	fragmentMust(t, err)
	conditionBody, err := m.MkAnd(app(p, x), condition)
	fragmentMust(t, err)
	for _, head := range []ast.ASTHandle{app(ordered, y, x), app(projected, x)} {
		rule, cause := m.MkImplies(conditionBody, head)
		fragmentMust(t, cause)
		fragmentMust(t, engine.AddRule(rule, "condition-only full column"))
	}
	rule, err := m.MkImplies(app(p, x), app(repeated, x, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "repeated bound head"))
	seven, err := m.ArithmeticIntegerNumeralText("7")
	fragmentMust(t, err)
	rule, err = m.MkImplies(app(p, x), app(constant, seven, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "coarse constant head"))
	rule, err = m.MkImplies(app(p, x), app(unbound, y, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "unbound head variable"))
	rule, err = m.MkImplies(app(p, x), app(unboundRepeated, y, y))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "repeated unbound head variable"))

	runtime, err := newBoundDatalogRuntime(m, engine.set, declarations)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, runtime.close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	fragmentMust(t, runtime.addFact(p, []ast.ASTHandle{zero}))
	status, err := runtime.query(declarations[1:])
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("assembled nonempty bound heads were not unknown", status)
	}

	orderedResult, err := runtime.relation(ordered)
	fragmentMust(t, err)
	strict, _, err := orderedResult.Bounds(0)
	fragmentMust(t, err)
	if len(strict) != 1 || strict[0] != 1 {
		t.Fatal("condition-only full column lost its interpreted order", strict)
	}
	projectedResult, err := runtime.relation(projected)
	fragmentMust(t, err)
	strict, weak, err := projectedResult.Bounds(0)
	fragmentMust(t, err)
	if len(strict) != 0 || len(weak) != 0 {
		t.Fatal("condition-only column was not projected from the head", strict, weak)
	}

	assertEquality := func(declaration ast.ASTHandle) {
		t.Helper()
		formula, cause := runtime.relationFormula(declaration)
		fragmentMust(t, cause)
		defer func() { fragmentMust(t, m.DelRef(&formula)) }()
		handle, cause := m.RefGet(formula)
		fragmentMust(t, cause)
		var containsEquality func(ast.ASTHandle) bool
		containsEquality = func(current ast.ASTHandle) bool {
			kind, queryErr := m.GetKind(current)
			if queryErr != nil || kind != ast.ASTApp {
				return false
			}
			op, queryErr := m.GetDeclKind(current)
			if queryErr != nil {
				return false
			}
			if op == ast.ASTOpEq {
				return true
			}
			count, queryErr := m.GetNumArgs(current)
			if queryErr != nil {
				return false
			}
			for i := uint32(0); i < count; i++ {
				argument, argumentErr := m.GetArg(current, i)
				if argumentErr == nil && containsEquality(argument) {
					return true
				}
			}
			return false
		}
		if !containsEquality(handle) {
			t.Fatal("repeated head column was not duplicated with equality")
		}
	}
	assertEquality(repeated)
	assertEquality(unboundRepeated)

	for _, declaration := range []ast.ASTHandle{constant, unbound} {
		result, cause := runtime.relation(declaration)
		fragmentMust(t, cause)
		for column := uint32(0); column < 2; column++ {
			strict, weak, cause = result.Bounds(column)
			fragmentMust(t, cause)
			if len(strict) != 0 || len(weak) != 0 {
				t.Fatal("coarse constant/unbound head unexpectedly narrowed", declaration, column, strict, weak)
			}
		}
	}
}

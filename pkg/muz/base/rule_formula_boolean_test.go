package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Source dl_rule.cpp hoist_compound normalizes the argument of NOT with
// ensure_app before descending. Bit blasting can produce exactly this body.
func TestRuleManagerFormulaNegatedBooleanVariable(t *testing.T) {
	f := newRuleSetFixture(t, "Goal")
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	negative, err := f.manager.MkNot(x)
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(negative, f.app["Goal"])
	ruleMust(t, err)
	owner, err := f.manager.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&owner)) }()
	rule, err := f.rules.NewRuleFromFormula(formula, "boolean-tail")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	if size, err := rule.TailSize(); err != nil || size != 1 {
		t.Fatal("Boolean tail count", size, err)
	}
	if size, err := rule.UninterpretedTailSize(); err != nil || size != 0 {
		t.Fatal("Boolean variable became a predicate", size, err)
	}
	tail, negated, err := rule.Tail(0)
	ruleMust(t, err)
	if operator, err := f.manager.GetDeclKind(tail); err != nil || operator != ast.ASTOpNot || negated {
		t.Fatal("interpreted negation", operator, negated, err)
	}
	argument, err := f.manager.GetArg(tail, 0)
	ruleMust(t, err)
	if operator, err := f.manager.GetDeclKind(argument); err != nil || operator != ast.ASTOpIff {
		t.Fatal("source ensure_app equality", operator, err)
	}
	variable, err := f.manager.GetArg(argument, 0)
	ruleMust(t, err)
	truth, err := f.manager.GetArg(argument, 1)
	ruleMust(t, err)
	if !f.manager.IsTrue(truth) || variable != x {
		t.Fatal("source Boolean application", variable, x, truth)
	}
}

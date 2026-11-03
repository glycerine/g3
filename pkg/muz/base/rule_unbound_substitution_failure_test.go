package base_test

// Source: dl_rule.cpp scopes var_subst in fix_unbound_vars; qe.cpp scopes
// var_shifter in reduce_quantifier. Their Go constructors return partial owners.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleFixUnboundClosesPartialSubstitutionAndRetries(t *testing.T) {
	// These reservations interrupt the enclosing substitution after its
	// header, first cache, and second cache respectively. No caller hook or
	// replacement constructor is used: each invokes the real QE/rule path.
	for _, stop := range []int64{4, 5, 6} {
		t.Run(fmt.Sprintf("reservation-%d", stop), func(t *testing.T) {
			testRuleUnboundPartialRewriterFailure(t, stop)
		})
	}
}

func TestRuleFixUnboundClosesPartialQuantifierShifterAndRetries(t *testing.T) {
	// QE has already produced a result reference when the forward shifter's
	// header succeeds and its first ActivityCache allocation is refused.
	testRuleUnboundPartialRewriterFailure(t, 14)
}

func testRuleUnboundPartialRewriterFailure(t *testing.T, stop int64) {
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("partial rewriter leaked bytes after complete teardown", budget.Used())
		}
	})
	f := newUnboundFixture(t)
	m := f.manager
	budget = m.Budget()
	boolean, err := m.MkBoolSort()
	ruleMust(t, err)
	x, err := m.MkVar(0, boolean)
	ruleMust(t, err)
	y, err := m.MkVar(1, boolean)
	ruleMust(t, err)
	head := f.predicate(t, "failed-substitution-head", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	name, err := m.Symbols.Intern("failed-substitution-opaque")
	ruleMust(t, err)
	declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	opaque, err := m.MkApp(declaration, []ast.ASTHandle{y})
	ruleMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{opaque}, nil, "failed-substitution")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rule.Close()) })
	// Both renamed variables already exist. Warm the reusable QE
	// services so budget differences measure only the failed call.
	for warm := 0; warm < 2; warm++ {
		fixed, _, err := f.rules.FixUnboundVariables(rule, f.lite, true)
		ruleMust(t, err)
		ruleMust(t, fixed.Close())
	}
	before := budget.Used()
	budget.FailAfter(stop)
	fixed, changed, cause := f.rules.FixUnboundVariables(rule, f.lite, true)
	budget.FailAfter(-1)
	if fixed != nil {
		ruleMust(t, fixed.Close())
	}
	if !errors.Is(cause, arena.ErrInjectedFailure) || fixed != nil || changed {
		t.Fatal("expected partial-constructor budget refusal", fixed, changed, cause)
	}
	if budget.Used() != before {
		t.Fatal("failed rewriting call lost its partial owner", before, budget.Used())
	}
	fixed, changed, err = f.rules.FixUnboundVariables(rule, f.lite, true)
	ruleMust(t, err)
	defer func() { ruleMust(t, fixed.Close()) }()
	if !changed {
		t.Fatal("retry failed to bind the residual variable")
	}
	tail, negative, err := fixed.Tail(0)
	ruleMust(t, err)
	operator, err := m.GetDeclKind(tail)
	ruleMust(t, err)
	if negative || operator != ast.ASTOpIff {
		t.Fatal("retry changed the source existential tail", negative, operator)
	}
	quantifier, err := m.GetArg(tail, 0)
	ruleMust(t, err)
	forall, err := m.IsForall(quantifier)
	ruleMust(t, err)
	if forall {
		t.Fatal("retry produced a universal tail")
	}
}

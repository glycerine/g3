package base_test

// Source coverage for dl_rule.cpp variable collection helpers retained by
// rule transforms.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleManagerVariableViewsPreserveIndicesAndSorts(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	defer func() { ruleMust(t, context.Close()) }()
	manager, err := context.Manager()
	ruleMust(t, err)
	rules, err := base.NewRuleManager(manager)
	ruleMust(t, err)
	defer func() { ruleMust(t, rules.Close()) }()
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	x, err := manager.MkVar(0, integer)
	ruleMust(t, err)
	y, err := manager.MkVar(1, integer)
	ruleMust(t, err)
	z, err := manager.MkVar(2, integer)
	ruleMust(t, err)
	declare := func(name string, arity int) ast.ASTHandle {
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = integer
		}
		symbol, makeErr := manager.Symbols.Intern(name)
		ruleMust(t, makeErr)
		declaration, makeErr := manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, makeErr)
		ruleMust(t, rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("variables-P", 1)
	q := declare("variables-Q", 2)
	head, err := manager.MkApp(p, []ast.ASTHandle{x})
	ruleMust(t, err)
	predicateTail, err := manager.MkApp(q, []ast.ASTHandle{x, y})
	ruleMust(t, err)
	equality, err := manager.MkEq(y, z)
	ruleMust(t, err)
	rule, err := rules.NewRule(head, []ast.ASTHandle{predicateTail, equality}, nil, "variables")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	outsidePredicate, err := rules.VariablesOutsideTail(rule, 0)
	ruleMust(t, err)
	if len(outsidePredicate) != 3 || outsidePredicate[0] != integer || outsidePredicate[1] != integer || outsidePredicate[2] != integer {
		t.Fatal("variables outside predicate tail", outsidePredicate)
	}
	outsideEquality, err := rules.VariablesOutsideTail(rule, 1)
	ruleMust(t, err)
	if len(outsideEquality) != 2 || outsideEquality[0] != integer || outsideEquality[1] != integer {
		t.Fatal("variables outside equality tail", outsideEquality)
	}
	tailVariables, err := rules.TailVariables(rule)
	ruleMust(t, err)
	if len(tailVariables) != 3 {
		t.Fatal("tail variables", tailVariables)
	}
	expressionVariables, err := rules.ExpressionVariables(equality)
	ruleMust(t, err)
	if len(expressionVariables) != 2 || expressionVariables[1] != integer || expressionVariables[2] != integer {
		t.Fatal("expression variables", expressionVariables)
	}
	registered, err := rules.IsRegisteredPredicate(q)
	ruleMust(t, err)
	if !registered {
		t.Fatal("registered predicate not recognized")
	}
	symbol, err := manager.Symbols.Intern("variables-unregistered")
	ruleMust(t, err)
	unregistered, err := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	registered, err = rules.IsRegisteredPredicate(unregistered)
	ruleMust(t, err)
	if registered {
		t.Fatal("unregistered predicate recognized")
	}
}

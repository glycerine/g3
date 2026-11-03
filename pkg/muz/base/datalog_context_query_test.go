package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
)

type datalogQueryFixture struct {
	manager   *ast.ASTManager
	context   *base.DatalogContext
	integer   ast.ASTHandle
	predicate ast.ASTHandle
}

func newDatalogQueryFixture(t *testing.T) *datalogQueryFixture {
	t.Helper()
	command, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, command.Close()) })
	manager, err := command.Manager()
	ruleMust(t, err)
	context, err := base.NewDatalogContext(manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	name, err := manager.Symbols.Intern("query-source")
	ruleMust(t, err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, context.RegisterPredicate(predicate, true))
	return &datalogQueryFixture{manager: manager, context: context, integer: integer, predicate: predicate}
}

func TestDatalogContextAddQueryLowersFreeVariableBody(t *testing.T) {
	f := newDatalogQueryFixture(t)
	variable, err := f.manager.MkVar(0, f.integer)
	ruleMust(t, err)
	body, err := f.manager.MkApp(f.predicate, []ast.ASTHandle{variable})
	ruleMust(t, err)
	queryPredicate, err := f.context.AddQuery(body)
	ruleMust(t, err)
	queryName, err := f.manager.GetASTName(queryPredicate)
	ruleMust(t, err)
	registered, found, err := f.context.Predicate(queryName)
	ruleMust(t, err)
	if !found || registered != queryPredicate {
		t.Fatal("fresh query predicate name was not registered", registered, found)
	}
	if arity, err := f.manager.GetArity(queryPredicate); err != nil || arity != 1 {
		t.Fatal("query arity", arity, err)
	}
	rules, err := f.context.Rules()
	ruleMust(t, err)
	if output, err := rules.IsOutputPredicate(queryPredicate); err != nil || !output {
		t.Fatal("query output", output, err)
	}
	queryRules, err := rules.PredicateRules(queryPredicate)
	ruleMust(t, err)
	defer func() { ruleMust(t, closeOwnedRules(queryRules)) }()
	if len(queryRules) != 1 {
		t.Fatal("query rules", len(queryRules))
	}
	if tails, err := queryRules[0].TailSize(); err != nil || tails != 1 {
		t.Fatal("query tail", tails, err)
	}
}

func closeOwnedRules(rules []*base.Rule) error {
	var result error
	for _, rule := range rules {
		if err := rule.Close(); err != nil {
			result = err
		}
	}
	return result
}

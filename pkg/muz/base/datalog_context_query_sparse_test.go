package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDatalogContextAddQueryCompactsSparseVariablesWhenBindingDisabled(t *testing.T) {
	f := newDatalogQueryFixture(t)
	ruleMust(t, f.context.SetBindVariables(false))
	variable, err := f.manager.MkVar(2, f.integer)
	ruleMust(t, err)
	body, err := f.manager.MkApp(f.predicate, []ast.ASTHandle{variable})
	ruleMust(t, err)
	declaration, err := f.context.AddQuery(body)
	ruleMust(t, err)
	if arity, err := f.manager.GetArity(declaration); err != nil || arity != 1 {
		t.Fatal("compacted query arity", arity, err)
	}
	rules, err := f.context.Rules()
	ruleMust(t, err)
	queryRules, err := rules.PredicateRules(declaration)
	ruleMust(t, err)
	defer func() { ruleMust(t, closeOwnedRules(queryRules)) }()
	tail, _, err := queryRules[0].Tail(0)
	ruleMust(t, err)
	argument, err := f.manager.GetArg(tail, 0)
	ruleMust(t, err)
	if index, err := f.manager.GetVarIndex(argument); err != nil || index != 0 {
		t.Fatal("compacted variable", index, err)
	}
}

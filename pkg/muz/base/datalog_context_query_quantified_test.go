package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDatalogContextAddQueryPullsExistentialPrefix(t *testing.T) {
	f := newDatalogQueryFixture(t)
	variable, err := f.manager.MkVar(0, f.integer)
	ruleMust(t, err)
	body, err := f.manager.MkApp(f.predicate, []ast.ASTHandle{variable})
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("quantified")
	ruleMust(t, err)
	query, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{name}, Body: body})
	ruleMust(t, err)
	declaration, err := f.context.AddQuery(query)
	ruleMust(t, err)
	if arity, err := f.manager.GetArity(declaration); err != nil || arity != 1 {
		t.Fatal("existential query arity", arity, err)
	}
}

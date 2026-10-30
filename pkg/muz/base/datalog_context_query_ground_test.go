package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDatalogContextAddQueryGroundBodyCreatesNullaryOutput(t *testing.T) {
	f := newDatalogQueryFixture(t)
	zero, err := f.manager.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	body, err := f.manager.MkApp(f.predicate, []ast.ASTHandle{zero})
	ruleMust(t, err)
	declaration, err := f.context.AddQuery(body)
	ruleMust(t, err)
	if arity, err := f.manager.GetArity(declaration); err != nil || arity != 0 {
		t.Fatal("ground query arity", arity, err)
	}
}

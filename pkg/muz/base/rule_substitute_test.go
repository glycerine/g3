package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleSubstitutionPreservesVariableIndices(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	defer func() { ruleMust(t, context.Close()) }()
	m, err := context.Manager()
	ruleMust(t, err)
	rm, err := base.NewRuleManager(m)
	ruleMust(t, err)
	defer func() { ruleMust(t, rm.Close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	integer, err := m.GetSort(zero)
	ruleMust(t, err)
	boolean, err := m.MkBoolSort()
	ruleMust(t, err)
	name, err := m.Symbols.Intern("substitute-head")
	ruleMust(t, err)
	pred, err := m.MkFuncDecl(name, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, rm.RegisterPredicate(pred))
	x, err := m.MkVar(0, integer)
	ruleMust(t, err)
	y, err := m.MkVar(1, integer)
	ruleMust(t, err)
	head, err := m.MkApp(pred, []ast.ASTHandle{x, y})
	ruleMust(t, err)
	original, err := rm.NewRule(head, nil, nil, "substituted")
	ruleMust(t, err)
	defer func() { ruleMust(t, original.Close()) }()
	result, err := rm.SubstituteRule(original, []ast.ASTHandle{zero, y})
	ruleMust(t, err)
	defer func() { ruleMust(t, result.Close()) }()
	resultHead, err := result.Head()
	ruleMust(t, err)
	first, err := m.GetArg(resultHead, 0)
	ruleMust(t, err)
	second, err := m.GetArg(resultHead, 1)
	ruleMust(t, err)
	if first != zero || second != y {
		t.Fatal("substitution normalized a surviving variable", first, second)
	}
	originalHead, err := original.Head()
	ruleMust(t, err)
	if originalHead != head {
		t.Fatal("substitution changed the original rule")
	}
}

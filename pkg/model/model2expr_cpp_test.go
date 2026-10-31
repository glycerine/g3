package model_test

// Source: src/test/model2expr.cpp tst_model2expr.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func TestModel2ExprCPPFullAndPartialFunctions(t *testing.T) {
	m, md, evaluator, _, _, _ := evaluatorFixture(t)
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	astTestPin(t, m, integer)
	var nums [3]ast.ASTHandle
	for i, value := range []string{"0", "1", "2"} {
		nums[i], err = m.ArithmeticIntegerNumeralText(value)
		astTestMust(t, err)
		astTestPin(t, m, nums[i])
	}
	mkDecl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		d, e := m.MkFuncDecl(astTestSymbol(t, m, name), domain, integer, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestPin(t, m, d)
		return d
	}
	constant := mkDecl("x", nil)
	p := mkDecl("p", []ast.ASTHandle{integer, integer})
	q := mkDecl("q", []ast.ASTHandle{integer, integer})
	astTestMust(t, md.Core().RegisterConstant(constant, nums[0]))
	for _, item := range []struct {
		decl ast.ASTHandle
		full bool
	}{{p, true}, {q, false}} {
		fn := modelTestFunction(t, m, 2)
		astTestMust(t, fn.InsertEntry([]ast.ASTHandle{nums[1], nums[2]}, nums[1]))
		astTestMust(t, fn.InsertEntry([]ast.ASTHandle{nums[0], nums[1]}, nums[2]))
		if item.full {
			astTestMust(t, fn.SetElse(nums[0]))
		}
		astTestMust(t, md.Core().RegisterFunction(item.decl, fn))
	}
	out := astTestPin(t, m, ast.ASTHandle{})
	builder, err := model.NewModel2ExprBuilder(md.Core())
	astTestMust(t, err)
	defer func() { astTestMust(t, builder.Close()) }()
	astTestMust(t, builder.Build(out))
	result, err := m.RefGet(out)
	astTestMust(t, err)
	op, err := m.GetDeclKind(result)
	astTestMust(t, err)
	count, err := m.GetNumArgs(result)
	astTestMust(t, err)
	if op != ast.ASTOpAnd || count != 3 {
		t.Fatal("model conjunction", op, count)
	}
	first, err := m.GetArg(result, 0)
	astTestMust(t, err)
	x, err := m.MkApp(constant, nil)
	astTestMust(t, err)
	want, err := m.MkEq(x, nums[0])
	astTestMust(t, err)
	if first != want {
		t.Fatal("constant interpretation", first, want)
	}
	for i := uint32(1); i < 3; i++ {
		q, err := m.GetArg(result, i)
		astTestMust(t, err)
		kind, err := m.GetKind(q)
		astTestMust(t, err)
		n, err := m.GetNumDeclarations(q)
		astTestMust(t, err)
		if kind != ast.ASTQuantifier || n != 2 {
			t.Fatal("function quantifier", i, kind, n)
		}
		for _, args := range [][]ast.ASTHandle{{nums[1], nums[2]}, {nums[0], nums[1]}, {nums[0], nums[0]}} {
			instance := astTestPin(t, m, ast.ASTHandle{})
			astTestMust(t, rewriter.InstantiateQuantifier(m, q, args, instance))
			formula, err := m.RefGet(instance)
			astTestMust(t, err)
			value := astTestPin(t, m, ast.ASTHandle{})
			astTestMust(t, evaluator.Evaluate(formula, value))
			actual, err := m.RefGet(value)
			astTestMust(t, err)
			if !m.IsTrue(actual) {
				t.Fatal("model does not satisfy function clause", i, args, actual)
			}
		}
	}
}

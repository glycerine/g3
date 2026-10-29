package qe_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
)

func arithmeticEquationConjunction(t *testing.T, manager *ast.ASTManager, formula ast.ASTHandle) *qe.ConjunctionEnum {
	t.Helper()
	conjunctions, err := qe.NewConjunctionEnum(manager, formula)
	pluginMust(t, err)
	t.Cleanup(func() { pluginMust(t, conjunctions.Close()) })
	return conjunctions
}

func takeArithmeticEquation(t *testing.T, f *arithmeticPluginFixture, formula ast.ASTHandle) *qe.ArithmeticEquationElimination {
	t.Helper()
	solved, err := f.plugin.Solve(arithmeticEquationConjunction(t, f.manager, formula), formula)
	pluginMust(t, err)
	if !solved {
		t.Fatal("arithmetic equality was not solved")
	}
	result, err := f.plugin.TakeEquationElimination()
	pluginMust(t, err)
	if result == nil {
		t.Fatal("solved equality did not publish its result")
	}
	t.Cleanup(func() { pluginMust(t, result.Close()) })
	return result
}

func TestArithmeticEquationSingularIntegerAndReal(t *testing.T) {
	t.Run("unit-integer", func(t *testing.T) {
		f := newArithmeticPluginFixture(t, false)
		zero := f.integerNumeral(t, "0")
		left := f.pin(f.manager.MkArithmeticAdd(f.x, f.y))
		formula := f.pin(f.manager.MkEq(left, zero))
		result := takeArithmeticEquation(t, f, formula)
		if result.VariableIndex != 0 || len(result.Fresh) != 0 {
			t.Fatal("integer result metadata", result.VariableIndex, len(result.Fresh))
		}
		rewritten, err := f.manager.RefGet(result.Formula)
		pluginMust(t, err)
		if found, cause := f.contains.Check(rewritten); cause != nil || found {
			t.Fatal("integer result retained eliminated variable", found, cause)
		}
		definition, err := f.manager.RefGet(result.Definition)
		pluginMust(t, err)
		if found, cause := f.contains.Check(definition); cause != nil || found {
			t.Fatal("integer definition retained eliminated variable", found, cause)
		}
	})

	t.Run("nonunit-real", func(t *testing.T) {
		f := newArithmeticPluginFixture(t, true)
		two := f.realNumeral(t, "2")
		zero := f.realNumeral(t, "0")
		twoX := f.pin(f.manager.MkArithmeticMul(two, f.rx))
		left := f.pin(f.manager.MkArithmeticAdd(twoX, f.ry))
		formula := f.pin(f.manager.MkEq(left, zero))
		result := takeArithmeticEquation(t, f, formula)
		if result.VariableIndex != 0 || len(result.Fresh) != 0 {
			t.Fatal("real result metadata", result.VariableIndex, len(result.Fresh))
		}
		rewritten, err := f.manager.RefGet(result.Formula)
		pluginMust(t, err)
		if found, cause := f.contains.Check(rewritten); cause != nil || found {
			t.Fatal("real result retained eliminated variable", found, cause)
		}
		definition, err := f.manager.RefGet(result.Definition)
		pluginMust(t, err)
		if sort, cause := f.manager.GetSort(definition); cause != nil || sort != f.real {
			t.Fatal("real definition sort", sort, cause)
		}
	})
}

func TestArithmeticEquationResultTransferAndCopyGuard(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	zero := f.integerNumeral(t, "0")
	formula := f.pin(f.manager.MkEq(f.x, zero))
	result := takeArithmeticEquation(t, f, formula)
	again, err := f.plugin.TakeEquationElimination()
	pluginMust(t, err)
	if again != nil {
		t.Fatal("equation result ownership transferred more than once")
	}
	copy := *result
	if err = copy.Close(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied owning result accepted", err)
	}
}

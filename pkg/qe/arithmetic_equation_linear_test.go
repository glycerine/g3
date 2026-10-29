package qe_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
)

func TestArithmeticEquationIntegerOmegaStep(t *testing.T) {
	f := newArithmeticQEFixture(t)
	// An unused real context variable must not prevent solving this integer row.
	context, err := qe.NewSolverContext(f.manager, []ast.ASTHandle{f.x, f.y, f.rx})
	pluginMust(t, err)
	t.Cleanup(func() { pluginMust(t, context.Close()) })
	plugin, err := qe.NewArithmeticPlugin(context)
	pluginMust(t, err)
	t.Cleanup(func() { pluginMust(t, plugin.Close()) })

	two := f.integerNumeral(t, "2")
	three := f.integerNumeral(t, "3")
	one := f.integerNumeral(t, "1")
	zero := f.integerNumeral(t, "0")
	twoX := f.pin(f.manager.MkArithmeticMul(two, f.x))
	threeY := f.pin(f.manager.MkArithmeticMul(three, f.y))
	linear := f.pin(f.manager.MkArithmeticAdd(twoX, threeY))
	left := f.pin(f.manager.MkArithmeticAdd(linear, one))
	formula := f.pin(f.manager.MkEq(left, zero))
	conjunctions := arithmeticEquationConjunction(t, f.manager, formula)
	solved, err := plugin.Solve(conjunctions, formula)
	pluginMust(t, err)
	if !solved {
		t.Fatal("integer Omega row was not solved")
	}
	result, err := plugin.TakeEquationElimination()
	pluginMust(t, err)
	if result == nil {
		t.Fatal("integer Omega row did not publish its result")
	}
	t.Cleanup(func() { pluginMust(t, result.Close()) })
	if result.VariableIndex != 0 || len(result.Fresh) != 1 {
		t.Fatal("integer Omega metadata", result.VariableIndex, len(result.Fresh))
	}

	containsX, err := ast.NewContainsApp(f.manager, f.x)
	pluginMust(t, err)
	t.Cleanup(func() { pluginMust(t, containsX.Close()) })
	rewritten, err := f.manager.RefGet(result.Formula)
	pluginMust(t, err)
	if found, cause := containsX.Check(rewritten); cause != nil || found {
		t.Fatal("Omega result retained eliminated variable", found, cause)
	}
	definition, err := f.manager.RefGet(result.Definition)
	pluginMust(t, err)
	if found, cause := containsX.Check(definition); cause != nil || found {
		t.Fatal("Omega definition retained eliminated variable", found, cause)
	}
	fresh, err := f.manager.RefGet(result.Fresh[0])
	pluginMust(t, err)
	containsFresh, err := ast.NewContainsApp(f.manager, fresh)
	pluginMust(t, err)
	t.Cleanup(func() { pluginMust(t, containsFresh.Close()) })
	if found, cause := containsFresh.Check(definition); cause != nil || !found {
		t.Fatal("Omega definition lost auxiliary integer", found, cause)
	}
}

func TestArithmeticEquationIntegerUnitAvoidsAuxiliary(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	zero := f.integerNumeral(t, "0")
	formula := f.pin(f.manager.MkEq(f.x, zero))
	result := takeArithmeticEquation(t, f, formula)
	if len(result.Fresh) != 0 {
		t.Fatal("unit integer equation introduced an auxiliary")
	}
}

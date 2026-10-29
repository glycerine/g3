package qe_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticPluginFiniteAssignSubstituteAndDefinition(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	zero := f.integerNumeral(t, "0")
	minusOne := f.integerNumeral(t, "-1")
	minusFive := f.integerNumeral(t, "-5")
	minusSeven := f.integerNumeral(t, "-7")
	lowerPoly := f.pin(f.manager.MkArithmeticMul(minusOne, f.x))
	upperPoly1 := f.pin(f.manager.MkArithmeticAdd(f.x, minusFive))
	upperPoly2 := f.pin(f.manager.MkArithmeticAdd(f.x, minusSeven))
	lower := f.pin(f.manager.MkArithmeticLE(lowerPoly, zero))
	u1 := f.pin(f.manager.MkArithmeticLE(upperPoly1, zero))
	u2 := f.pin(f.manager.MkArithmeticLE(upperPoly2, zero))
	pair := f.pin(f.manager.MkAnd(lower, u1))
	formula := f.pin(f.manager.MkAnd(pair, u2))
	ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{lower, u1, u2}, nil)
	pluginMust(t, err)
	if !ok {
		t.Fatal("prepare")
	}
	service := f.manager.NumeralService()
	one, err := service.NewValue()
	pluginMust(t, err)
	pluginMust(t, service.SetUnsigned(&one, 1))
	defer func() { pluginMust(t, service.Del(&one)) }()
	pluginMust(t, f.plugin.Assign(f.contains, formula, &one))
	result, err := f.manager.NewRef(formula)
	pluginMust(t, err)
	defer func() { pluginMust(t, f.manager.DelRef(&result)) }()
	definition, err := f.manager.NewRef(ast.ASTHandle{})
	pluginMust(t, err)
	defer func() { pluginMust(t, f.manager.DelRef(&definition)) }()
	pluginMust(t, f.plugin.Substitute(f.contains, &one, result, &definition))
	h, err := f.manager.RefGet(result)
	pluginMust(t, err)
	if found, err := f.contains.Check(h); err != nil || found {
		t.Fatal("branch result retained variable", found, err)
	}
	d, err := f.manager.RefGet(definition)
	pluginMust(t, err)
	if d == (ast.ASTHandle{}) {
		t.Fatal("equality branch omitted definition")
	}
	constraints, err := f.plugin.TakeConstraints()
	pluginMust(t, err)
	if len(constraints) < 3 {
		t.Fatal("missing path constraints", len(constraints))
	}
	for i := range constraints {
		pluginMust(t, f.manager.DelRef(&constraints[i].Formula))
	}
}

func TestArithmeticPluginInfinityBranchEliminatesAllBounds(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	zero := f.integerNumeral(t, "0")
	minusOne := f.integerNumeral(t, "-1")
	minusFive := f.integerNumeral(t, "-5")
	lowerPoly := f.pin(f.manager.MkArithmeticMul(minusOne, f.x))
	upperPoly := f.pin(f.manager.MkArithmeticAdd(f.x, minusFive))
	lower := f.pin(f.manager.MkArithmeticLE(lowerPoly, zero))
	upper := f.pin(f.manager.MkArithmeticLE(upperPoly, zero))
	formula := f.pin(f.manager.MkAnd(lower, upper))
	ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{lower, upper}, nil)
	pluginMust(t, err)
	if !ok {
		t.Fatal("prepare")
	}
	service := f.manager.NumeralService()
	branch, err := service.NewValue()
	pluginMust(t, err)
	pluginMust(t, service.SetUnsigned(&branch, 0))
	defer func() { pluginMust(t, service.Del(&branch)) }()
	pluginMust(t, f.plugin.Assign(f.contains, formula, &branch))
	result, err := f.manager.NewRef(formula)
	pluginMust(t, err)
	defer func() { pluginMust(t, f.manager.DelRef(&result)) }()
	pluginMust(t, f.plugin.Substitute(f.contains, &branch, result, nil))
	h, err := f.manager.RefGet(result)
	pluginMust(t, err)
	if found, err := f.contains.Check(h); err != nil || found {
		t.Fatal("infinity retained x", found, err)
	}
}

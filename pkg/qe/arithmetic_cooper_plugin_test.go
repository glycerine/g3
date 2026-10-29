package qe_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func assignArithmeticInfinity(t *testing.T, f *arithmeticPluginFixture, formula ast.ASTHandle) ast.ASTRef {
	t.Helper()
	service := f.manager.NumeralService()
	branch, err := service.NewValue()
	pluginMust(t, err)
	pluginMust(t, service.SetUnsigned(&branch, 0))
	defer func() { pluginMust(t, service.Del(&branch)) }()
	pluginMust(t, f.plugin.Assign(f.contains, formula, &branch))
	result, err := f.manager.NewRef(formula)
	pluginMust(t, err)
	pluginMust(t, f.plugin.Substitute(f.contains, &branch, result, nil))
	return result
}

func releaseArithmeticCooperOutputs(t *testing.T, f *arithmeticPluginFixture) ([]ast.ASTHandle, int) {
	t.Helper()
	fresh, err := f.plugin.TakeFreshVariables()
	pluginMust(t, err)
	handles := make([]ast.ASTHandle, len(fresh))
	for i := range fresh {
		handles[i], err = f.manager.RefGet(fresh[i])
		pluginMust(t, err)
		pluginMust(t, f.manager.DelRef(&fresh[i]))
	}
	constraints, err := f.plugin.TakeConstraints()
	pluginMust(t, err)
	for i := range constraints {
		pluginMust(t, f.manager.DelRef(&constraints[i].Formula))
	}
	return handles, len(constraints)
}

func TestArithmeticCooperDirectAndNegatedDivisibility(t *testing.T) {
	for _, negated := range []bool{false, true} {
		t.Run(map[bool]string{false: "positive", true: "negated"}[negated], func(t *testing.T) {
			f := newArithmeticPluginFixture(t, false)
			two := f.integerNumeral(t, "2")
			seven := f.integerNumeral(t, "7")
			twoX := f.pin(f.manager.MkArithmeticMul(two, f.x))
			polynomial := f.pin(f.manager.MkArithmeticAdd(twoX, f.y))
			modular := arithmeticQEMod(t, f.arithmeticQEFixture, polynomial, seven)
			zero := f.integerNumeral(t, "0")
			atom := f.pin(f.manager.MkEq(zero, modular))
			formula := atom
			positive := []ast.ASTHandle{atom}
			negative := []ast.ASTHandle(nil)
			if negated {
				formula = f.pin(f.manager.MkNot(atom))
				positive = nil
				negative = []ast.ASTHandle{atom}
			}
			ok, err := f.plugin.Prepare(f.contains, formula, positive, negative)
			pluginMust(t, err)
			if !ok {
				t.Fatal("divisibility rejected")
			}
			result := assignArithmeticInfinity(t, f, formula)
			defer func() { pluginMust(t, f.manager.DelRef(&result)) }()
			h, err := f.manager.RefGet(result)
			pluginMust(t, err)
			if found, cause := f.contains.Check(h); cause != nil || found {
				t.Fatal("Cooper result retained eliminated variable", found, cause)
			}
			fresh, constraints := releaseArithmeticCooperOutputs(t, f)
			if len(fresh) != 1 || constraints != 4 {
				t.Fatal("Cooper outputs", len(fresh), constraints)
			}
			again, err := f.plugin.TakeFreshVariables()
			pluginMust(t, err)
			if len(again) != 0 {
				t.Fatal("fresh ownership was not transferred")
			}
		})
	}
}

func TestArithmeticCooperNestedModuloRoutesResidues(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	two := f.integerNumeral(t, "2")
	five := f.integerNumeral(t, "5")
	one := f.integerNumeral(t, "1")
	twoX := f.pin(f.manager.MkArithmeticMul(two, f.x))
	polynomial := f.pin(f.manager.MkArithmeticAdd(twoX, f.y))
	modular := arithmeticQEMod(t, f.arithmeticQEFixture, polynomial, five)
	formula := f.pin(f.manager.MkEq(modular, one))
	ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{formula}, nil)
	pluginMust(t, err)
	if !ok {
		t.Fatal("nested modulo rejected")
	}
	result := assignArithmeticInfinity(t, f, formula)
	defer func() { pluginMust(t, f.manager.DelRef(&result)) }()
	h, err := f.manager.RefGet(result)
	pluginMust(t, err)
	if found, cause := f.contains.Check(h); cause != nil || found {
		t.Fatal("nested Cooper result retained eliminated variable", found, cause)
	}
	fresh, constraints := releaseArithmeticCooperOutputs(t, f)
	if len(fresh) != 2 || constraints != 4 {
		t.Fatal("nested Cooper outputs", len(fresh), constraints)
	}
}

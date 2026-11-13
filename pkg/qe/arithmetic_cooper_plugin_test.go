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

func releaseArithmeticCooperOutputs(t *testing.T, f *arithmeticPluginFixture) ([]uint32, int) {
	t.Helper()
	fresh, err := f.plugin.TakeFreshVariables()
	pluginMust(t, err)
	widths := make([]uint32, len(fresh))
	for i := range fresh {
		handle, err := f.manager.RefGet(fresh[i])
		pluginMust(t, err)
		widths[i], err = f.manager.BitVectorSize(handle)
		pluginMust(t, err)
		pluginMust(t, f.manager.DelRef(&fresh[i]))
	}
	constraints, err := f.plugin.TakeConstraints()
	pluginMust(t, err)
	for i := range constraints {
		pluginMust(t, f.manager.DelRef(&constraints[i].Formula))
	}
	return widths, len(constraints)
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
			// assign publishes z_bv; subst's get_cache publishes it again.
			fresh, constraints := releaseArithmeticCooperOutputs(t, f)
			if len(fresh) != 2 || constraints != 4 {
				t.Fatal("Cooper outputs", len(fresh), constraints)
			}
			if fresh[0] != 3 || fresh[1] != 3 {
				t.Fatal("residue bit-vector widths", fresh)
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
	// assign publishes z1_bv and z_bv; subst's get_cache publishes both again.
	fresh, constraints := releaseArithmeticCooperOutputs(t, f)
	if len(fresh) != 4 || constraints != 4 {
		t.Fatal("nested Cooper outputs", len(fresh), constraints)
	}
	for _, width := range fresh {
		if width != 3 {
			t.Fatal("nested residue bit-vector widths", fresh)
		}
	}
}

func TestArithmeticCooperCachedBranchReusesFreshVariables(t *testing.T) {
	for _, nested := range []bool{false, true} {
		name := "direct"
		if nested {
			name = "nested"
		}
		t.Run(name, func(t *testing.T) {
			f := newArithmeticPluginFixture(t, false)
			two := f.integerNumeral(t, "2")
			five := f.integerNumeral(t, "5")
			term := f.pin(f.manager.MkArithmeticAdd(f.pin(f.manager.MkArithmeticMul(two, f.x)), f.y))
			mod := arithmeticQEMod(t, f.arithmeticQEFixture, term, five)
			other := f.integerNumeral(t, "0")
			if nested {
				other = f.integerNumeral(t, "1")
			}
			formula := f.pin(f.manager.MkEq(mod, other))
			ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{formula}, nil)
			pluginMust(t, err)
			if !ok {
				t.Fatal("Cooper formula rejected")
			}
			first := assignArithmeticInfinity(t, f, formula)
			defer func() { pluginMust(t, f.manager.DelRef(&first)) }()
			before, err := f.manager.RefGet(first)
			pluginMust(t, err)
			owners, err := f.plugin.TakeFreshVariables()
			pluginMust(t, err)
			if len(owners) == 0 {
				t.Fatal("missing fresh variables")
			}
			variables := make([]ast.ASTHandle, len(owners))
			for i := range owners {
				variables[i], err = f.manager.RefGet(owners[i])
				pluginMust(t, err)
				pluginMust(t, f.manager.DelRef(&owners[i]))
			}
			constraints, err := f.plugin.TakeConstraints()
			pluginMust(t, err)
			if len(constraints) == 0 {
				t.Fatal("first assignment emitted no constraints")
			}
			for i := range constraints {
				pluginMust(t, f.manager.DelRef(&constraints[i].Formula))
			}
			second := assignArithmeticInfinity(t, f, formula)
			defer func() { pluginMust(t, f.manager.DelRef(&second)) }()
			after, err := f.manager.RefGet(second)
			pluginMust(t, err)
			if before != after {
				t.Fatal("cached branch formula was reconstructed")
			}
			owners, err = f.plugin.TakeFreshVariables()
			pluginMust(t, err)
			if len(owners) != len(variables) {
				t.Fatal("cached fresh variable count", len(owners))
			}
			for i := range owners {
				h, e := f.manager.RefGet(owners[i])
				pluginMust(t, e)
				if h != variables[i] {
					t.Fatal("cached assignment changed a fresh variable")
				}
				pluginMust(t, f.manager.DelRef(&owners[i]))
			}
			constraints, err = f.plugin.TakeConstraints()
			pluginMust(t, err)
			if len(constraints) != 0 {
				t.Fatal("cached assignment re-emitted constraints")
			}
		})
	}
}

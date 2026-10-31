package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/symbol"
	"math"
	"testing"
)

func TestNLArithmeticExtractNonlinearTraversalAndWeightCache(t *testing.T) {
	f := newArithmeticQEFixture(t)
	zero := f.realNumeral(t, "0")
	product := f.pin(f.manager.MkArithmeticMul(f.rx, f.ry))
	formula := f.pin(f.manager.MkArithmeticLE(product, zero))
	u, err := qe.NewNonlinearArithmeticUtil(f.manager)
	pluginMust(t, err)
	variables, err := u.ExtractNonlinear(formula)
	pluginMust(t, err)
	if len(variables) != 2 || variables[0] != f.ry || variables[1] != f.rx {
		t.Fatal("source LIFO discovery order", variables)
	}
	pluginMust(t, u.Close())
	cache, err := qe.NewNLWeightCache(f.manager, false)
	pluginMust(t, err)
	defer func() { pluginMust(t, cache.Close()) }()
	if w, err := cache.Weight(f.rx, formula); err != nil || w != 100 {
		t.Fatal("nonlinear weight", w, err)
	}
	linear := f.pin(f.manager.MkArithmeticAdd(f.rx, f.ry))
	if w, err := cache.Weight(f.rx, linear); err != nil || w != math.MaxUint32 {
		t.Fatal("linear fallback", w, err)
	}
}

func TestNLArithmeticEnableLinearAndNonArithmeticPropagation(t *testing.T) {
	f := newArithmeticQEFixture(t)
	u, err := qe.NewNonlinearArithmeticUtil(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, u.Close()) }()
	pluginMust(t, u.SetEnableLinear(true))
	linear := f.pin(f.manager.MkArithmeticAdd(f.rx, f.ry))
	variables, err := u.ExtractNonlinear(linear)
	pluginMust(t, err)
	if len(variables) != 2 {
		t.Fatal("enabled linear variables", len(variables))
	}
}

func TestNLArithmeticExtractTraversesQuantifierBody(t *testing.T) {
	f := newArithmeticQEFixture(t)
	zero := f.realNumeral(t, "0")
	product := f.pin(f.manager.MkArithmeticMul(f.rx, f.ry))
	body := f.pin(f.manager.MkArithmeticLE(product, zero))
	quantified := f.pin(f.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body,
	}))
	u, err := qe.NewNonlinearArithmeticUtil(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, u.Close()) }()
	variables, err := u.ExtractNonlinear(quantified)
	pluginMust(t, err)
	if len(variables) != 2 || variables[0] != f.ry || variables[1] != f.rx {
		t.Fatal("nonlinear variables inside quantifier", variables)
	}
}

package qe_test

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
)

type arithmeticPluginFixture struct {
	*arithmeticQEFixture
	context  *qe.SolverContext
	plugin   *qe.ArithmeticPlugin
	contains *ast.ContainsApp
}

func newArithmeticPluginFixture(t *testing.T, real bool) *arithmeticPluginFixture {
	f := &arithmeticPluginFixture{arithmeticQEFixture: newArithmeticQEFixture(t)}
	target := f.x
	if real {
		target = f.rx
	}
	contains, err := ast.NewContainsApp(f.manager, target)
	pluginMust(t, err)
	f.contains = contains
	context, err := qe.NewSolverContext(f.manager, []ast.ASTHandle{target})
	pluginMust(t, err)
	f.context = context
	plugin, err := qe.NewArithmeticPlugin(context)
	pluginMust(t, err)
	f.plugin = plugin
	t.Cleanup(func() { pluginMust(t, plugin.Close()); pluginMust(t, context.Close()); pluginMust(t, contains.Close()) })
	return f
}

func (f *arithmeticPluginFixture) numeral(t *testing.T, value uint32) *big.Rat {
	t.Helper()
	return new(big.Rat).SetInt64(int64(value))
}

func TestArithmeticPluginIntegerBranchPolicyAndSourceIndexing(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	zero := f.integerNumeral(t, "0")
	minusOne := f.integerNumeral(t, "-1")
	minusFive := f.integerNumeral(t, "-5")
	minusSeven := f.integerNumeral(t, "-7")
	lowerPoly := f.pin(f.manager.MkArithmeticMul(minusOne, f.x))
	upperPoly1 := f.pin(f.manager.MkArithmeticAdd(f.x, minusFive))
	upperPoly2 := f.pin(f.manager.MkArithmeticAdd(f.x, minusSeven))
	lower := f.pin(f.manager.MkArithmeticLE(lowerPoly, zero))
	upper1 := f.pin(f.manager.MkArithmeticLE(upperPoly1, zero))
	upper2 := f.pin(f.manager.MkArithmeticLE(upperPoly2, zero))
	formula := f.pin(f.manager.MkAnd(lower, upper1))
	ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{lower, upper1, upper2}, nil)
	pluginMust(t, err)
	if !ok {
		t.Fatal("bounds rejected")
	}
	policy, err := f.plugin.BranchPolicy(f.contains, formula)
	pluginMust(t, err)
	if !policy.Lower || policy.NonStrictCount != 1 || policy.StrictCount != 0 || policy.Real {
		t.Fatalf("policy: %+v", policy)
	}
	inf, err := f.plugin.SelectBranch(f.contains, formula, f.numeral(t, 0))
	pluginMust(t, err)
	if inf.Kind != qe.ArithmeticInfinityBranch || !inf.Lower {
		t.Fatalf("infinity: %+v", inf)
	}
	finite, err := f.plugin.SelectBranch(f.contains, formula, f.numeral(t, 1))
	pluginMust(t, err)
	if finite.Kind != qe.ArithmeticEqualityBranch || finite.Index != 0 || finite.Bound == nil {
		t.Fatalf("finite: %+v", finite)
	}
}

func TestArithmeticPluginRealNonStrictBranchesAreDoubled(t *testing.T) {
	f := newArithmeticPluginFixture(t, true)
	zero := f.realNumeral(t, "0")
	minusOne := f.realNumeral(t, "-1")
	minusFive := f.realNumeral(t, "-5")
	minusSeven := f.realNumeral(t, "-7")
	lowerPoly := f.pin(f.manager.MkArithmeticMul(minusOne, f.rx))
	upperPoly1 := f.pin(f.manager.MkArithmeticAdd(f.rx, minusFive))
	upperPoly2 := f.pin(f.manager.MkArithmeticAdd(f.rx, minusSeven))
	lower := f.pin(f.manager.MkArithmeticLE(lowerPoly, zero))
	u1 := f.pin(f.manager.MkArithmeticLE(upperPoly1, zero))
	u2 := f.pin(f.manager.MkArithmeticLE(upperPoly2, zero))
	formula := f.pin(f.manager.MkAnd(lower, u1))
	ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{lower, u1, u2}, nil)
	pluginMust(t, err)
	if !ok {
		t.Fatal("real bounds rejected")
	}
	p, err := f.plugin.BranchPolicy(f.contains, formula)
	pluginMust(t, err)
	if !p.Real || !p.Lower || p.NonStrictCount != 1 {
		t.Fatalf("policy: %+v", p)
	}
	eq, err := f.plugin.SelectBranch(f.contains, formula, f.numeral(t, 1))
	pluginMust(t, err)
	open, err := f.plugin.SelectBranch(f.contains, formula, f.numeral(t, 2))
	pluginMust(t, err)
	if eq.Kind != qe.ArithmeticEqualityBranch || open.Kind != qe.ArithmeticOpenBranch {
		t.Fatal("real pair ordering", eq.Kind, open.Kind)
	}
}

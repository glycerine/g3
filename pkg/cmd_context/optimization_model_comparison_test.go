package cmd_context

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func optimizationComparisonValue(t *testing.T, c *Context, md *model.ModelCore, ref ast.ASTRef) bool {
	t.Helper()
	defer func() {
		if err := c.manager.DelRef(&ref); err != nil {
			t.Error(err)
		}
	}()
	h, err := c.manager.RefGet(ref)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := model.NewModelEvaluator(md, c.config.ModelEvaluator)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = evaluator.Close() }()
	value, err := evaluator.EvaluateValue(h)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.manager.DelRef(&value) }()
	resolved, err := c.manager.RefGet(value)
	if err != nil {
		t.Fatal(err)
	}
	return c.manager.IsTrue(resolved)
}

func optimizationComparisonInteger(t *testing.T, c *Context, name string) ast.ASTHandle {
	t.Helper()
	manager, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	arith, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	sort, err := manager.MkFamilySort(arith, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	symbol, err := manager.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	value, err := manager.MkConst(symbol, sort)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestOptimizationArithmeticModelComparisons(t *testing.T) {
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "comparison-x")
	zero, err := c.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	contextMust(t, err)
	five, err := c.manager.ArithmeticNumeralRat(big.NewRat(5, 1), true)
	contextMust(t, err)
	lower, err := c.manager.MkArithmeticGE(x, zero)
	contextMust(t, err)
	upper, err := c.manager.MkArithmeticLE(x, five)
	contextMust(t, err)
	bounded, err := c.manager.MkAnd(lower, upper)
	contextMust(t, err)
	contextMust(t, c.Assert(bounded, ""))
	contextMust(t, c.AddObjective(x, true))
	status, err := c.CheckSat(nil)
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	md, err := c.Model()
	contextMust(t, err)
	defer func() { _ = md.DecRef() }()
	for _, row := range []struct {
		make func(uint32, *model.ModelCore) (ast.ASTRef, error)
		want bool
	}{{c.OptimizationModelGE, true}, {c.OptimizationModelLE, true}, {c.OptimizationModelGT, false}} {
		ref, err := row.make(0, md)
		contextMust(t, err)
		if got := optimizationComparisonValue(t, c, md, ref); got != row.want {
			t.Fatal(got, row.want)
		}
	}
}

func TestOptimizationSoftModelComparisons(t *testing.T) {
	c := contextFixture(t)
	manager, err := c.Manager()
	contextMust(t, err)
	boolean, err := manager.MkBoolSort()
	contextMust(t, err)
	name, err := manager.Symbols.Intern("comparison-p")
	contextMust(t, err)
	p, err := manager.MkConst(name, boolean)
	contextMust(t, err)
	notP, err := manager.MkNot(p)
	contextMust(t, err)
	contextMust(t, c.AddSoftConstraint(p, big.NewRat(3, 1), "pref"))
	contextMust(t, c.AddSoftConstraint(notP, big.NewRat(1, 1), "pref"))
	status, err := c.CheckSat(nil)
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	md, err := c.Model()
	contextMust(t, err)
	defer func() { _ = md.DecRef() }()
	ge, err := c.OptimizationModelGE(0, md)
	contextMust(t, err)
	if !optimizationComparisonValue(t, c, md, ge) {
		t.Fatal("source model did not satisfy its soft lower bound")
	}
	gt, err := c.OptimizationModelGT(0, md)
	contextMust(t, err)
	if optimizationComparisonValue(t, c, md, gt) {
		t.Fatal("strict soft improvement accepted source model")
	}
}

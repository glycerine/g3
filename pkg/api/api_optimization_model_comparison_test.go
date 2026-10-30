package api

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func apiComparisonInteger(t *testing.T, c *SMTContext, name string) ast.ASTHandle {
	t.Helper()
	manager, err := c.Manager()
	astTestMust(t, err)
	arith, err := manager.RegisterArithmeticPlugin()
	astTestMust(t, err)
	sort, err := manager.MkFamilySort(arith, 1, nil)
	astTestMust(t, err)
	symbol, err := manager.Symbols.Intern(name)
	astTestMust(t, err)
	value, err := manager.MkConst(symbol, sort)
	astTestMust(t, err)
	return value
}

func apiComparisonBounds(t *testing.T, c *SMTContext, x ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	manager, err := c.Manager()
	astTestMust(t, err)
	zero, err := manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	astTestMust(t, err)
	five, err := manager.ArithmeticNumeralRat(big.NewRat(5, 1), true)
	astTestMust(t, err)
	lower, err := manager.MkArithmeticGE(x, zero)
	astTestMust(t, err)
	upper, err := manager.MkArithmeticLE(x, five)
	astTestMust(t, err)
	result, err := manager.MkAnd(lower, upper)
	astTestMust(t, err)
	return result
}

func apiComparisonTruth(t *testing.T, md *APIModel, ref ast.ASTRef) bool {
	t.Helper()
	manager, err := md.Manager()
	astTestMust(t, err)
	defer func() { astTestMust(t, manager.DelRef(&ref)) }()
	h, err := manager.RefGet(ref)
	astTestMust(t, err)
	value, err := md.Evaluate(h, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, manager.DelRef(&value)) }()
	resolved, err := manager.RefGet(value)
	astTestMust(t, err)
	return manager.IsTrue(resolved)
}

func TestSMTContextOptimizationModelComparisons(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	x := apiComparisonInteger(t, context, "api-comparison-x")
	astTestMust(t, context.Assert(apiComparisonBounds(t, context, x)))
	astTestMust(t, context.Maximize(x))
	status, err := context.CheckSat()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	md, err := context.CurrentAPIModel()
	astTestMust(t, err)
	defer func() { astTestMust(t, md.Close()) }()
	ge, err := context.OptimizationModelGE(0, md)
	astTestMust(t, err)
	if !apiComparisonTruth(t, md, ge) {
		t.Fatal("model failed non-strict comparison")
	}
	gt, err := context.OptimizationModelGT(0, md)
	astTestMust(t, err)
	if apiComparisonTruth(t, md, gt) {
		t.Fatal("model satisfied strict improvement")
	}
}

func TestAPIOptimizeModelComparisonsAndOwnership(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	x := apiComparisonInteger(t, context, "api-optimize-comparison-x")
	optimizer, err := context.NewOptimize()
	astTestMust(t, err)
	defer func() { astTestMust(t, optimizer.Close()) }()
	astTestMust(t, optimizer.Assert(apiComparisonBounds(t, context, x)))
	_, err = optimizer.Maximize(x)
	astTestMust(t, err)
	status, err := optimizer.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	md, err := optimizer.Model()
	astTestMust(t, err)
	defer func() { astTestMust(t, md.Close()) }()
	le, err := optimizer.OptimizationModelLE(0, md)
	astTestMust(t, err)
	if !apiComparisonTruth(t, md, le) {
		t.Fatal("model failed reverse non-strict comparison")
	}
	gt, err := optimizer.OptimizationModelGT(0, md)
	astTestMust(t, err)
	if apiComparisonTruth(t, md, gt) {
		t.Fatal("model satisfied strict improvement")
	}
}

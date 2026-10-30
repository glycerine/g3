package cmd_context

import (
	"fmt"
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

// opt_context.cpp mk_cmp calls model::eval with its default completion=false.
// Missing or symbolic numerical values make GE/LE true; mk_gt uses mk_not,
// yielding the actual false node rather than a raw (not true) application.
func TestOptimizationModelComparisonsPreserveIncompleteModels(t *testing.T) {
	for _, domain := range []string{"int", "real", "bv"} {
		for _, maximize := range []bool{false, true} {
			for _, symbolic := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/max=%v/symbolic=%v", domain, maximize, symbolic), func(t *testing.T) {
					c := contextFixture(t)
					m, err := c.Manager()
					contextMust(t, err)
					var sort ast.ASTHandle
					if domain == "bv" {
						_, err = m.RegisterBitVectorPlugin()
						contextMust(t, err)
						sort, err = m.BitVectorSort(65)
					} else {
						fid, e := m.RegisterArithmeticPlugin()
						contextMust(t, e)
						kind := int32(1)
						if domain == "real" {
							kind = 0
						}
						sort, err = m.MkFamilySort(fid, kind, nil)
					}
					contextMust(t, err)
					name, err := m.Symbols.Intern("incomplete-objective")
					contextMust(t, err)
					x, err := m.MkConst(name, sort)
					contextMust(t, err)
					contextMust(t, c.AddObjective(x, maximize))
					owner, err := model.NewModel(m)
					contextMust(t, err)
					md := owner.Core()
					t.Cleanup(func() { contextMust(t, owner.Close()) })
					if symbolic {
						symbol, err := m.Symbols.Intern("incomplete-symbolic-value")
						contextMust(t, err)
						y, err := m.MkConst(symbol, sort)
						contextMust(t, err)
						d, err := m.GetDecl(x)
						contextMust(t, err)
						contextMust(t, md.RegisterConstant(d, y))
					}
					before, err := md.NumConstants()
					contextMust(t, err)
					for _, cmp := range []OptimizationModelComparison{OptimizationModelGE, OptimizationModelLE, OptimizationModelGT} {
						ref, err := c.OptimizationCompareModel(0, md, cmp)
						contextMust(t, err)
						got, err := m.RefGet(ref)
						contextMust(t, err)
						want := m.IsTrue(got)
						if cmp == OptimizationModelGT {
							want = m.IsFalse(got)
						}
						contextMust(t, m.DelRef(&ref))
						if !want {
							t.Fatalf("comparison %d did not use source neutral result", cmp)
						}
						after, err := md.NumConstants()
						contextMust(t, err)
						if after != before {
							t.Fatal("comparison completed the caller's model", before, after)
						}
					}
				})
			}
		}
	}
}

func TestOptimizationSoftModelComparisonDoesNotCompleteNegatedTerm(t *testing.T) {
	for _, cmp := range []OptimizationModelComparison{OptimizationModelGE, OptimizationModelLE, OptimizationModelGT} {
		t.Run(fmt.Sprint(cmp), func(t *testing.T) {
			c := contextFixture(t)
			m, err := c.Manager()
			contextMust(t, err)
			boolean, err := m.MkBoolSort()
			contextMust(t, err)
			name, err := m.Symbols.Intern("incomplete-soft-p")
			contextMust(t, err)
			p, err := m.MkConst(name, boolean)
			contextMust(t, err)
			notP, err := m.MkNot(p)
			contextMust(t, err)
			contextMust(t, c.AddSoftConstraint(p, big.NewRat(3, 1), "pref"))
			contextMust(t, c.AddSoftConstraint(notP, big.NewRat(1, 1), "pref"))
			owner, err := model.NewModel(m)
			contextMust(t, err)
			incomplete := owner.Core()
			t.Cleanup(func() { contextMust(t, owner.Close()) })
			ref, err := c.OptimizationCompareModel(0, incomplete, cmp)
			contextMust(t, err)
			defer func() { contextMust(t, m.DelRef(&ref)) }()
			if count, err := incomplete.NumConstants(); err != nil || count != 0 {
				t.Fatal("soft comparison completed the model", count, err)
			}
			comparison, err := m.RefGet(ref)
			contextMust(t, err)
			for _, truth := range []bool{false, true} {
				witness, err := model.NewModelCore(m)
				contextMust(t, err)
				d, err := m.GetDecl(p)
				contextMust(t, err)
				value, err := m.MkBoolVal(truth)
				contextMust(t, err)
				contextMust(t, witness.RegisterConstant(d, value))
				copyRef, err := m.NewRef(comparison)
				contextMust(t, err)
				got := optimizationComparisonValue(t, c, witness, copyRef)
				contextMust(t, witness.Close())
				if want := cmp != OptimizationModelLE; got != want {
					t.Fatal("source zero soft threshold", cmp, truth, got, want)
				}
			}
		})
	}
}

func TestOptimizationComparisonCatchesSourceEvaluatorStepFailure(t *testing.T) {
	for _, soft := range []bool{false, true} {
		for _, cmp := range []OptimizationModelComparison{OptimizationModelGE, OptimizationModelLE, OptimizationModelGT} {
			t.Run(fmt.Sprintf("soft=%v/comparison=%d", soft, cmp), func(t *testing.T) {
				c := contextFixture(t)
				m, err := c.Manager()
				contextMust(t, err)
				if soft {
					boolean, err := m.MkBoolSort()
					contextMust(t, err)
					name, err := m.Symbols.Intern("step-failure-soft")
					contextMust(t, err)
					p, err := m.MkConst(name, boolean)
					contextMust(t, err)
					term, err := m.MkNot(p)
					contextMust(t, err)
					contextMust(t, c.AddSoftConstraint(term, big.NewRat(1, 1), "pref"))
				} else {
					x := optimizationComparisonInteger(t, c, "step-failure-term")
					one, err := m.ArithmeticIntegerNumeralText("1")
					contextMust(t, err)
					term, err := m.MkArithmeticAdd(x, one)
					contextMust(t, err)
					contextMust(t, c.AddObjective(term, true))
				}
				owner, err := model.NewModel(m)
				contextMust(t, err)
				t.Cleanup(func() { contextMust(t, owner.Close()) })
				local, err := c.config.Params.NewValue()
				contextMust(t, err)
				t.Cleanup(func() { contextMust(t, c.config.Params.Del(&local)) })
				key := "max_steps"
				contextMust(t, c.config.Params.SetUIntName(&local, &key, 0))
				c.config.ModelEvaluator.Params = &local
				ref, err := c.OptimizationCompareModel(0, owner.Core(), cmp)
				contextMust(t, err)
				t.Cleanup(func() { contextMust(t, m.DelRef(&ref)) })
				result, err := m.RefGet(ref)
				contextMust(t, err)
				if !soft {
					neutral := m.IsTrue(result)
					if cmp == OptimizationModelGT {
						neutral = m.IsFalse(result)
					}
					if !neutral {
						t.Fatal("failed term evaluation was not neutral")
					}
				}
				count, err := owner.Core().NumConstants()
				contextMust(t, err)
				if count != 0 {
					t.Fatal("failed evaluation completed the model")
				}
			})
		}
	}
}

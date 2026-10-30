package cmd_context

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func objectivePipelineFacadeInteger(t *testing.T, c *Context, name string) ast.ASTHandle {
	t.Helper()
	m, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	arith, err := m.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	sort, err := m.MkFamilySort(arith, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	value, err := m.MkConst(symbol, sort)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestCommandContextObjectivePipelineFacade(t *testing.T) {
	c := contextFixture(t)
	x := objectivePipelineFacadeInteger(t, c, "facade-x")
	if err := c.AddObjective(x, true); err != nil {
		t.Fatal(err)
	}
	set, err := c.EncodeOptimizationFormulas()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := set.Decode()
	if err != nil || len(decoded.Objectives) != 1 {
		t.Fatal(decoded, err)
	}
	if err = set.Close(); err != nil {
		t.Fatal(err)
	}
	pipeline, err := c.PrepareOptimizationPipeline(nil)
	if err != nil {
		t.Fatal(err)
	}
	info, err := pipeline.ObjectiveInfo(0)
	if err != nil || info.Soft || info.Term != x || info.Converted {
		t.Fatal(info, err)
	}
	if err = pipeline.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCommandContextPurifiedPipelineFacade(t *testing.T) {
	c := contextFixture(t)
	x := objectivePipelineFacadeInteger(t, c, "facade-purified-x")
	m, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	zero, err := m.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	if err != nil {
		t.Fatal(err)
	}
	condition, err := m.MkArithmeticGT(x, zero)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.AddSoftConstraint(condition, big.NewRat(4, 1), "pref"); err != nil {
		t.Fatal(err)
	}
	pipeline, err := c.PreparePurifiedOptimizationPipeline(nil)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := pipeline.Statistics()
	if err != nil || stats.SoftDefinitions != 1 || stats.FreshDeclarations != 1 {
		t.Fatal(stats, err)
	}
	if err = pipeline.Close(); err != nil {
		t.Fatal(err)
	}
}

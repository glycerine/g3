package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func apiObjectivePipelineInteger(t *testing.T, context *SMTContext, name string) ast.ASTHandle {
	t.Helper()
	manager, err := context.Manager()
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

func TestSMTContextObjectivePipelineFacades(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	x := apiObjectivePipelineInteger(t, context, "api-pipeline-x")
	astTestMust(t, context.Maximize(x))
	set, err := context.EncodeOptimizationFormulas()
	astTestMust(t, err)
	decoded, err := set.Decode()
	astTestMust(t, err)
	if len(decoded.Objectives) != 1 {
		t.Fatal(decoded)
	}
	astTestMust(t, set.Close())
	pipeline, err := context.PrepareOptimizationPipeline(nil)
	astTestMust(t, err)
	if pipeline.ObjectiveCount() != 1 {
		t.Fatal(pipeline.ObjectiveCount())
	}
	astTestMust(t, pipeline.Close())
	purified, err := context.PreparePurifiedOptimizationPipeline(nil)
	astTestMust(t, err)
	stats, err := purified.Statistics()
	astTestMust(t, err)
	if stats.FreshDeclarations != 0 {
		t.Fatal(stats)
	}
	astTestMust(t, purified.Close())
}

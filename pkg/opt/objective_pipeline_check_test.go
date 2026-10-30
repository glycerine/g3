package opt_test

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestObjectivePipelineChecksSoftThroughArithmeticSolver(t *testing.T) {
	f := newObjectivePipelineFixture(t)
	p := pipelineBoolean(t, f, "pipeline-check-p")
	notP, err := f.manager.MkNot(p)
	if err != nil {
		t.Fatal(err)
	}
	objective := opt.DecomposedObjective{SourceKind: opt.FormulaMaxSAT, SoftTerms: []ast.ASTHandle{p}, Weights: []big.Rat{*big.NewRat(2, 1)}, Offset: *big.NewRat(5, 1), Negate: true, Converted: true}
	pipeline, err := opt.NewObjectivePipeline(f.manager, f.params, "QF_UF", f.config, []ast.ASTHandle{notP}, []opt.DecomposedObjective{objective})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pipeline.Close(); err != nil {
			t.Error(err)
		}
	}()
	result, err := pipeline.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := result.Close(); err != nil {
			t.Error(err)
		}
	}()
	if result.Status != lbool.True || !result.Optimal || result.Model == nil || len(result.Values) != 1 || !result.Values[0].Soft || result.Values[0].Exact.RatString() != "3" {
		t.Fatal(result.Status, result.Optimal, result.Model, result.Values)
	}
	if !result.Statistics.Arithmetic || result.Statistics.Objectives != 1 || result.Statistics.Hard != 1 {
		t.Fatal(result.Statistics)
	}
}

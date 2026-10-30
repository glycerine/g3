package opt_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

func TestObjectivePipelineBuildInspectAndCopy(t *testing.T) {
	f := newObjectivePipelineFixture(t)
	p := pipelineBoolean(t, f, "pipeline-build-p")
	weight := *big.NewRat(3, 2)
	objectives := []opt.DecomposedObjective{{SourceKind: opt.FormulaMaxSAT, SoftTerms: []ast.ASTHandle{p}, Weights: []big.Rat{weight}, ID: "pref", Converted: true}}
	pipeline, err := opt.NewObjectivePipeline(f.manager, f.params, "QF_UF", f.config, nil, objectives)
	if err != nil {
		t.Fatal(err)
	}
	objectives[0].Weights[0].SetInt64(99)
	if pipeline.ObjectiveCount() != 1 || pipeline.HardCount() != 0 {
		t.Fatal(pipeline.ObjectiveCount(), pipeline.HardCount())
	}
	info, err := pipeline.ObjectiveInfo(0)
	if err != nil || !info.Soft || info.SoftCount != 1 || info.ID != "pref" || !info.Converted {
		t.Fatal(info, err)
	}
	if _, err = pipeline.ObjectiveInfo(1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if err = pipeline.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestObjectivePipelineBuildGuards(t *testing.T) {
	f := newObjectivePipelineFixture(t)
	p := pipelineBoolean(t, f, "pipeline-guard-p")
	objective := opt.DecomposedObjective{SourceKind: opt.FormulaMaxSAT, SoftTerms: []ast.ASTHandle{p}, Converted: true}
	if _, err := opt.NewObjectivePipeline(nil, f.params, "QF_UF", f.config, nil, []opt.DecomposedObjective{objective}); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if _, err := opt.NewObjectivePipeline(f.manager, nil, "QF_UF", f.config, nil, []opt.DecomposedObjective{objective}); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if _, err := opt.NewObjectivePipeline(f.manager, f.params, "QF_UF", f.config, nil, nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if _, err := opt.NewObjectivePipeline(f.manager, f.params, "QF_UF", f.config, nil, []opt.DecomposedObjective{objective}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
}

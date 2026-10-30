package opt_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

func TestObjectivePipelineClosedLifecycle(t *testing.T) {
	f := newObjectivePipelineFixture(t)
	p := pipelineBoolean(t, f, "pipeline-close-p")
	pipeline, err := opt.NewObjectivePipeline(f.manager, f.params, "QF_UF", f.config, nil, []opt.DecomposedObjective{{SourceKind: opt.FormulaMaxSAT, SoftTerms: []ast.ASTHandle{p}, Weights: []big.Rat{*big.NewRat(1, 1)}, Converted: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err = pipeline.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = pipeline.Check(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
	if _, err = pipeline.ObjectiveInfo(0); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
	if err = pipeline.Close(); err != nil {
		t.Fatal(err)
	}
	var result *opt.ObjectivePipelineResult
	if err = result.Close(); err != nil {
		t.Fatal(err)
	}
}

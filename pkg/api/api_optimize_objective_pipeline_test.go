package api

import (
	"errors"
	"testing"
)

func TestAPIOptimizeObjectivePipelineFacadesAndLifecycle(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	x := apiObjectivePipelineInteger(t, context, "api-optimize-pipeline-x")
	optimizer, err := context.NewOptimize()
	astTestMust(t, err)
	_, err = optimizer.Maximize(x)
	astTestMust(t, err)
	set, err := optimizer.EncodeOptimizationFormulas()
	astTestMust(t, err)
	astTestMust(t, set.Close())
	pipeline, err := optimizer.PrepareOptimizationPipeline(nil)
	astTestMust(t, err)
	astTestMust(t, pipeline.Close())
	purified, err := optimizer.PreparePurifiedOptimizationPipeline(nil)
	astTestMust(t, err)
	astTestMust(t, purified.Close())
	astTestMust(t, optimizer.Close())
	if _, err = optimizer.EncodeOptimizationFormulas(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
	if _, err = optimizer.PrepareOptimizationPipeline(nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
	if _, err = optimizer.PreparePurifiedOptimizationPipeline(nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
}

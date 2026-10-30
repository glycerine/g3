package opt_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/opt"
)

func TestPurifiedObjectivePipelineBuildsArithmeticDefinitions(t *testing.T) {
	f := newObjectivePipelineFixture(t)
	arith, err := f.manager.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	sort, err := f.manager.MkFamilySort(arith, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	x, err := f.manager.MkFreshConst("pipeline-purified-x", sort)
	if err != nil {
		t.Fatal(err)
	}
	square, err := f.manager.MkArithmeticMul(x, x)
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := opt.NewPurifiedObjectivePipeline(f.manager, f.params, "QF_NIA", f.config, nil, []opt.DecomposedObjective{{SourceKind: opt.FormulaMaximize, Term: square, OriginalTerm: square}})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := pipeline.Statistics()
	if err != nil || stats.FreshDeclarations != 1 || stats.ArithmeticDefinitions != 1 {
		t.Fatal(stats, err)
	}
	if err = pipeline.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = pipeline.Statistics(); err == nil {
		t.Fatal("closed purified pipeline remained open")
	}
}

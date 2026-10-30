package opt

// Source: src/opt/maxsmt.cpp model publication without a converter/model.

import "testing"

func TestMaxSMTModelFilterNoOpsBeforePublishedModel(t *testing.T) {
	solver := &ArithmeticSolver{}
	if err := solver.filterResultModel(nil); err != nil {
		t.Fatal(err)
	}
	result := &ArithmeticResult{}
	if err := solver.filterResultModel(result); err != nil || result.ModelFiltered {
		t.Fatal("empty publication", err, result.ModelFiltered)
	}
}

package opt

import (
	"testing"
)

func TestMaxResPrimalDualEmptyCorrectionClosesBound(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	engine := &maxResEngine{solver: &ArithmeticSolver{manager: manager}, strategy: maxResPrimalDual, index: 0, bestSet: true, lower: testRat(2, 1), upper: testRat(5, 2)}
	q := manager.NumeralService().Manager
	result := &ArithmeticResult{Values: NewObjectiveValues(q, 1), Q: q}
	done, err := engine.processSearchSAT(result, true)
	if err != nil || !done {
		t.Fatal("empty correction set should complete primal-dual search", done, err)
	}
	if !ratEq(&engine.lower, &engine.upper) || !ratIs(&engine.lower, 5, 2) {
		t.Fatal("primal-dual terminal bound", &engine.lower, &engine.upper)
	}
	if !ratIs(&result.Values[0].SoftCost, 5, 2) {
		t.Fatal("published cost must equal the closed bound", &result.Values[0].SoftCost)
	}
}

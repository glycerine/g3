package opt

import (
	"math/big"
	"testing"
)

func TestMaxResPrimalDualEmptyCorrectionClosesBound(t *testing.T) {
	engine := &maxResEngine{strategy: maxResPrimalDual, index: 0, bestSet: true}
	engine.lower.Set(big.NewRat(2, 1))
	engine.upper.Set(big.NewRat(5, 2))
	result := &ArithmeticResult{Values: []ObjectiveValue{{}}}
	done, err := engine.processSearchSAT(result, true)
	if err != nil || !done {
		t.Fatal("empty correction set should complete primal-dual search", done, err)
	}
	if engine.lower.Cmp(&engine.upper) != 0 || engine.lower.Cmp(big.NewRat(5, 2)) != 0 {
		t.Fatal("primal-dual terminal bound", &engine.lower, &engine.upper)
	}
	if result.Values[0].SoftCost.Cmp(big.NewRat(5, 2)) != 0 {
		t.Fatal("published cost must equal the closed bound", &result.Values[0].SoftCost)
	}
}

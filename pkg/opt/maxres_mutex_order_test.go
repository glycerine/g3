package opt

// Source: src/opt/maxsmt.cpp rational mutex weight ordering.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResMutexOrderIsDescendingAndStable(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "maxres-mutex-order-a")
	b := maxResTestBool(t, manager, "maxres-mutex-order-b")
	c := maxResTestBool(t, manager, "maxres-mutex-order-c")
	weights := map[ast.ASTHandle]*big.Rat{a: big.NewRat(2, 1), b: big.NewRat(7, 2), c: big.NewRat(2, 1)}
	mutex := []ast.ASTHandle{a, b, c}
	sortMaxResMutex(mutex, weights)
	if mutex[0] != b || mutex[1] != a || mutex[2] != c {
		t.Fatal("rational mutex order", mutex)
	}
}

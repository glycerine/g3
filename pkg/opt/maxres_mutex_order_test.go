package opt

// Source: src/opt/maxsmt.cpp rational mutex weight ordering.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResMutexOrderIsDescendingAndStable(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "maxres-mutex-order-a")
	b := maxResTestBool(t, manager, "maxres-mutex-order-b")
	c := maxResTestBool(t, manager, "maxres-mutex-order-c")
	weights := map[ast.ASTHandle]*numeral.Rational{a: testRatPtr(2, 1), b: testRatPtr(7, 2), c: testRatPtr(2, 1)}
	mutex := []ast.ASTHandle{a, b, c}
	engine := &maxResEngine{lower: testRat(0, 1), upper: testRat(0, 1), solver: &ArithmeticSolver{manager: manager}}
	engine.sortMaxResMutex(mutex, weights)
	if mutex[0] != b || mutex[1] != a || mutex[2] != c {
		t.Fatal("rational mutex order", mutex)
	}
}

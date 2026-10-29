package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticModelProjectRealPrefersStrictSourceCandidate(t *testing.T) {
	f := newArithmeticModelFixture(t, true, "5")
	m := f.r.manager
	n := func(text string) ast.ASTHandle { return f.own(m.ArithmeticRealNumeralText(text)) }
	zero, minusOne, minusTwo := n("0"), n("-1"), n("-2")
	negativeX := f.own(m.MkArithmeticMul(minusOne, f.x))
	lower0 := f.own(m.MkArithmeticLE(negativeX, zero))
	lower1 := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(negativeX, minusOne)), zero))
	lower2 := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(negativeX, minusTwo)), zero))
	upper := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(f.x, n("-5"))), zero))
	strict := f.own(m.MkArithmeticLT(f.own(m.MkArithmeticAdd(f.x, n("-6"))), zero))
	original, _ := f.project(t, lower0, lower1, lower2, upper, strict)
	if f.plugin.branches[ArithmeticBranchKey{Formula: original, Variable: f.x, Branch: 3}] == nil {
		t.Fatal("real projection did not select source strict branch ordering")
	}
}

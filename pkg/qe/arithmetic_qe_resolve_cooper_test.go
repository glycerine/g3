package qe

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticQESmallCooperResolutionMatchesSolver(t *testing.T) {
	for _, coefficients := range [][2]int64{{2, -3}, {3, -2}, {-2, 3}, {-3, 2}} {
		t.Run(strconv.FormatInt(coefficients[0], 10), func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			x := f.constant("cooper-check-x", f.integer)
			left := f.constant("cooper-check-left", f.integer)
			right := f.constant("cooper-check-right", f.integer)
			u, err := NewArithmeticQETermUtil(m)
			fmMust(t, err)
			defer func() { fmMust(t, u.Close()) }()
			result, handled, err := u.ResolveUnitOrReal(x, false, big.NewRat(coefficients[0], 1), left, big.NewRat(coefficients[1], 1), right)
			fmMust(t, err)
			if !handled {
				t.Fatal("small Cooper resolution was not handled")
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			contains, err := ast.NewContainsApp(m, x)
			fmMust(t, err)
			found, err := contains.Check(value)
			fmMust(t, err)
			fmMust(t, contains.Close())
			if found {
				t.Fatal("resolved formula still contains the eliminated variable")
			}
			firstCoeff := f.number(strconv.FormatInt(coefficients[0], 10))
			secondCoeff := f.number(strconv.FormatInt(coefficients[1], 10))
			firstTerm := f.own(m.MkArithmeticAdd(f.own(m.MkArithmeticMul(firstCoeff, x)), left))
			secondTerm := f.own(m.MkArithmeticAdd(f.own(m.MkArithmeticMul(secondCoeff, x)), right))
			zero := f.number("0")
			firstBound := f.own(m.MkArithmeticLE(firstTerm, zero))
			secondBound := f.own(m.MkArithmeticLE(secondTerm, zero))
			k := qsatKernelOwner(t, f)
			original, err := k.solver()
			fmMust(t, err)
			projected, err := k.solver()
			fmMust(t, err)
			fmMust(t, original.AssertFormula(firstBound))
			fmMust(t, original.AssertFormula(secondBound))
			fmMust(t, projected.AssertFormula(value))
			for l := -2; l <= 2; l++ {
				for r := -2; r <= 2; r++ {
					leftEq := f.own(m.MkEq(left, f.number(strconv.Itoa(l))))
					rightEq := f.own(m.MkEq(right, f.number(strconv.Itoa(r))))
					fmMust(t, original.Push())
					fmMust(t, projected.Push())
					fmMust(t, original.AssertFormula(leftEq))
					fmMust(t, original.AssertFormula(rightEq))
					fmMust(t, projected.AssertFormula(leftEq))
					fmMust(t, projected.AssertFormula(rightEq))
					want, e := original.CheckSat(nil)
					fmMust(t, e)
					got, e := projected.CheckSat(nil)
					fmMust(t, e)
					if got != want {
						t.Fatalf("coefficients %v, offsets (%d,%d): got %v want %v", coefficients, l, r, got, want)
					}
					fmMust(t, original.Pop(1))
					fmMust(t, projected.Pop(1))
				}
			}
		})
	}
}

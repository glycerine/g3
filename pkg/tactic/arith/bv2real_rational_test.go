package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:159-176.

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func bv2RealAlignmentInputs(t *testing.T, u *BV2RealUtil) [4]ast.ASTHandle {
	t.Helper()
	var args [4]ast.ASTHandle
	for i, n := range []int64{-3, 2, 5, -7} {
		args[i] = bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle { return c.signedNumeral(big.NewRat(n, 1)) })
	}
	return args
}

func TestBV2RealDivisorAlignmentProjectArithmeticAndScope(t *testing.T) {
	wide1 := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 80), big.NewInt(1))
	wide1.Mul(wide1, big.NewInt(15))
	wide2 := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 90), big.NewInt(7))
	wide2.Mul(wide2, big.NewInt(21))
	for i, pair := range [][2]*big.Int{{big.NewInt(6), big.NewInt(10)}, {big.NewInt(3), big.NewInt(15)}, {wide1, wide2}} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			u, f := bv2RealFixture(t, 2048)
			args := bv2RealAlignmentInputs(t, u)
			d1, d2 := new(big.Rat).SetInt(pair[0]), new(big.Rat).SetInt(pair[1])
			g := new(big.Int).GCD(nil, nil, pair[0], pair[1])
			common := new(big.Int).Quo(new(big.Int).Mul(pair[0], pair[1]), g)
			factors := []*big.Int{new(big.Int).Quo(pair[1], g), new(big.Int).Quo(pair[1], g), new(big.Int).Quo(pair[0], g), new(big.Int).Quo(pair[0], g)}
			c := &bv2RealCall{u: u}
			t.Cleanup(func() { equationMust(t, c.close()) })
			service := f.m.NumeralService()
			outer := c.number()
			equationMust(t, c.err)
			equationMust(t, service.SetString(outer, "1234567890123456789012345678901234567890"))
			a, b, x, y := c.alignDivisors(args[0], args[1], args[2], args[3], d1, d2)
			equationMust(t, c.err)
			if d1.Num().Cmp(common) != 0 || d2.Num().Cmp(common) != 0 {
				t.Fatal("wrong common divisor", d1, d2, common)
			}
			for i, h := range []ast.ASTHandle{a, b, x, y} {
				want := new(big.Int).Mul(big.NewInt([]int64{-3, 2, 5, -7}[i]), factors[i])
				if got := bv2RealSignedValue(t, f, h); got.Cmp(want) != 0 {
					t.Fatal("scaled coefficient", i, got, want)
				}
			}
			text, err := service.String(outer)
			equationMust(t, err)
			if text != "1234567890123456789012345678901234567890" || len(c.numbers) != 1 {
				t.Fatal("alignment numeric locals escaped or destroyed caller ownership", text, len(c.numbers))
			}
		})
	}
}

func TestBV2RealDivisorAlignmentFailureAssignmentOrder(t *testing.T) {
	u, f := bv2RealFixture(t, 256)
	args := bv2RealAlignmentInputs(t, u)
	c := &bv2RealCall{u: u}
	t.Cleanup(func() { equationMust(t, c.close()) })
	d1 := new(big.Rat).SetInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 521), big.NewInt(1)))
	d2 := new(big.Rat).SetInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 607), big.NewInt(7)))
	wantD1, wantD2 := new(big.Rat).Set(d1), new(big.Rat).Set(d2)
	f.m.Budget().FailAfter(0)
	a, b, x, y := c.alignDivisors(args[0], args[1], args[2], args[3], d1, d2)
	f.m.Budget().FailAfter(-1)
	var failure umemory.Failure
	if !errors.Is(c.err, arena.ErrInjectedFailure) && (!errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
		t.Fatal("scaling allocation failure not reached", c.err)
	}
	if d1.Cmp(wantD1) != 0 || d2.Cmp(wantD2) != 0 {
		t.Fatal("failed scaling published common divisors", d1, d2)
	}
	if [4]ast.ASTHandle{a, b, x, y} != args {
		t.Fatal("failed scaling replaced an unfinished coefficient")
	}
}

func TestBV2RealDivisorAlignmentAllocationFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 2048; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			u, f := bv2RealFixture(t, 256)
			args := bv2RealAlignmentInputs(t, u)
			c := &bv2RealCall{u: u}
			t.Cleanup(func() { equationMust(t, c.close()) })
			factor := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 521), big.NewInt(1))
			oldD1 := new(big.Rat).SetInt(new(big.Int).Mul(factor, big.NewInt(6)))
			oldD2 := new(big.Rat).SetInt(new(big.Int).Mul(factor, big.NewInt(10)))
			common := new(big.Rat).SetInt(new(big.Int).Mul(factor, big.NewInt(30)))
			d1, d2 := new(big.Rat).Set(oldD1), new(big.Rat).Set(oldD2)
			f.m.Budget().FailAfter(stop)
			a, b, x, y := c.alignDivisors(args[0], args[1], args[2], args[3], d1, d2)
			f.m.Budget().FailAfter(-1)
			if c.err == nil {
				succeeded = true
				if d1.Cmp(common) != 0 || d2.Cmp(d1) != 0 {
					t.Fatal("common divisors not published")
				}
			} else {
				var failure umemory.Failure
				if !errors.Is(c.err, arena.ErrInjectedFailure) && (!errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
					t.Fatal(c.err)
				}
				if d1.Cmp(oldD1) != 0 && d1.Cmp(common) != 0 || d2.Cmp(oldD2) != 0 && d2.Cmp(common) != 0 {
					t.Fatal("invalid divisor assignment prefix", d1, d2)
				}
				if d2.Cmp(common) == 0 && d1.Cmp(common) != 0 {
					t.Fatal("second divisor was assigned before the first")
				}
				if d1.Cmp(oldD1) != 0 {
					for i, h := range []ast.ASTHandle{a, b, x, y} {
						if h == args[i] {
							t.Fatal("divisor assignment preceded completed scaling", i)
						}
					}
				}
			}
			// Each returned coefficient is either unchanged or a completed source
			// assignment. No unowned/null handle may replace an unfinished expression.
			failedPrefix := false
			for i, h := range []ast.ASTHandle{a, b, x, y} {
				if h.IsNull() {
					t.Fatal("failed coefficient assignment returned null", i)
				}
				if h == args[i] {
					failedPrefix = true
				} else if failedPrefix {
					t.Fatal("scaling skipped an earlier coefficient", i)
				}
			}
			equationMust(t, c.close())
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("alignment allocation success frontier not reached")
	}
}

func TestBV2RealEqualDivisorsSkipProjectArithmetic(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	args := bv2RealAlignmentInputs(t, u)
	c := &bv2RealCall{u: u}
	t.Cleanup(func() { equationMust(t, c.close()) })
	d1, d2 := big.NewRat(6, 1), big.NewRat(6, 1)
	f.m.Budget().FailAfter(0)
	a, b, x, y := c.alignDivisors(args[0], args[1], args[2], args[3], d1, d2)
	f.m.Budget().FailAfter(-1)
	equationMust(t, c.err)
	if [4]ast.ASTHandle{a, b, x, y} != args {
		t.Fatal("equal divisors changed coefficients")
	}
}

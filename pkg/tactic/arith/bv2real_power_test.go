package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:143-150,244-276.

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestBV2RealPowerUsesGeneralExponentiationAndLocalOwnership(t *testing.T) {
	u, f := bv2RealFixture(t, 2048)
	service := f.m.NumeralService()
	before, err := service.Stats()
	equationMust(t, err)
	for _, exponent := range []uint32{0, 1, 31, 32, 65, 521} {
		c := &bv2RealCall{u: u}
		q := c.rationalPowerTwo(exponent)
		equationMust(t, c.err)
		got := c.rationalValue(q)
		equationMust(t, c.err)
		want := new(big.Int).Lsh(big.NewInt(1), uint(exponent))
		if !got.IsInt() || got.Num().Cmp(want) != 0 {
			t.Fatal("power value", exponent, got, want)
		}
		equationMust(t, c.close())
		after, err := service.Stats()
		equationMust(t, err)
		if after.CacheLength != before.CacheLength || after.LiveNumericObjects != before.LiveNumericObjects {
			t.Fatal("general power expanded the unrelated cache or leaked temporaries", exponent, before, after)
		}
	}
}

func TestBV2RealSignedNumeralProjectBitCountBoundaries(t *testing.T) {
	for _, bits := range []uint{0, 1, 4, 31, 32, 65, 521} {
		power := new(big.Int).Lsh(big.NewInt(1), bits)
		for _, offset := range []int64{-1, 0, 1} {
			magnitude := new(big.Int).Add(power, big.NewInt(offset))
			if magnitude.Sign() < 0 {
				continue
			}
			for _, negative := range []bool{false, true} {
				t.Run(fmt.Sprintf("bits=%d/offset=%d/negative=%v", bits, offset, negative), func(t *testing.T) {
					u, f := bv2RealFixture(t, 2048)
					n := new(big.Int).Set(magnitude)
					if negative {
						n.Neg(n)
					}
					c := &bv2RealCall{u: u}
					t.Cleanup(func() { equationMust(t, c.close()) })
					h := c.signedNumeral(new(big.Rat).SetInt(n))
					equationMust(t, c.err)
					width, err := f.m.BitVectorSize(h)
					equationMust(t, err)
					wantWidth := uint32(magnitude.BitLen())
					if wantWidth == 0 {
						wantWidth = 1
					}
					wantWidth++
					if width != wantWidth {
						t.Fatal("signed numeral width", width, wantWidth)
					}
					if got := bv2RealSignedValue(t, f, h); got.Cmp(n) != 0 {
						t.Fatal("signed value", got, n)
					}
					if len(c.numbers) != 0 {
						t.Fatal("bit-count/absolute-value numeric scope escaped")
					}
				})
			}
		}
	}
}

func TestBV2RealExtensionProjectPowerAndSignBoundary(t *testing.T) {
	for _, width := range []uint32{1, 4, 32, 65, 521} {
		limit := new(big.Int).Lsh(big.NewInt(1), uint(width-1))
		values := []*big.Int{big.NewInt(0), new(big.Int).Sub(new(big.Int).Set(limit), big.NewInt(1)), new(big.Int).Set(limit), new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(width)), big.NewInt(1))}
		for i, n := range values {
			t.Run(fmt.Sprintf("width=%d/value=%d", width, i), func(t *testing.T) {
				u, f := bv2RealFixture(t, 2048)
				c := &bv2RealCall{u: u}
				t.Cleanup(func() { equationMust(t, c.close()) })
				input := c.numeral(new(big.Rat).SetInt(n), width)
				equationMust(t, c.err)
				result := c.extend(7, input)
				equationMust(t, c.err)
				gotWidth, err := f.m.BitVectorSize(result)
				equationMust(t, err)
				if gotWidth != width+7 {
					t.Fatal("extension width", gotWidth)
				}
				kind, err := f.m.GetDeclKind(result)
				equationMust(t, err)
				if (kind == 0) != (n.Cmp(limit) < 0) {
					t.Fatal("positive numeral shortcut crossed sign boundary", kind, n, limit)
				}
				want := new(big.Int).Set(n)
				if n.Cmp(limit) >= 0 {
					want.Sub(want, new(big.Int).Lsh(big.NewInt(1), uint(width)))
				}
				if got := bv2RealSignedValue(t, f, result); got.Cmp(want) != 0 {
					t.Fatal("signed extension", got, want)
				}
			})
		}
	}
}

func TestBV2RealCachedWideZeroStillComputesSourcePower(t *testing.T) {
	u, f := bv2RealFixture(t, 2048)
	input := f.bv(t, 0, 521)
	f.bv(t, 0, 522) // The final zero numeral already exists; source power still runs.
	c := &bv2RealCall{u: u}
	t.Cleanup(func() { equationMust(t, c.close()) })
	f.m.Budget().FailAfter(0)
	result := c.extend(1, input)
	f.m.Budget().FailAfter(-1)
	var failure umemory.Failure
	if !errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory {
		t.Fatal("source power numeric allocation was bypassed", c.err)
	}
	if !result.IsNull() {
		t.Fatal("failed source power published extended zero")
	}
}

func TestBV2RealPowerHelperFailureFrontiers(t *testing.T) {
	for _, op := range []string{"signed-numeral", "extend", "signed-real"} {
		succeeded := false
		for stop := int64(0); stop < 2048; stop++ {
			t.Run(fmt.Sprintf("%s/%d", op, stop), func(t *testing.T) {
				u, f := bv2RealFixture(t, 256)
				input := f.bv(t, 0, 65)
				n := new(big.Rat).SetInt(new(big.Int).Neg(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(3))))
				c := &bv2RealCall{u: u}
				t.Cleanup(func() { equationMust(t, c.close()) })
				outer := c.number()
				equationMust(t, c.err)
				equationMust(t, f.m.NumeralService().SetInt64(outer, 37))
				f.m.Budget().FailAfter(stop)
				var result ast.ASTHandle
				switch op {
				case "signed-numeral":
					result = c.signedNumeral(n)
				case "extend":
					result = c.extend(1, input)
				case "signed-real":
					result = c.signedReal(input)
				}
				f.m.Budget().FailAfter(-1)
				if c.err == nil {
					succeeded = true
					if result.IsNull() {
						t.Fatal("successful helper lost result")
					}
				} else {
					var failure umemory.Failure
					if !errors.Is(c.err, arena.ErrInjectedFailure) && (!errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
						t.Fatal(c.err)
					}
					if !result.IsNull() {
						t.Fatal("failed helper published an expression")
					}
				}
				text, err := f.m.NumeralService().String(outer)
				equationMust(t, err)
				if text != "37" || len(c.numbers) != 1 {
					t.Fatal("inner numeric scope changed caller ownership", text, len(c.numbers))
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("helper allocation success frontier not reached", op)
		}
	}
}

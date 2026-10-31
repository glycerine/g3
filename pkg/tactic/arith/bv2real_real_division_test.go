package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:127-141.

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestBV2RealRealDivisionOwnedCopyAndFailureFrontiers(t *testing.T) {
	for _, text := range []string{"3", "1267650600228229401496703205379"} {
		succeeded := false
		for stop := int64(0); stop < 256; stop++ {
			t.Run(fmt.Sprintf("divisor=%s/%d", text, stop), func(t *testing.T) {
				u, f := bv2RealFixture(t, 256)
				input := fixDLNumber(t, f.m, false, "-7")
				c := &bv2RealCall{u: u}
				t.Cleanup(func() { equationMust(t, c.close()) })
				q, ok := new(big.Rat).SetString(text)
				if !ok {
					t.Fatal(text)
				}
				divisor := c.rational(q)
				equationMust(t, c.err)
				f.m.Budget().FailAfter(stop)
				result := c.divideRealOwned(input, divisor)
				f.m.Budget().FailAfter(-1)
				if c.err != nil {
					var failure umemory.Failure
					if !errors.Is(c.err, arena.ErrInjectedFailure) && (!errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
						t.Fatal(c.err)
					}
					if !result.IsNull() {
						t.Fatal("failed division published an expression")
					}
				} else {
					succeeded = true
					kind, err := f.m.GetDeclKind(result)
					equationMust(t, err)
					left, err := f.m.GetArg(result, 0)
					equationMust(t, err)
					right, err := f.m.GetArg(result, 1)
					equationMust(t, err)
					actual, integer, yes, err := f.m.ArithmeticNumeralText(right)
					equationMust(t, err)
					if kind != 10 || left != input || !yes || integer || actual != text {
						t.Fatal("source real division graph", kind, left, input, actual, integer)
					}
					// The AST numeral must own its copy after the caller mutates d.
					equationMust(t, f.m.NumeralService().SetInt64(divisor, 19))
					actual, _, _, err = f.m.ArithmeticNumeralText(right)
					equationMust(t, err)
					if actual != text {
						t.Fatal("division numeral borrowed the mutable divisor", actual)
					}
				}
				if len(c.numbers) != 1 {
					t.Fatal("division copy escaped its lexical scope", len(c.numbers))
				}
				actual, _, _, err := f.m.ArithmeticNumeralText(input)
				equationMust(t, err)
				if actual != "-7" {
					t.Fatal("division changed the input expression", actual)
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("division success frontier was not reached", text)
		}
	}
}

func TestBV2RealRealDivisionCachedOutputReleasesTemporaryCopies(t *testing.T) {
	u, f := bv2RealFixture(t, 256)
	input := fixDLNumber(t, f.m, false, "7")
	c := &bv2RealCall{u: u}
	t.Cleanup(func() { equationMust(t, c.close()) })
	divisor := c.rational(new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 100)))
	equationMust(t, c.err)
	h := c.divideRealOwned(input, divisor)
	equationMust(t, c.err)
	before, err := f.m.NumeralService().Stats()
	equationMust(t, err)
	for i := 0; i < 4; i++ {
		got := c.divideRealOwned(input, divisor)
		equationMust(t, c.err)
		if got != h || len(c.numbers) != 1 {
			t.Fatal("cached division identity or numeric lifetime changed")
		}
		after, err := f.m.NumeralService().Stats()
		equationMust(t, err)
		if after.LiveNumericObjects != before.LiveNumericObjects {
			t.Fatal("division temporary numeric copies survived the helper", before, after)
		}
	}
}

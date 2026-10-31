package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:208-212,264-277;
// bit-vector numeral construction and recognition use lexical rational values.

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestBV2RealNumericHelpersReleaseWidePayloadsBeforeCallerReturns(t *testing.T) {
	u, f := bv2RealFixture(t, 512)
	wide := new(big.Rat).SetInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 250), big.NewInt(19)))
	h := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle { return c.numeral(wide, 257) })
	c := &bv2RealCall{u: u}
	t.Cleanup(func() { equationMust(t, c.close()) })
	service := f.m.NumeralService()
	// An enclosing numeric owner must survive the inner helper scopes.
	outer := c.number()
	equationMust(t, c.err)
	equationMust(t, service.SetString(outer, "1234567890123456789012345678901234567890"))
	before, err := service.Stats()
	equationMust(t, err)
	for i := 0; i < 8; i++ {
		same := c.numeral(wide, 257)
		equationMust(t, c.err)
		if same != h {
			t.Fatal("numeral cache identity changed")
		}
		value, ok := c.bvNumeral(h)
		equationMust(t, c.err)
		if !ok || value.Cmp(wide) != 0 {
			t.Fatal("numeral copy lost at helper handoff", value)
		}
		after, err := service.Stats()
		equationMust(t, err)
		if after.LiveNumericObjects != before.LiveNumericObjects {
			t.Fatal("helper left numeric payloads alive in the enclosing call", before, after)
		}
		text, err := service.String(outer)
		equationMust(t, err)
		if text != "1234567890123456789012345678901234567890" {
			t.Fatal("inner scope destroyed the caller's numeral", text)
		}
	}
}

func TestBV2RealNumericHelperFailureScopes(t *testing.T) {
	for _, query := range []bool{false, true} {
		succeeded := false
		for stop := int64(0); stop < 128; stop++ {
			t.Run(fmt.Sprintf("query=%v/%d", query, stop), func(t *testing.T) {
				u, f := bv2RealFixture(t, 512)
				wide := new(big.Rat).SetInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 250), big.NewInt(19)))
				h := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle { return c.numeral(wide, 257) })
				c := &bv2RealCall{u: u}
				t.Cleanup(func() {
					if c != nil {
						equationMust(t, c.close())
					}
				})
				f.m.Budget().FailAfter(stop)
				if query {
					c.bvNumeral(h)
				} else {
					c.numeral(wide, 257)
				}
				f.m.Budget().FailAfter(-1)
				if c.err == nil {
					succeeded = true
				} else {
					var failure umemory.Failure
					if !errors.Is(c.err, arena.ErrInjectedFailure) && (!errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
						t.Fatal(c.err)
					}
				}
				equationMust(t, c.close())
				if len(c.numbers) != 0 {
					t.Fatal("numeric lexical cleanup left owned values", len(c.numbers))
				}
				c = nil
				// The already-owned AST numeral survives failed helper construction/copy.
				value := bv2RealSignedValue(t, f, h)
				if value.Cmp(wide.Num()) != 0 {
					t.Fatal("failure damaged the caller's numeral", value)
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("numeric helper success frontier not reached", query)
		}
	}
}

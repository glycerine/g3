package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:300-309.

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestNLA2BVProjectLog2WideBoundariesAndLocalOwnership(t *testing.T) {
	r := nla2BVFixture(t, false)
	c, err := r.newInvocation()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, c.close()) })
	s := r.manager.NumeralService()
	outer := c.local.rational(big.NewRat(37, 1))
	equationMust(t, c.local.err)
	for _, exponent := range []uint{1, 31, 32, 63, 65, 127, 257, 521} {
		for _, offset := range []int64{-1, 0, 1} {
			n := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), exponent), big.NewInt(offset))
			got := c.log2Rat(new(big.Rat).SetInt(n))
			equationMust(t, c.local.err)
			// Integer ceil(log2(n)) = bit_length(n-1), with source minimum one.
			want := new(big.Int).Sub(new(big.Int).Set(n), big.NewInt(1)).BitLen()
			if want == 0 {
				want = 1
			}
			if got != uint32(want) || len(c.local.numbers) != 1 || len(c.local.readPending) != 0 {
				t.Fatal("project log2 value or lexical ownership", exponent, offset, got, want)
			}
			text, err := s.String(outer)
			equationMust(t, err)
			if text != "37" {
				t.Fatal("log2 destroyed an enclosing numeral", text)
			}
		}
	}
}

func TestNLA2BVProjectLog2CompoundFailureFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 1024; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			r := nla2BVFixture(t, false)
			c, err := r.newInvocation()
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, c.close()) })
			s := r.manager.NumeralService()
			n := new(big.Rat).SetInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 65), big.NewInt(1)))
			input := c.local.rational(n)
			equationMust(t, c.local.err)
			before, err := s.Stats()
			equationMust(t, err)
			persistent := bv2RealRecognitionPersistentObjects(t, s)
			r.manager.Budget().FailAfter(stop)
			got := c.log2Owned(input)
			r.manager.Budget().FailAfter(-1)
			if c.local.err == nil {
				succeeded = true
				if got != 66 {
					t.Fatal("wide ceil(log2)", got)
				}
			} else {
				var failure umemory.Failure
				if !errors.Is(c.local.err, arena.ErrInjectedFailure) && (!errors.As(c.local.err, &failure) || failure.Kind != umemory.FailureMemout || !s.RuntimeOwner().State().OutOfMemory) {
					t.Fatal(c.local.err)
				}
			}
			if len(c.local.numbers) != 1 || len(c.local.readPending) != 0 {
				t.Fatal("log2 numeric receiver escaped helper cleanup")
			}
			actual, err := s.String(input)
			equationMust(t, err)
			if actual != n.RatString() {
				t.Fatal("failed compound multiplication changed its input", actual)
			}
			after, err := s.Stats()
			equationMust(t, err)
			if after.CacheLength != before.CacheLength || after.LiveNumericObjects-bv2RealRecognitionPersistentObjects(t, s) != before.LiveNumericObjects-persistent {
				t.Fatal("log2 grew the power cache or leaked local numeric payloads", before, after)
			}
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("compound log2 success frontier was not reached")
	}
}

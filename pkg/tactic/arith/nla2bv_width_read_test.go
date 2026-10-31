package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:288-298.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestNLA2BVWidthReadIntegerAbsoluteValueAndRealRefusal(t *testing.T) {
	for _, row := range []struct {
		text    string
		integer bool
		want    uint32
	}{
		{"0", true, 2}, {"1", true, 2}, {"-1", true, 2},
		{"2", true, 2}, {"-3", true, 3}, {"4", true, 3}, {"-5", true, 4},
		{"-36893488147419103233", true, 67},
		{"-36893488147419103233", false, 1},
		{"-36893488147419103233/7", false, 1},
	} {
		t.Run(fmt.Sprintf("%s/int=%v", row.text, row.integer), func(t *testing.T) {
			r := nla2BVFixture(t, false)
			c, err := r.newInvocation()
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, c.close()) })
			c.numBits = 1
			h := fixDLNumber(t, r.manager, row.integer, row.text)
			equationMust(t, c.updateNumBits(h))
			if c.numBits != row.want || len(c.local.numbers) != 0 || len(c.local.readPending) != 0 {
				t.Fatal("source width or helper lifetime", c.numBits, row.want)
			}
			// An ordinary application cannot change the selected width.
			equationMust(t, c.updateNumBits(fixDLConst(t, r.manager, true, "width-not-a-numeral")))
			if c.numBits != row.want {
				t.Fatal("non-numeral changed width")
			}
			actual, integer, numeral, err := r.manager.ArithmeticNumeralText(h)
			equationMust(t, err)
			if !numeral || integer != row.integer || actual != row.text {
				t.Fatal("width recognition changed its input", actual, integer)
			}
		})
	}
}

func TestNLA2BVWidthReadAndAbsoluteAssignmentFailureFrontiers(t *testing.T) {
	for _, integer := range []bool{false, true} {
		succeeded := false
		for stop := int64(0); stop < 1024; stop++ {
			t.Run(fmt.Sprintf("integer=%v/%d", integer, stop), func(t *testing.T) {
				r := nla2BVFixture(t, false)
				c, err := r.newInvocation()
				equationMust(t, err)
				t.Cleanup(func() { equationMust(t, c.close()) })
				s := r.manager.NumeralService()
				text := "-36893488147419103233"
				h := fixDLNumber(t, r.manager, integer, text)
				before, err := s.Stats()
				equationMust(t, err)
				persistent := bv2RealRecognitionPersistentObjects(t, s)
				r.manager.Budget().FailAfter(stop)
				err = c.updateNumBits(h)
				r.manager.Budget().FailAfter(-1)
				if err == nil {
					succeeded = true
					want := uint32(4)
					if integer {
						want = 67
					}
					if c.numBits != want {
						t.Fatal("source width after success/refusal", c.numBits, want)
					}
				} else {
					var failure umemory.Failure
					if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != umemory.FailureMemout || !s.RuntimeOwner().State().OutOfMemory) {
						t.Fatal(err)
					}
					if c.numBits != 4 {
						t.Fatal("failed recognition/absolute assignment published a width", c.numBits)
					}
				}
				if len(c.local.numbers) != 0 || len(c.local.readPending) != 0 {
					t.Fatal("width recognition left local numeric owners")
				}
				actual, flag, yes, err := r.manager.ArithmeticNumeralText(h)
				equationMust(t, err)
				if !yes || flag != integer || actual != text {
					t.Fatal("failure damaged the caller's numeral", actual, flag)
				}
				after, err := s.Stats()
				equationMust(t, err)
				if after.LiveNumericObjects-bv2RealRecognitionPersistentObjects(t, s) != before.LiveNumericObjects-persistent {
					t.Fatal("width recognition leaked temporary numeric payloads", before, after)
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("width-read success frontier was not reached", integer)
		}
	}
}

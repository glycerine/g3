package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:300-309.

import (
	"errors"
	"math/big"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c *nla2BVInvocation) log2Rat(n *big.Rat) uint32 {
	local := c.local
	count := len(local.numbers)
	defer func() { local.err = errors.Join(local.err, local.releaseNumbersTo(count)) }()
	value := local.rational(n)
	if local.err != nil {
		return 0
	}
	return c.log2Owned(value)
}

// Keep the actual source pow receiver alive across compound multiplications.
// The same concrete numeric invocation owner used for arithmetic numeral reads
// holds failed lower-operation roots until the receiver is destroyed.
func (c *nla2BVInvocation) log2Owned(n *unumeral.OwnedRational) (bits uint32) {
	local := c.local
	if local.err != nil {
		return 0
	}
	count := len(local.numbers)
	pow := &bv2RealArithmeticRead{manager: c.t.manager}
	defer func() {
		local.err = errors.Join(local.err, local.releaseNumbersTo(count))
		if err := pow.close(); err != nil {
			local.readPending = append(local.readPending, pow)
			local.err = errors.Join(local.err, err)
		}
	}()
	s := c.t.manager.NumeralService()
	pow.value, local.err = s.NewValue()
	pow.ready = local.err == nil
	if local.err != nil {
		return 0
	}
	local.err = s.SetInt64(&pow.value, 1)
	two := local.number()
	if local.err == nil {
		local.err = s.SetInt64(two, 2)
	}
	for local.err == nil {
		var less bool
		less, local.err = s.Less(&pow.value, n)
		if local.err != nil || !less {
			break
		}
		bits++
		local.err = s.MulAssignWithInvocation(&pow.value, two, &pow.invocation)
		if local.err == nil {
			local.err = pow.retire()
		}
	}
	if bits == 0 && local.err == nil {
		bits = 1
	}
	return bits
}

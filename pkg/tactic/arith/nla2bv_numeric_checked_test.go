//go:build g3checked

package arith

import (
	"math/big"
	"testing"
)

// Checked numeric descriptors require a fresh reservation for this fixture.
// Native descriptors can reuse existing pooled storage throughout the loop.
func TestNLA2BVProjectLog2CheckedMultiplicationReservation(t *testing.T) {
	r := nla2BVFixture(t, false)
	c, err := r.newInvocation()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, c.close()) })
	n := new(big.Rat).SetInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 65), big.NewInt(1)))
	input := c.local.rational(n)
	equationMust(t, c.local.err)
	r.manager.Budget().FailAfter(0)
	c.log2Owned(input)
	r.manager.Budget().FailAfter(-1)
	if c.local.err == nil {
		t.Fatal("project multiplication reservation was bypassed")
	}
}

//go:build g3checked

package arith

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Grow beyond the numeric pool's current storage so the checked test reaches
// a real allocation, rather than assuming a reservation at a small width.
func TestNLA2BVProjectLog2CheckedMultiplicationReservation(t *testing.T) {
	r := nla2BVFixture(t, false)
	c, err := r.newInvocation()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, c.close()) })
	n := new(big.Rat).SetInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 4097), big.NewInt(1)))
	input := c.local.rational(n)
	equationMust(t, c.local.err)
	r.manager.Budget().FailAfter(0)
	c.log2Owned(input)
	r.manager.Budget().FailAfter(-1)
	var failure umemory.Failure
	if !errors.Is(c.local.err, arena.ErrInjectedFailure) && (!errors.As(c.local.err, &failure) || failure.Kind != umemory.FailureMemout) {
		t.Fatal("expected an injected allocation failure", c.local.err)
	}
	actual, err := r.manager.NumeralService().String(input)
	equationMust(t, err)
	if actual != n.RatString() {
		t.Fatal("failed multiplication changed its input")
	}
}

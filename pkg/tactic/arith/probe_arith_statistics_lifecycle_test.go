package arith

// Source: src/tactic/probe.h retained virtual/ref-count boundary.
import (
	"errors"
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func TestArithmeticStatisticProbeLifecycleAndComposition(t *testing.T) {
	_, m := fixDLFixture(t, false)
	p, e := NewArithmeticMaximumDegreeProbe(m.Budget())
	equationMust(t, e)
	composed, e := tactic.NewNotProbe(m.Budget(), p)
	equationMust(t, e)
	if p.refs != 1 {
		t.Fatal("composition did not retain child", p.refs)
	}
	equationMust(t, composed.Close())
	if !p.closed {
		t.Fatal("last composed reference did not close child")
	}
	if e = p.Check(); !errors.Is(e, tactic.ErrRegistryOwner) {
		t.Fatal("closed probe accepted", e)
	}
}

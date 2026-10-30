package arith

import "testing"

func TestFMEliminateOneSidedVariable(t *testing.T) {
	store, v := fmResolutionFixture(t)
	x := v.Add(false, false)
	c := fmSubsumptionConstraint(nil, []uint32{x}, []int64{-1}, 2, false)
	c.ID = 9
	v.Register(c)
	e, err := NewFMEliminator(store, v, DefaultFMEliminationConfig())
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, e.Close()) })
	eliminated, err := e.Try(x)
	equationMust(t, err)
	if !eliminated || !c.Dead {
		t.Fatal("one-sided variable was not eliminated")
	}
}

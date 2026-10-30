package arith

import "testing"

func TestFMConstraintQueries(t *testing.T) {
	c := &FMConstraint{Variables: []uint32{3, 8}}
	if !c.Live() {
		t.Fatal("fresh constraint is not live")
	}
	if p, ok := c.VariablePosition(8); !ok || p != 1 {
		t.Fatalf("position = %d, %v", p, ok)
	}
	if _, ok := c.VariablePosition(7); ok {
		t.Fatal("absent variable reported present")
	}
	c.Dead = true
	if c.Live() {
		t.Fatal("dead constraint reported live")
	}
}

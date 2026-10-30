package arith

import "testing"

func TestFMVariableTableClassification(t *testing.T) {
	v := NewFMVariableTable()
	r := v.Add(false, false)
	i := v.Add(true, false)
	f := v.Add(true, true)
	if r != 0 || i != 1 || f != 2 || v.IsInteger(r) || !v.IsInteger(i) || !v.IsForbidden(f) || v.IsForbidden(i) {
		t.Fatal("variable metadata was not retained")
	}
	c := &FMConstraint{Variables: []uint32{i, f}}
	if !v.AllInteger(c) {
		t.Fatal("integer constraint was classified as mixed")
	}
	c.Variables = append(c.Variables, r)
	if v.AllInteger(c) {
		t.Fatal("mixed constraint was classified as integer")
	}
}

package arith

import (
	"reflect"
	"testing"
)

func TestFMCandidatesSourceOrdering(t *testing.T) {
	v := NewFMVariableTable()
	zeroInt := v.Add(true, false)
	zeroReal := v.Add(false, false)
	real := v.Add(false, false)
	integer := v.Add(true, false)
	v.Add(false, true)
	c := &FMConstraint{}
	v.Index.Lower[real] = []*FMConstraint{c, c}
	v.Index.Upper[real] = []*FMConstraint{c}
	v.Index.Lower[integer] = []*FMConstraint{c}
	v.Index.Upper[integer] = []*FMConstraint{c}
	want := []uint32{zeroInt, zeroReal, real, integer}
	if got := v.Candidates(); !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate order = %v, want %v", got, want)
	}
}

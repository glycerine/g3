package arith

import "testing"

func TestFMConstraintIndexDirectionAndBackwardSubsumption(t *testing.T) {
	i := NewFMConstraintIndex()
	strong := fmSubsumptionConstraint(nil, []uint32{4}, []int64{-1}, 3, false)
	strong.ID = 1
	weak := fmSubsumptionConstraint(nil, []uint32{4}, []int64{-1}, 8, false)
	weak.ID = 2
	i.Add(weak)
	i.Add(strong)
	if len(i.Lower[4]) != 2 || len(i.Upper[4]) != 0 || i.Todo.Len() != 2 {
		t.Fatal("coefficient sign was indexed in the wrong direction")
	}
	i.Subsume()
	if !weak.Dead || strong.Dead || len(i.Lower[4]) != 1 || i.Lower[4][0] != strong {
		t.Fatal("backward subsumption did not compact the selected index")
	}
}

func TestFMConstraintIndexForbiddenVariable(t *testing.T) {
	i := NewFMConstraintIndex()
	i.Forbidden[9] = true
	c := fmSubsumptionConstraint(nil, []uint32{9}, []int64{1}, 0, false)
	i.Add(c)
	if len(i.Upper[9]) != 0 || i.Todo.Len() != 1 {
		t.Fatal("forbidden variable was indexed")
	}
}

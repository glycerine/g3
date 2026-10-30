package arith

import "testing"

func TestFMConstraintCleanupAndLiveCopy(t *testing.T) {
	v := NewFMVariableTable()
	x := v.Add(false, false)
	live := &FMConstraint{}
	dead := &FMConstraint{Dead: true}
	v.Index.Lower[x] = []*FMConstraint{dead, live, nil}
	v.Index.Upper[x] = []*FMConstraint{live, dead}
	copy := v.LiveBounds(x, true)
	if len(copy) != 1 || copy[0] != live || len(v.Index.Lower[x]) != 3 {
		t.Fatal("live copy changed or retained dead bounds")
	}
	v.Cleanup(x)
	if len(v.Index.Lower[x]) != 1 || len(v.Index.Upper[x]) != 1 || v.Index.Lower[x][0] != live {
		t.Fatal("cleanup did not compact bounds")
	}
}

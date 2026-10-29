package rel

import "testing"

func TestCheckedRelationUnionDeltaAndMismatchDetection(t *testing.T) {
	s, _ := NewSignature([]uint64{5})
	dst, _ := NewCheckedRelation(s)
	src, _ := NewCheckedRelation(s)
	delta, _ := NewCheckedRelation(s)
	src.Add(Fact{1})
	src.Add(Fact{4})
	must(t, dst.Union(src, delta))
	if n, _ := delta.Len(); n != 2 {
		t.Fatal(n)
	}
	dst.checker.Reset()
	if e := dst.Consistent(); e == nil {
		t.Fatal("mismatch not detected")
	}
}

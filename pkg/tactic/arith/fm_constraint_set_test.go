package arith

import "testing"

func TestFMConstraintSetDensePositions(t *testing.T) {
	a, b, c := &FMConstraint{ID: 2}, &FMConstraint{ID: 7}, &FMConstraint{ID: 4}
	var s FMConstraintSet
	s.Insert(a)
	s.Insert(b)
	s.Insert(c)
	s.Insert(b)
	if s.Len() != 3 || !s.Contains(a) || !s.Contains(b) || !s.Contains(c) {
		t.Fatalf("bad insertion state: len=%d", s.Len())
	}
	s.Erase(a) // exercises the moved-last position repair
	if s.Contains(a) || !s.Contains(c) || s.Len() != 2 {
		t.Fatal("erase did not repair dense positions")
	}
	if got := s.Pop(); got == nil || s.Contains(got) || s.Len() != 1 {
		t.Fatal("pop did not clear membership")
	}
	s.Reset()
	if s.Len() != 0 || s.Contains(b) {
		t.Fatal("reset retained membership")
	}
}

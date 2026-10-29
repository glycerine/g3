package rel

import "testing"

func TestSparseCloneResetRemoveClose(t *testing.T) {
	s, _ := NewSignature([]uint64{8})
	x, _ := NewSparseTable(s, 0)
	x.Add(Fact{1})
	x.Add(Fact{7})
	c, e := x.Clone()
	must(t, e)
	removed, e := c.Remove(Fact{1})
	must(t, e)
	if !removed {
		t.Fatal()
	}
	if n, _ := x.Len(); n != 2 {
		t.Fatal(n)
	}
	must(t, c.Reset())
	if n, _ := c.Len(); n != 0 {
		t.Fatal(n)
	}
	must(t, c.Close())
	must(t, c.Close())
	if _, e = c.Len(); e == nil {
		t.Fatal("closed table")
	}
}

package rel

import "testing"

func TestCheckedRelationMutationCloneAndLifecycle(t *testing.T) {
	s, _ := NewSignature([]uint64{4, 3})
	r, e := NewCheckedRelation(s)
	must(t, e)
	added, e := r.Add(Fact{3, 2})
	must(t, e)
	if !added {
		t.Fatal()
	}
	ok, e := r.Contains(Fact{3, 2})
	must(t, e)
	if !ok {
		t.Fatal()
	}
	c, e := r.Clone()
	must(t, e)
	removed, e := c.Remove(Fact{3, 2})
	must(t, e)
	if !removed {
		t.Fatal()
	}
	if n, _ := r.Len(); n != 1 {
		t.Fatal(n)
	}
	must(t, c.Close())
	must(t, c.Close())
	if _, e = c.Len(); e == nil {
		t.Fatal("closed")
	}
}

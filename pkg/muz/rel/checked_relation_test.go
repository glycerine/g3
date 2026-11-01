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
	primary, checker := r.primary, r.checker
	must(t, r.Close())
	if _, e = primary.Len(); e == nil {
		t.Fatal("primary table remains open after closing its relation")
	}
	if _, e = checker.Len(); e == nil {
		t.Fatal("checker table remains open after closing its relation")
	}
	if _, e = r.SelectEqualAndProject(0, 0); e == nil {
		t.Fatal("filter-project accepted a closed relation")
	}
}

package rel

import "testing"

func TestCheckedRelationJoinProjectRenameAndFilters(t *testing.T) {
	s, _ := NewSignature([]uint64{4, 4})
	a, _ := NewCheckedRelation(s)
	b, _ := NewCheckedRelation(s)
	a.Add(Fact{1, 2})
	a.Add(Fact{3, 1})
	b.Add(Fact{2, 0})
	b.Add(Fact{1, 3})
	j, e := a.JoinProject(b, []int{1}, []int{0}, []int{1, 2})
	must(t, e)
	if n, _ := j.Len(); n != 2 {
		t.Fatal(n)
	}
	r, e := a.Rename([]int{0, 1})
	must(t, e)
	ok, e := r.Contains(Fact{2, 1})
	must(t, e)
	if !ok {
		t.Fatal()
	}
	must(t, a.FilterNegation(b, []int{1}, []int{0}))
	if n, _ := a.Len(); n != 0 {
		t.Fatal(n)
	}
}

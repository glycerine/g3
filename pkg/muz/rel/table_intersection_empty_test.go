package rel

import "testing"

func TestHashTableIntersectionEmptyAndCartesianCases(t *testing.T) {
	target := intersectionTestTable(t, []uint64{2}, Fact{0}, Fact{1})
	empty := intersectionTestTable(t, []uint64{2})
	if err := target.FilterByIntersection(empty, nil, nil); err != nil || target.rows.Len() != 0 {
		t.Fatal("empty source", target.rows.Len(), err)
	}
	target = intersectionTestTable(t, []uint64{2}, Fact{0}, Fact{1})
	nonempty := intersectionTestTable(t, []uint64{3}, Fact{2})
	if err := target.FilterByIntersection(nonempty, nil, nil); err != nil || target.rows.Len() != 2 {
		t.Fatal("nonempty Cartesian source", target.rows.Len(), err)
	}
}

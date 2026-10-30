package realclosure

import "testing"

func TestManagerIntScalarRelations(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	value := f.rational(3)
	for _, tc := range []struct {
		n                       int64
		eq, neq, lt, gt, le, ge bool
	}{{3, true, false, false, false, true, true}, {4, false, true, true, false, true, false}, {2, false, true, false, true, false, true}} {
		eq, e0 := f.store.equalValueInt64(value, tc.n)
		neq, e1 := f.store.notEqualValueInt64(value, tc.n)
		lt, e2 := f.store.lessValueInt64(value, tc.n)
		gt, e3 := f.store.greaterValueInt64(value, tc.n)
		le, e4 := f.store.lessEqualValueInt64(value, tc.n)
		ge, e5 := f.store.greaterEqualValueInt64(value, tc.n)
		if e0 != nil || e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || eq != tc.eq || neq != tc.neq || lt != tc.lt || gt != tc.gt || le != tc.le || ge != tc.ge {
			t.Fatal(tc.n, eq, neq, lt, gt, le, ge, e0, e1, e2, e3, e4, e5)
		}
	}
}

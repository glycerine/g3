package interval

import "testing"

func TestModularIntervalSmallIntersectionBranches(t *testing.T) {
	result := &ModularInterval{}
	for _, tc := range []struct {
		a, b, lo, hi          string
		at, bt, wantTight, ok bool
	}{
		{"2", "10", "5", "8", true, true, true, true},
		{"2", "4", "7", "9", true, true, false, false},
		{"240", "15", "3", "8", true, true, false, true},
		{"240", "15", "250", "4", true, true, false, true},
		{"240", "15", "230", "20", true, true, false, true},
	} {
		a, b := modularTestInterval(t, tc.a, tc.b, 8, tc.at), modularTestInterval(t, tc.lo, tc.hi, 8, tc.bt)
		ok, err := a.Intersect(b, result)
		if err != nil || ok != tc.ok {
			t.Fatal(tc, ok, err)
		}
		if ok && !result.Invariant() {
			t.Fatal("invalid result", tc, result.Lower(), result.Upper())
		}
	}
	a := modularTestInterval(t, "2", "10", 8, true)
	b := modularTestInterval(t, "5", "8", 8, true)
	ok, err := a.Intersect(b, result)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	requireModularInterval(t, result, "5", "8", true)
}

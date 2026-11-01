package api

import "testing"

func TestAPIRCFRationalRelations(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	a, b, same := f.rational("1/3"), f.rational("2/3"), f.rational("1/3")
	for _, tc := range []struct {
		name  string
		left  *RCFNumber
		right *RCFNumber
		want  [6]bool // less, greater, less-equal, greater-equal, equal, not-equal
	}{
		{"less", a, b, [6]bool{true, false, true, false, false, true}},
		{"equal", a, same, [6]bool{false, false, true, true, true, false}},
		{"greater", b, a, [6]bool{false, true, false, true, false, true}},
	} {
		got := [6]bool{}
		var err error
		got[0], err = f.context.RCFLess(tc.left, tc.right)
		if err != nil {
			t.Fatal(tc.name, "less:", err)
		}
		got[1], err = f.context.RCFGreater(tc.left, tc.right)
		if err != nil {
			t.Fatal(tc.name, "greater:", err)
		}
		got[2], err = f.context.RCFLessEqual(tc.left, tc.right)
		if err != nil {
			t.Fatal(tc.name, "less-equal:", err)
		}
		got[3], err = f.context.RCFGreaterEqual(tc.left, tc.right)
		if err != nil {
			t.Fatal(tc.name, "greater-equal:", err)
		}
		got[4], err = f.context.RCFEqual(tc.left, tc.right)
		if err != nil {
			t.Fatal(tc.name, "equal:", err)
		}
		got[5], err = f.context.RCFNotEqual(tc.left, tc.right)
		if err != nil {
			t.Fatal(tc.name, "not-equal:", err)
		}
		if got != tc.want {
			t.Fatalf("%s comparisons got %v want %v", tc.name, got, tc.want)
		}
	}
}

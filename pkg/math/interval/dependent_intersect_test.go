package interval

import (
	"reflect"
	"testing"
)

func TestDependentIntervalIntersectionTieBreakAndAliasing(t *testing.T) {
	f := newDependentIntervalFixture(t)
	a := f.interval("1", "5", false, false, 1, 2)
	b := f.interval("1", "4", true, true, 3, 4)
	if err := a.Intersect(a, b, true); err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, a); got != "(1,4) ld 3 ud 4" {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(a.LowerDependencies(), []uint32{3}) || !reflect.DeepEqual(a.UpperDependencies(), []uint32{4}) {
		t.Fatal(a.LowerDependencies(), a.UpperDependencies())
	}
	if empty, err := a.IsEmpty(); err != nil || empty {
		t.Fatal(empty, err)
	}
}

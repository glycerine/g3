package interval

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDependentIntervalSetPredicatesConflictAndDisplay(t *testing.T) {
	f := newDependentIntervalFixture(t)
	v := f.interval("0", "3", true, false, 7, 8)
	separated, err := v.SeparatedFromZeroLower()
	if err != nil || !separated {
		t.Fatal(separated, err)
	}
	initial := f.manager.Leaf(6)
	expl, conflict, err := v.ZeroConflict(initial)
	if err != nil || !conflict || !reflect.DeepEqual(expl, []uint32{6, 7}) {
		t.Fatal(expl, conflict, err)
	}
	var out bytes.Buffer
	if err = v.Display(&out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "(0,3] ld 7 ud 8" {
		t.Fatal(got)
	}
	shift := f.rational("-4")
	if err = v.AddScalar(&shift); err != nil {
		t.Fatal(err)
	}
	zero := f.rational("0")
	if below, err := v.IsBelow(&zero); err != nil || !below {
		t.Fatal(below, err)
	}
	if !reflect.DeepEqual(v.LowerDependencies(), []uint32{7}) {
		t.Fatal(v.LowerDependencies())
	}
}

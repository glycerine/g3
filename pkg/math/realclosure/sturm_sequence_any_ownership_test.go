package realclosure

import "testing"

func TestAnySturmSequenceOwnsAndReleasesRows(t *testing.T) {
	f := newAnySturmFixture(t)
	tracked := []*realValue{f.polynomial[0], f.polynomial[1], f.polynomial[2]}
	before := make([]uint32, len(tracked))
	for i, value := range tracked {
		before[i] = value.refs
	}
	rows, err := f.base.store.rootSturmSequenceAny(f.polynomial, true)
	if err != nil {
		t.Fatal(err)
	}
	for i, value := range tracked {
		if value.refs <= before[i] {
			t.Fatal("sequence did not retain source coefficient", i, before[i], value.refs)
		}
	}
	if err := f.base.store.closePolynomialSequence(rows); err != nil {
		t.Fatal(err)
	}
	for i, value := range tracked {
		if value.refs != before[i] {
			t.Fatal("sequence retained coefficient after close", i, before[i], value.refs)
		}
	}
}

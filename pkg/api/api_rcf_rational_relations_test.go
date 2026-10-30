package api

import "testing"

func TestAPIRCFRationalRelations(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	a, b, same := f.rational("1/3"), f.rational("2/3"), f.rational("1/3")
	if yes, err := f.context.RCFLess(a, b); err != nil || !yes {
		t.Fatal(yes, err)
	}
	if yes, err := f.context.RCFGreater(b, a); err != nil || !yes {
		t.Fatal(yes, err)
	}
	if yes, err := f.context.RCFEqual(a, same); err != nil || !yes {
		t.Fatal(yes, err)
	}
	if yes, err := f.context.RCFNotEqual(a, b); err != nil || !yes {
		t.Fatal(yes, err)
	}
}

package bv

import "testing"

func TestBVBoundsMaySimplifyNumeralAndSingleBound(t *testing.T) {
	f := newBVBoundsFixture(t)
	may, err := f.simp.MaySimplify(f.num(t, 9))
	sharingMust(t, err)
	if may {
		t.Fatal("numeral should use source fast rejection")
	}
	may, err = f.simp.MaySimplify(f.ule(t, f.x, f.num(t, 24)))
	sharingMust(t, err)
	if may {
		t.Fatal("single unconstrained proper bound should be skipped")
	}
	may, err = f.simp.MaySimplify(f.ule(t, f.num(t, 0), f.x))
	sharingMust(t, err)
	if !may {
		t.Fatal("full source bound should be visited")
	}
}

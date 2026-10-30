package bv

import "testing"

func TestBVBoundsMaySimplifyContextAndNegation(t *testing.T) {
	f := newBVBoundsFixture(t)
	lower := f.ule(t, f.num(t, 4), f.x)
	ok, err := f.simp.AssertExpression(lower, false)
	sharingMust(t, err)
	if !ok {
		t.Fatal("context bound rejected")
	}
	upper := f.ule(t, f.x, f.num(t, 200))
	not, err := f.m.MkNot(upper)
	sharingMust(t, err)
	f.pin(t, not)
	may, err := f.simp.MaySimplify(not)
	sharingMust(t, err)
	if !may {
		t.Fatal("negated contextual bound was skipped")
	}
	and, err := f.m.MkAnd(upper, upper)
	sharingMust(t, err)
	f.pin(t, and)
	may, err = f.simp.MaySimplify(and)
	sharingMust(t, err)
	if !may {
		t.Fatal("nested contextual bounds were skipped")
	}
}

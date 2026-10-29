package api

import "testing"

func TestAPIAlgebraicNumberBoundsEncloseRoot(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	root := f.sqrt2(t)
	lower := f.pin(f.c.AlgebraicNumberLower(root, 8))
	upper := f.pin(f.c.AlgebraicNumberUpper(root, 8))
	below, err := f.c.AlgebraicLT(lower, root)
	astTestMust(t, err)
	above, err := f.c.AlgebraicLT(root, upper)
	astTestMust(t, err)
	if !below || !above {
		t.Fatal("bounds do not strictly enclose sqrt(2)", below, above)
	}
}

package realclosure

import "testing"

func TestAlgebraicRegistryReusesDiscardedUnpublishedIndex(t *testing.T) {
	f := newRankedValueFixture(t, true)
	owner := f.alpha.root.owner
	root, err := owner.mkAlgebraic(f.alpha.root.polynomial.coefficients, f.alpha.root.interval, f.alpha.root.isolating, nil, -1)
	if err != nil {
		t.Fatal(err)
	}
	index := root.index
	if err := root.discardUnpublished(); err != nil {
		t.Fatal(err)
	}
	replacement := newRegistryTestValue(t, f)
	if replacement.root.index != index {
		t.Fatal(index, replacement.root.index)
	}
}

package realclosure

import "testing"

func newRegistryTestValue(t *testing.T, f *rankedValueFixture) *realValue {
	t.Helper()
	owner := f.alpha.root.owner
	root, err := owner.mkAlgebraic(f.alpha.root.polynomial.coefficients, f.alpha.root.interval, f.alpha.root.isolating, nil, -1)
	if err != nil {
		t.Fatal(err)
	}
	value, err := f.base.store.newAlgebraicValue(root, f.one)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !value.deleted {
			if err := f.base.store.release(value); err != nil {
				t.Error(err)
			}
		}
	})
	return value
}

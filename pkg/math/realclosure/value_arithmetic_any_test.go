package realclosure

import "testing"

func TestAnyCanonicalArithmeticOnSharedRoots(t *testing.T) {
	fixture := newMultiRootExactFixture(t, true)
	zeroes := 0
	for i, root := range fixture.roots {
		difference, err := fixture.f.store.subtractAnyCanonicalValues(root, fixture.two, true)
		if err != nil {
			t.Fatal(err)
		}
		if difference == nil {
			zeroes++
			continue
		}
		sign, err := fixture.f.store.signValue(difference)
		if err != nil || sign == 0 {
			t.Fatal(i, sign, err)
		}
		product, err := fixture.f.store.multiplyAnyCanonicalValues(difference, difference, true)
		if err != nil {
			t.Fatal(err)
		}
		productSign, err := fixture.f.store.signValue(product)
		if err != nil || productSign != 1 {
			t.Fatal(productSign, err)
		}
		if err := fixture.f.store.release(product); err != nil {
			t.Fatal(err)
		}
		if err := fixture.f.store.release(difference); err != nil {
			t.Fatal(err)
		}
	}
	if zeroes != 1 {
		t.Fatal(zeroes)
	}
}

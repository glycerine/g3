package realclosure

import (
	"sort"
	"testing"
)

func TestExactMultiRootPolynomialSigns(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		fixture := newMultiRootExactFixture(t, usePrem)
		signs := make([]int, 0, 3)
		for _, value := range fixture.values {
			sign, err := fixture.f.store.exactMultiRootPolynomialSign(value, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			signs = append(signs, int(sign))
		}
		sort.Ints(signs)
		if signs[0] != -1 || signs[1] != 0 || signs[2] != 1 {
			t.Fatal(signs)
		}
	}
}

package realclosure

import (
	"sort"
	"testing"
)

func TestDetermineAnyRetainedValueSignAndCanonicalZero(t *testing.T) {
	fixture := newMultiRootExactFixture(t, true)
	signs := make([]int, 0, 3)
	zeroIndex := -1
	for i, value := range fixture.values {
		sign, err := fixture.f.store.determineAnyRetainedValueSign(value, true)
		if err != nil {
			t.Fatal(err)
		}
		signs = append(signs, int(sign))
		if sign == 0 {
			zeroIndex = i
		} else {
			contains, err := value.functionInterval.ContainsZero()
			if err != nil || contains {
				t.Fatal(contains, err)
			}
		}
	}
	sort.Ints(signs)
	if signs[0] != -1 || signs[1] != 0 || signs[2] != 1 || zeroIndex < 0 {
		t.Fatal(signs, zeroIndex)
	}
	canonical, err := fixture.f.store.canonicalizeAnyRetainedResult(fixture.values[zeroIndex], true)
	if err != nil || canonical != nil {
		t.Fatal(canonical, err)
	}
	fixture.values[zeroIndex] = nil
}

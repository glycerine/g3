package interval

import "testing"

func TestModularIntervalNegationComplements(t *testing.T) {
	for _, size := range []uint32{8, 80} {
		bound := modularBound(size).String()
		result := &ModularInterval{}
		loZero := modularTestInterval(t, "0", "10", size, true)
		ok, err := loZero.Negate(result)
		if err != nil || !ok {
			t.Fatal(size, ok, err)
		}
		requireModularInterval(t, result, "11", bound, false)
		hiBound := modularTestInterval(t, "10", bound, size, true)
		ok, err = hiBound.Negate(result)
		if err != nil || !ok {
			t.Fatal(size, ok, err)
		}
		requireModularInterval(t, result, "0", "9", false)
		middle := modularTestInterval(t, "10", "20", size, true)
		ok, err = middle.Negate(result)
		if err != nil || !ok {
			t.Fatal(size, ok, err)
		}
		requireModularInterval(t, result, "21", "9", false)
		loose := modularTestInterval(t, "10", "20", size, false)
		ok, err = loose.Negate(result)
		if err != nil || !ok {
			t.Fatal(size, ok, err)
		}
		requireModularInterval(t, result, "0", bound, true)
		full := modularTestInterval(t, "0", bound, size, true)
		ok, err = full.Negate(result)
		if err != nil || ok {
			t.Fatal(size, ok, err)
		}
	}
}

package interval

import (
	"math/big"
	"testing"
)

func modularTestValue(t *testing.T, text string) *big.Int {
	t.Helper()
	v, ok := new(big.Int).SetString(text, 10)
	if !ok {
		t.Fatal(text)
	}
	return v
}
func modularTestInterval(t *testing.T, lower, upper string, size uint32, tight bool) *ModularInterval {
	t.Helper()
	v, err := NewModularInterval(modularTestValue(t, lower), modularTestValue(t, upper), size, tight)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func requireModularInterval(t *testing.T, v *ModularInterval, lower, upper string, tight bool) {
	t.Helper()
	if v.Lower().String() != lower || v.Upper().String() != upper || v.Tight() != tight || !v.Invariant() {
		t.Fatal(v.Lower(), v.Upper(), v.Tight())
	}
}

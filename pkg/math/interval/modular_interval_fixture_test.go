package interval

import (
	"strconv"
	"testing"
)

func modularTestValue(t *testing.T, text string) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		t.Fatal(text, err)
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
	if strconv.FormatUint(v.Lower(), 10) != lower || strconv.FormatUint(v.Upper(), 10) != upper || v.Tight() != tight || !v.Invariant() {
		t.Fatal(v.Lower(), v.Upper(), v.Tight())
	}
}

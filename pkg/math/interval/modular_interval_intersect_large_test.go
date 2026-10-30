package interval

import "testing"

func TestModularIntervalLargeIntersectionAndCopies(t *testing.T) {
	// 2^80 - 1 = 1208925819614629174706175.
	full := modularTestInterval(t, "0", "1208925819614629174706175", 80, true)
	wrapped := modularTestInterval(t, "1208925819614629174706100", "75", 80, true)
	plain := modularTestInterval(t, "20", "60", 80, true)
	result := &ModularInterval{}
	if ok, err := full.Intersect(wrapped, result); err != nil || !ok {
		t.Fatal(ok, err)
	}
	requireModularInterval(t, result, "1208925819614629174706100", "75", true)
	if ok, err := wrapped.Intersect(plain, result); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if result.small {
		t.Fatal("lost large dispatch")
	}
	lo := result.Lower()
	lo.SetInt64(0)
	if result.Lower().Sign() == 0 {
		t.Fatal("returned lower aliases storage")
	}
}

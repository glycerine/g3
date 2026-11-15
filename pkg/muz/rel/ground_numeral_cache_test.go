package rel

import "testing"

// The interpreted-filter numeral cache returns the hash-consed numeral
// DecodeValue builds, once per (sort, value), and releases it on close.
func TestGroundNumeralCache(t *testing.T) {
	f := newTableRelationFixture(t)
	cache := newGroundNumeralCache(f.relations)
	first, cached, err := cache.numeral(f.bits, 2)
	if err != nil || !cached {
		t.Fatal(cached, err)
	}
	again, cached, err := cache.numeral(f.bits, 2)
	if err != nil || !cached || again != first || len(cache.refs) != 1 {
		t.Fatal("cache miss on a repeated numeral", cached, err, len(cache.refs))
	}
	reference, err := f.relations.DecodeValue(f.bits, 2)
	if err != nil {
		t.Fatal(err)
	}
	built, err := f.ast.RefGet(reference)
	if err != nil {
		t.Fatal(err)
	}
	if built != first {
		t.Fatal("cached numeral differs from DecodeValue's")
	}
	if err = f.ast.DelRef(&reference); err != nil {
		t.Fatal(err)
	}
	if _, _, err = cache.numeral(f.bits, 3); err != nil || len(cache.refs) != 2 {
		t.Fatal(err, len(cache.refs))
	}
	if err = cache.close(); err != nil {
		t.Fatal(err)
	}
	var none *groundNumeralCache
	if _, cached, err = none.numeral(f.bits, 1); cached || err != nil {
		t.Fatal("nil cache must defer to the caller", cached, err)
	}
}

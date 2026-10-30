package api

import "testing"

func TestSMTContextFiniteDomainSortAndNumeral(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort, err := f.c.FiniteDomainSort("api-finite-four", 4)
	astTestMust(t, err)
	size, err := f.c.FiniteDomainSortSize(sort)
	astTestMust(t, err)
	if size != 4 {
		t.Fatal("finite-domain size", size)
	}
	value := f.pin(f.c.UintNumeral(3, sort))
	number, matched, err := f.m.DatalogNumeralValue(value)
	astTestMust(t, err)
	if !matched || number != 3 {
		t.Fatal("finite-domain numeral", number, matched)
	}
}

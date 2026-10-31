package arena

import (
	"errors"
	"regexp"
	"testing"
)

func TestSmallBytesDiagnosticAddressIdentityAndLifetime(t *testing.T) {
	a := newSmallTest(t, 64, NewBudget(1<<20))
	if address, err := a.AddressText(SmallBytesRef{}); err != nil || address != "0" {
		t.Fatal(address, err)
	}
	r := smallAlloc(t, a, 16)
	address, err := a.AddressText(r)
	if err != nil || !regexp.MustCompile(`^0x[0-9a-f]+$`).MatchString(address) {
		t.Fatal(address, err)
	}
	other := smallAlloc(t, a, 16)
	second, err := a.AddressText(other)
	if err != nil || second == address {
		t.Fatal("live allocations share address", address, second, err)
	}
	// Growing the directory and consolidating empty chunks cannot move live data.
	var many []SmallBytesRef
	for i := 0; i < 24; i++ {
		many = append(many, smallAlloc(t, a, 16))
	}
	for _, ref := range many {
		smallFree(t, a, 16, ref)
	}
	if err = a.Consolidate(); err != nil {
		t.Fatal(err)
	}
	if after, err := a.AddressText(r); err != nil || after != address {
		t.Fatal("live address moved", after, address, err)
	}
	smallFree(t, a, 16, r)
	if _, err = a.AddressText(r); !errors.Is(err, ErrInvalidSlot) {
		t.Fatal("dead handle address", err)
	}
	a.Close()
	if _, err = a.AddressText(other); !errors.Is(err, ErrClosed) {
		t.Fatal("closed allocator address", err)
	}
}

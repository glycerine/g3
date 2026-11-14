package numeral

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// The validated digit path (ReadWord, writeWord, copyWordsFrom) must stop
// with ErrRuntimeUninitialized when a cell's backing names an allocator
// object that has no digit payload, as the byte runtime did for digits that
// were never written. math/polynomial's convert-bounds tests redirect a
// cell's backing to a header-only object exactly this way to make a digit
// read stop; after digits moved to native payloads the read silently
// returned another object's words (upolynomial.cpp:2833 bqm.eq(c, a)).
func TestIntegerDigitReadOnObjectWithoutPayloadStops(t *testing.T) {
	b := arena.NewBudget(1 << 24)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	a, e := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: r})
	if e != nil {
		t.Fatal(e)
	}
	c, e := allocateSourceCell(a, 6)
	if e != nil {
		t.Fatal(e)
	}
	if e = c.writeWord(a, 8, 7); e != nil {
		t.Fatal(e)
	}
	// A header-only query object, as the polynomial tests build it: the size
	// word is written, the digits never were.
	query, e := a.Allocate(4096)
	if e != nil {
		t.Fatal(e)
	}
	raw, offset, e := a.RawLocation(query)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.WriteUint32(raw, offset, 1); e != nil {
		t.Fatal(e)
	}
	redirected := c
	redirected.Object, redirected.Raw, redirected.Offset = query, raw, offset
	if v, e := redirected.ReadWord(a, 0); e != nil || v != 1 {
		t.Fatal("redirected size header", v, e)
	}
	if _, e := redirected.ReadWord(a, 8); !errors.Is(e, umemory.ErrRuntimeUninitialized) {
		t.Fatalf("digit read on an object without a payload: %v", e)
	}
	if e := redirected.writeWord(a, 8, 5); !errors.Is(e, umemory.ErrRuntimeUninitialized) {
		t.Fatalf("digit write on an object without a payload: %v", e)
	}
	other, e := allocateSourceCell(a, 6)
	if e != nil {
		t.Fatal(e)
	}
	if e := other.copyWordsFrom(a, redirected, a, 8, 1); !errors.Is(e, umemory.ErrRuntimeUninitialized) {
		t.Fatalf("copy from an object without a payload: %v", e)
	}
	if e := redirected.copyWordsFrom(a, c, a, 8, 1); !errors.Is(e, umemory.ErrRuntimeUninitialized) {
		t.Fatalf("copy into an object without a payload: %v", e)
	}
	// The real cell is untouched by all of the above.
	if v, e := c.ReadWord(a, 8); e != nil || v != 7 {
		t.Fatal("original digit", v, e)
	}
	if v, e := other.ReadWord(a, 8); e != nil || v != 0 {
		t.Fatal("copy target digit after refused copy", v, e)
	}
	if numericValidateCells {
		// Checked builds route limb views through the validated path too.
		func() {
			defer func() {
				if p := recover(); p == nil {
					t.Fatal("checked limb read on an object without a payload did not stop")
				} else if e, ok := p.(error); !ok || !errors.Is(e, umemory.ErrRuntimeUninitialized) {
					t.Fatalf("checked limb read stop: %v", p)
				}
			}()
			limbView{allocator: a, backing: redirected, size: 1}.read(0)
		}()
	}
	if e = a.Deallocate(4096, query); e != nil {
		t.Fatal(e)
	}
	if e = a.Deallocate(c.Requested, c.Object); e != nil {
		t.Fatal(e)
	}
	if e = a.Deallocate(other.Requested, other.Object); e != nil {
		t.Fatal(e)
	}
}

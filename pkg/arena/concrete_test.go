package arena

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestSlab32ConcreteLifecycle(t *testing.T) {
	budget := NewBudget(4096)
	s, e := NewSlab32(Config{PageSize: 1, SlotLimit: 1, Budget: budget})
	if e != nil {
		t.Fatal(e)
	}
	a, g, e := s.Allocate()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Write(a, math.MaxUint32); e != nil {
		t.Fatal(e)
	}
	if x, e := s.Read(a); e != nil || x != math.MaxUint32 {
		t.Fatal(x, e)
	}
	if _, _, e = s.Allocate(); !errors.Is(e, ErrSlotLimit) {
		t.Fatal(e)
	}
	s.Reset()
	s.TrimFreePages()
	if s.IsLive(a, g) {
		t.Fatal("reset token live")
	}
	budget.FailAfter(0)
	before := s.Stats()
	if _, _, e = s.Allocate(); !errors.Is(e, ErrInjectedFailure) {
		t.Fatal(e)
	}
	if s.Stats() != before {
		t.Fatal("failed recovery changed state")
	}
	budget.FailAfter(-1)
	b, h, e := s.Allocate()
	if e != nil || b != a || h <= g {
		t.Fatal(b, h, e)
	}
	s.pages[0].meta[0].generation = math.MaxUint32
	s.Recycle(b)
	if _, _, e = s.Allocate(); !errors.Is(e, ErrSlotLimit) {
		t.Fatal(e)
	}
	s.Close()
	if budget.Used() != 0 {
		t.Fatal(budget.Used())
	}
}

func TestSlab64ConcreteLifecycle(t *testing.T) {
	budget := NewBudget(4096)
	s, e := NewSlab64(Config{PageSize: 1, SlotLimit: 1, Budget: budget})
	if e != nil {
		t.Fatal(e)
	}
	a, g, e := s.Allocate()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Write(a, math.MaxUint64); e != nil {
		t.Fatal(e)
	}
	if x, e := s.Read(a); e != nil || x != math.MaxUint64 {
		t.Fatal(x, e)
	}
	if _, _, e = s.Allocate(); !errors.Is(e, ErrSlotLimit) {
		t.Fatal(e)
	}
	s.Reset()
	s.TrimFreePages()
	if s.IsLive(a, g) {
		t.Fatal("reset token live")
	}
	budget.FailAfter(0)
	before := s.Stats()
	if _, _, e = s.Allocate(); !errors.Is(e, ErrInjectedFailure) {
		t.Fatal(e)
	}
	if s.Stats() != before {
		t.Fatal("failed recovery changed state")
	}
	budget.FailAfter(-1)
	b, h, e := s.Allocate()
	if e != nil || b != a || h <= g {
		t.Fatal(b, h, e)
	}
	s.pages[0].meta[0].generation = math.MaxUint32
	s.Recycle(b)
	if _, _, e = s.Allocate(); !errors.Is(e, ErrSlotLimit) {
		t.Fatal(e)
	}
	s.Close()
	if budget.Used() != 0 {
		t.Fatal(budget.Used())
	}
}

func TestConcreteRecordAudit(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(uint32(0)), reflect.TypeOf(uint64(0)), reflect.TypeOf(record{}), reflect.TypeOf(vectorDescriptor{}), reflect.TypeOf(RegionRef{}), reflect.TypeOf(regionMarkState{})} {
		if e := ValidateRecordType(typ); e != nil {
			t.Fatal(typ, e)
		}
	}
	if e := ValidateRecordType(nil); !errors.Is(e, ErrPointerful) {
		t.Fatal(e)
	}
	if e := ValidateRecordType(reflect.TypeOf(RegionMark{})); !errors.Is(e, ErrPointerful) {
		t.Fatal("cold mark accepted as hot record", e)
	}
}

func TestVectors32SignedValues(t *testing.T) {
	v, _ := NewVectors32Signed(VectorConfig{PageSize: 2})
	defer v.Close()
	a, _ := v.Allocate(2, 2)
	v.Write(a, 0, math.MinInt32)
	v.Write(a, 1, -1)
	b, e := v.Reserve(a, 8)
	if e != nil || b != a {
		t.Fatal(b, e)
	}
	if x, e := v.Read(a, 0); e != nil || x != math.MinInt32 {
		t.Fatal(x, e)
	}
	if x, e := v.Read(a, 1); e != nil || x != -1 {
		t.Fatal(x, e)
	}
}

func TestRegion64PreservesWideValues(t *testing.T) {
	r, _ := NewRegion64(Config{PageSize: 1})
	defer r.Close(nil)
	m, _ := r.Mark()
	a, _ := r.Allocate(math.MaxUint64)
	if x, e := r.Read(a); e != nil || x != math.MaxUint64 {
		t.Fatal(x, e)
	}
	seen := false
	if e := r.Rollback(m, func(ref RegionRef, x uint64) {
		if ref != a || x != math.MaxUint64 {
			t.Fatal(ref, x)
		}
		seen = true
	}); e != nil || !seen {
		t.Fatal(e, seen)
	}
}

func TestDescriptorFreeLinkValidity(t *testing.T) {
	slab, err := newDescriptorSlab(Config{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer slab.Close()
	var refs [3]SmallBytesRef
	for i := range refs {
		slot, generation, err := slab.Allocate()
		if err != nil {
			t.Fatal(err)
		}
		refs[i] = SmallBytesRef{slot, generation}
	}
	for _, r := range refs {
		if _, ok := slab.readLive(r.Slot, 0); ok {
			t.Fatal("zero generation accepted")
		}
		if err := slab.Recycle(r.Slot); err != nil {
			t.Fatal(err)
		}
	}
	slab.TrimFreePages()
	for i := len(refs) - 1; i >= 0; i-- {
		r := refs[i]
		if _, ok := slab.readLive(r.Slot, r.Generation); ok {
			t.Fatal("free descriptor accepted")
		}
		slot, generation, err := slab.Allocate()
		if err != nil || slot != r.Slot || generation != r.Generation+1 {
			t.Fatal(slot, generation, err)
		}
		if _, ok := slab.readLive(r.Slot, r.Generation); ok {
			t.Fatal("old generation accepted after reuse")
		}
	}
}

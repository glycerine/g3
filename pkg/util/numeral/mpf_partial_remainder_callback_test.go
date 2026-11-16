package numeral

// Source: src/util/mpf.cpp:1297-1298 (actual quotient allocation transport).

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestArbitraryFloatPartialRemainderReturningTransportAndOwnerGuards(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	var m *ArbitraryFloatManager
	var x, y ArbitraryFloat
	armed, calls := false, 0
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error {
		if armed {
			calls++
			s := &m.partialRemainder
			if !s.executing {
				t.Fatal("transport missed partial remainder invocation")
			}
			xBefore, yBefore, valuesBefore, used := x, y, s.values, b.Used()
			if numeralContextRef(s.context()).roots(m.integer) != nil {
				t.Fatal("transport acquired roots")
			}
			copied := *s
			copied.hooks.Policy = &copied
			if numeralContextRef(copied.context()).roots(m.integer) != nil {
				t.Fatal("copied actual scope acquired roots")
			}
			for i, err := range []error{m.PartialRemainder(&x, &y, 0, false), m.CleanupPartialRemainder(), m.SourceFinalize(), m.Close(), m.integer.SourceFinalize(), m.integer.CloseChecked(), m.integer.SetInt32(&x.significand, 7), m.integer.Del(&x.significand)} {
				if !errors.Is(err, ErrNumericPending) {
					t.Fatal("actual owner/mutation guard", i, err)
				}
			}
			if x != xBefore || y != yBefore || s.values != valuesBefore || b.Used() != used {
				t.Fatal("transport changed stopped graph")
			}
		}
		return f
	}})
	serviceMust(t, e)
	m, e = NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	n := NewInt(1)
	serviceMust(t, m.integer.Mul2kInPlace(&n, 39999))
	serviceMust(t, m.SetComponents(&x, 17, 40000, false, 0, &n))
	serviceMust(t, m.SetComponents(&y, 17, 40000, false, 0, &n))
	serviceMust(t, m.integer.Del(&n))
	pad, e := r.Allocate(uint64(99990 - r.State().LocalSize - 8))
	serviceMust(t, e)
	serviceMust(t, r.SetMaxSize(1))
	armed = true
	body := m.PartialRemainder(&x, &y, 0, false)
	armed = false
	if body == nil || calls != 1 || !m.partialRemainder.active {
		t.Fatal("actual quotient transport missing", body, calls)
	}
	nums := []*Int{&x.significand, &y.significand}
	for i := range m.partialRemainder.values {
		nums = append(nums, &m.partialRemainder.values[i])
	}
	integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{pad}, nums...)
	if b.Used() != m.OwnedStorageBytes()+r.Stats().TotalBytes {
		t.Fatal("immediate transport physical graph", b.Used(), m.OwnedStorageBytes(), r.Stats())
	}
	serviceMust(t, r.SetMaxSize(1<<28))
	serviceMust(t, r.Deallocate(&pad))
	serviceMust(t, m.CleanupPartialRemainder())
	serviceMust(t, m.CleanupValue(&x))
	serviceMust(t, m.CleanupValue(&y))
	if e := m.Close(); e != nil {
		z := m.integer
		if !errors.Is(e, arena.ErrInvalidSlot) || !z.sourceGuard || z.SourceFinalizeStage < 2 || z.SourceFinalizeStage > 5 {
			t.Fatal("unexpected enclosing source stop", e, z.SourceFinalizeStage)
		}
		index := int(z.SourceFinalizeStage-2) / 2
		ref := z.Tmp[index]
		if z.SourceFinalizeStage%2 == 1 {
			ref = z.Arg[index]
		}
		if ref != 0 || m.partialRemainder.active || NumericAllocationPending(&m.partialRemainder.numeric) {
			t.Fatal("unproved scratch hole after owning join", ref)
		}
		serviceMust(t, m.Close())
	}
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("abandoned remainder refund", b.Used())
	}
}

package numeral

// Source: src/util/mpf.cpp:811-813; source returning-failure transport profile.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestArbitraryFloatFMAUnpackTransportAndOwnerGuards(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	var m *ArbitraryFloatManager
	var x, y, z, out ArbitraryFloat
	armed, calls := false, 0
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error {
		if armed {
			calls++
			s := &m.fma
			if !s.executing || m.valueExecuting {
				t.Fatal("transport did not reach the unpack arithmetic boundary")
			}
			xBefore, outBefore, localBefore, used := x, out, s.a, b.Used()
			if numeralContextRef(s.context()).roots(m.integer) != nil {
				t.Fatal("transport acquired allocating caller roots")
			}
			for index, err := range []error{m.FMA(ArbitraryFloatNearestEven, &x, &y, &z, &out), m.CleanupFMA(), m.SourceFinalize(), m.Close(), m.CleanupValue(&x), m.integer.SourceFinalize(), m.integer.CloseChecked(), m.integer.SetInt32(&x.significand, 7)} {
				if !errors.Is(err, ErrNumericPending) {
					t.Fatal("actual returning transport guard", index, err)
				}
			}
			if x != xBefore || out != outBefore || s.a != localBefore || b.Used() != used {
				t.Fatal("transport mutated stopped graph")
			}
		}
		return f
	}})
	serviceMust(t, e)
	m, e = NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	n := NewInt(0)
	serviceMust(t, m.SetComponents(&x, 17, 40000, false, 1, &n))
	serviceMust(t, m.SetComponents(&y, 17, 40000, true, -1, &n))
	serviceMust(t, m.SetComponents(&z, 17, 40000, false, -1, &n))
	_, e = m.IsNormal(&x)
	serviceMust(t, e)
	_, e = m.powerTwoWrapper(39999, false)
	serviceMust(t, e)
	// The input and copied local remain inline zero significands. The cached
	// hidden bit is BIG, so unpack's source ADD/SET must allocate the local.
	pad, e := r.Allocate(uint64(99990 - r.State().LocalSize - 8))
	serviceMust(t, e)
	serviceMust(t, r.SetMaxSize(1))
	armed = true
	body := m.FMA(ArbitraryFloatNearestEven, &x, &y, &z, &out)
	armed = false
	if body == nil || calls != 1 || !m.fma.active || m.fma.copyA != (ArbitraryFloatValueCopyReceipt{}) {
		t.Fatal("actual unpack allocation transport missing", body, calls)
	}
	s := &m.fma
	integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{pad}, &x.significand, &out.significand, &s.a.significand, &s.b.significand, &s.sticky, &y.significand, &z.significand, &s.mr.significand, &s.c.significand, &s.res.significand)
	if b.Used() != m.OwnedStorageBytes()+r.Stats().TotalBytes {
		t.Fatal("immediate unpack physical graph", b.Used(), m.OwnedStorageBytes(), r.Stats())
	}
	serviceMust(t, r.SetMaxSize(1<<28))
	serviceMust(t, r.Deallocate(&pad))
	// Let the enclosing owner join the abandoned inner arithmetic and local.
	if source := m.Close(); source != nil {
		z := m.integer
		if !errors.Is(source, arena.ErrInvalidSlot) || !z.sourceGuard || z.SourceFinalizeStage < 2 || z.SourceFinalizeStage > 5 {
			t.Fatal("unexpected enclosing source destructor stop", source, z.SourceFinalizeStage)
		}
		index := int(z.SourceFinalizeStage-2) / 2
		ref := z.Tmp[index]
		if z.SourceFinalizeStage%2 == 1 {
			ref = z.Arg[index]
		}
		if ref != 0 || m.fma.active || m.fma.aLive || NumericAllocationPending(&m.fma.numeric) {
			t.Fatal("source stop was not the proved scratch hole after inner owning cleanup", ref)
		}
		serviceMust(t, m.Close())
	}
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("abandoned unpack scope owner end", b.Used())
	}
}

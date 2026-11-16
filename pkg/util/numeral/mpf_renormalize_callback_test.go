package numeral

// Source: src/util/mpf.cpp:1259 (real cached-power allocation transport).

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestArbitraryFloatRenormalizeReturningTransportAndOwnerGuards(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	var m *ArbitraryFloatManager
	sig, exp := NewInt(1), int64(7)
	armed, calls := false, 0
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error {
		if armed {
			calls++
			s := &m.renormalize
			if !s.executing {
				t.Fatal("transport missed renormalize invocation")
			}
			used := b.Used()
			if numeralContextRef(s.context()).roots(m.integer) != nil {
				t.Fatal("transport acquired roots")
			}
			copied := *s
			copied.hooks.Policy = &copied
			if numeralContextRef(copied.context()).roots(m.integer) != nil {
				t.Fatal("copied actual scope acquired roots")
			}
			for i, err := range []error{m.Renormalize(8, 40000, &exp, &sig), m.CleanupRenormalize(), m.SourceFinalize(), m.Close(), m.integer.SourceFinalize(), m.integer.CloseChecked(), m.integer.SetInt32(&sig, 7), m.integer.Del(&sig)} {
				if !errors.Is(err, ErrNumericPending) {
					t.Fatal("actual owner/mutation guard", i, err)
				}
			}
			if sig != NewInt(1) || exp != 7 || b.Used() != used {
				t.Fatal("transport changed stopped graph")
			}
		}
		return f
	}})
	serviceMust(t, e)
	m, e = NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	pad, e := r.Allocate(uint64(99990 - r.State().LocalSize - 8))
	serviceMust(t, e)
	serviceMust(t, r.SetMaxSize(1))
	armed = true
	body := m.Renormalize(8, 40000, &exp, &sig)
	armed = false
	if body == nil || calls != 1 || !m.renormalize.active {
		t.Fatal("actual cached-power transport missing", body, calls)
	}
	integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{pad}, &sig)
	if b.Used() != m.OwnedStorageBytes()+r.Stats().TotalBytes {
		t.Fatal("immediate transport physical graph", b.Used(), m.OwnedStorageBytes(), r.Stats())
	}
	serviceMust(t, r.SetMaxSize(1<<28))
	serviceMust(t, r.Deallocate(&pad))
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
		if ref != 0 || m.renormalize.active || NumericAllocationPending(&m.renormalize.numeric) {
			t.Fatal("unproved scratch hole after owning join", ref)
		}
		serviceMust(t, m.Close())
	}
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("abandoned renormalize refund", b.Used())
	}
}

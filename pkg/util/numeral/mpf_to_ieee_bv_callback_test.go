package numeral

// Source: src/util/mpf.cpp:1243-1244; source returning-failure transport profile.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestArbitraryFloatToIEEEBVBiasSetTransportAndOwnerGuards(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	var m *ArbitraryFloatManager
	var x ArbitraryFloat
	out := NewInt(7)
	armed, calls := false, 0
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error {
		if armed {
			calls++
			s := &m.ieeeBV
			if !s.executing || m.valueExecuting || !s.biasedExpLive {
				t.Fatal("transport did not reach the bias SET arithmetic boundary")
			}
			xBefore, outBefore, localBefore, used := x, out, s.biasedExp, b.Used()
			if numeralContextRef(s.context()).roots(m.integer) != nil {
				t.Fatal("transport acquired allocating caller roots")
			}
			for index, err := range []error{m.ToIEEEBVMPZ(&x, &out), m.CleanupToIEEEBVMPZ(), m.SourceFinalize(), m.Close(), m.CleanupValue(&x), m.integer.SourceFinalize(), m.integer.CloseChecked(), m.integer.SetInt32(&x.significand, 7), m.rational.SourceFinalize(), m.rational.CloseChecked(), m.integer.Neg(&out)} {
				if !errors.Is(err, ErrNumericPending) {
					t.Fatal("actual returning transport guard", index, err)
				}
			}
			if x != xBefore || out != outBefore || s.biasedExp != localBefore || b.Used() != used {
				t.Fatal("transport mutated stopped graph")
			}
		}
		return f
	}})
	serviceMust(t, e)
	m, e = NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	n := NewInt(0)
	serviceMust(t, m.SetComponents(&x, 32, 24, false, 1, &n))
	_, e = m.IsNormal(&x)
	serviceMust(t, e)
	_, e = m.ExponentBias(32)
	serviceMust(t, e)
	// Bias 2^31-1 plus exponent 1 requires a BIG local SET.
	pad, e := r.Allocate(uint64(99990 - r.State().LocalSize - 8))
	serviceMust(t, e)
	serviceMust(t, r.SetMaxSize(1))
	armed = true
	body := m.ToIEEEBVMPZ(&x, &out)
	armed = false
	if body == nil || calls != 1 || !m.ieeeBV.active || m.ieeeBV.biasedExpLive {
		t.Fatal("actual bias SET allocation transport missing", body, calls)
	}
	s := &m.ieeeBV
	integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{pad}, &x.significand, &out, &s.biasedExp, &m.RoundScope.sigma, &m.RoundScope.cap, &m.RoundScope.sticky)
	if b.Used() != m.OwnedStorageBytes()+r.Stats().TotalBytes {
		t.Fatal("immediate bias SET physical graph", b.Used(), m.OwnedStorageBytes(), r.Stats())
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
		if ref != 0 || m.ieeeBV.active || m.ieeeBV.biasedExpLive || NumericAllocationPending(&m.ieeeBV.numeric) {
			t.Fatal("source stop was not the proved scratch hole after inner owning cleanup", ref)
		}
		serviceMust(t, m.Close())
	}
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("abandoned bias SET scope owner end", b.Used())
	}
}

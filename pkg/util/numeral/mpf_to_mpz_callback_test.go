package numeral

// Source: src/util/mpf.cpp:1174 (foreign Integer COPY allocation transport).

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestArbitraryFloatToMPZForeignTransportAndBothOwners(t *testing.T) {
	m, sourceRuntime, sourceBudget := mpfNumeralHelpersFixture(t)
	var x ArbitraryFloat
	n := NewInt(1)
	serviceMust(t, m.integer.Mul2kInPlace(&n, 39999))
	x = ArbitraryFloat{format: arbitraryFloatFormat(17, 40000, true), exponent: 50000, significand: n}
	out := NewInt(7)
	var other *ArbitraryFloatManager
	var otherValue ArbitraryFloat
	b := arena.NewBudget(1 << 28)
	armed, calls := false, 0
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error {
		if armed {
			calls++
			s := &m.toMPZ
			if !s.executing || s.output != other.integer || sourceRuntime.FailureTransportActive() {
				t.Fatal("actual independent foreign transport missing")
			}
			xBefore, outBefore, otherBefore, used, sourceUsed := x, out, otherValue, b.Used(), sourceBudget.Used()
			if numeralContextRef(s.context()).roots(other.integer) != nil {
				t.Fatal("transport acquired roots")
			}
			copied := *s
			copied.hooks.Policy = &copied
			if numeralContextRef(copied.context()).roots(other.integer) != nil {
				t.Fatal("copied actual scope acquired roots")
			}
			for i, err := range []error{m.ToMPZ(&x, other.integer, &out), m.CleanupToMPZ(), m.SourceFinalize(), m.Close(), m.CleanupValue(&x), m.integer.SourceFinalize(), m.integer.CloseChecked(), m.integer.SetInt32(&x.significand, 9), other.integer.SourceFinalize(), other.integer.CloseChecked(), other.integer.SetInt32(&out, 9), other.integer.Del(&out), other.SourceFinalize(), other.Close(), other.MkOne(8, 3, false, &otherValue)} {
				if !errors.Is(err, ErrNumericPending) {
					t.Fatal("actual two-owner guard", i, err)
				}
			}
			if err := sourceRuntime.Close(); !errors.Is(err, umemory.ErrRuntimeLive) {
				t.Fatal("actual source Runtime live blocks", err)
			}
			if err := other.memory.Close(); !errors.Is(err, umemory.ErrRuntimeLive) {
				t.Fatal("actual output Runtime live blocks", err)
			}
			if x != xBefore || out != outBefore || otherValue != otherBefore || b.Used() != used || sourceBudget.Used() != sourceUsed {
				t.Fatal("transport mutated either owner graph")
			}
		}
		return f
	}})
	serviceMust(t, e)
	other, e = NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	serviceMust(t, other.MkOne(8, 3, false, &otherValue))
	pad, e := r.Allocate(uint64(99990 - r.State().LocalSize - 8))
	serviceMust(t, e)
	serviceMust(t, r.SetMaxSize(1))
	armed = true
	body := m.ToMPZ(&x, other.integer, &out)
	armed = false
	if body == nil || calls != 1 || !m.toMPZ.active {
		t.Fatal("actual foreign COPY refusal", body, calls)
	}
	integerPhysicalAudit(t, m.integer, &x.significand)
	integerPhysicalAuditWithRaw(t, other.integer, []umemory.Block{pad}, &out, &otherValue.significand)
	if sourceBudget.Used() != m.OwnedStorageBytes()+sourceRuntime.Stats().TotalBytes || b.Used() != other.OwnedStorageBytes()+r.Stats().TotalBytes {
		t.Fatal("immediate two-owner physical graph")
	}
	serviceMust(t, r.SetMaxSize(1<<28))
	serviceMust(t, r.Deallocate(&pad))
	serviceMust(t, m.CleanupToMPZ())
	serviceMust(t, other.integer.CleanupRawNumeral(&out))
	serviceMust(t, other.Del(&otherValue))
	serviceMust(t, m.CleanupValue(&x))
	serviceMust(t, other.Close())
	serviceMust(t, r.Close())
	serviceMust(t, m.Close())
	serviceMust(t, sourceRuntime.Close())
	if b.Used() != 0 || sourceBudget.Used() != 0 {
		t.Fatal("two-owner refunds", b.Used(), sourceBudget.Used())
	}
}

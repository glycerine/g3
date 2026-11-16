package numeral

// Source: src/util/mpf.cpp:957-958; returning-failure transport profile.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestMyMPZSqrtReturningTransportAndOwnerGuards(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	var z *IntegerManager
	input, out := NewInt(1), NewInt(7)
	armed, calls := false, 0
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error {
		if armed {
			calls++
			s := &z.mySqrt
			if !s.executing {
				t.Fatal("transport did not reach the real helper")
			}
			before, used := s.values, b.Used()
			if numeralContextRef(s.context()).roots(z) != nil {
				t.Fatal("transport acquired allocating caller roots")
			}
			for index, err := range []error{MyMPZSqrt(z, 40000, false, &input, &out), z.CleanupMyMPZSqrt(), z.SourceFinalize(), z.CloseChecked(), z.SetInt32(&input, 9), z.Del(&input)} {
				if !errors.Is(err, ErrNumericPending) {
					t.Fatal("actual returning transport guard", index, err)
				}
			}
			if err := z.Natural.CloseChecked(); !errors.Is(err, arena.ErrInvalidSlot) {
				t.Fatal("actual Natural owner guard", err)
			}
			if input != NewInt(1) || out != NewInt(7) || s.values != before || b.Used() != used {
				t.Fatal("transport mutated stopped graph")
			}
		}
		return f
	}})
	serviceMust(t, e)
	z, e = NewIntegerManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	pad, e := r.Allocate(uint64(99990 - r.State().LocalSize - 8))
	serviceMust(t, e)
	serviceMust(t, r.SetMaxSize(1))
	armed = true
	body := MyMPZSqrt(z, 40000, false, &input, &out)
	armed = false
	if body == nil || calls != 1 || !z.mySqrt.active {
		t.Fatal("actual helper allocation transport missing", body, calls)
	}
	s := &z.mySqrt
	integerPhysicalAuditWithRaw(t, z, []umemory.Block{pad}, &input, &out, &s.values[0], &s.values[1], &s.values[2], &s.values[3], &s.values[4])
	if b.Used() != integerActualOwnedBytes(z) {
		t.Fatal("immediate helper physical graph", b.Used(), integerActualOwnedBytes(z), r.Stats())
	}
	serviceMust(t, r.SetMaxSize(1<<28))
	serviceMust(t, r.Deallocate(&pad))
	serviceMust(t, z.CloseChecked())
	serviceMust(t, r.Close())
	if b.Used() != 0 || s.active {
		t.Fatal("abandoned helper owner end", b.Used(), s.active)
	}
}

package numeral

// Source: src/ast/fpa_decl_plugin.cpp:176-180; src/util/mpf.cpp:327-333.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestArbitraryFloatCopyFromExplicitForeignOwner(t *testing.T) {
	sourceBudget := arena.NewBudget(1 << 26)
	targetBudget := arena.NewBudget(1 << 26)
	source, e := NewArbitraryFloatManager(serviceConfig(sourceBudget).Integers)
	serviceMust(t, e)
	target, e := NewArbitraryFloatManager(serviceConfig(targetBudget).Integers)
	serviceMust(t, e)
	var raw Int
	digits := []uint32{0xffffffff, 0xfffffffe, 0x7fffffff}
	serviceMust(t, source.integer.SetLimbs(&raw, digits))
	var input ArbitraryFloat
	serviceMust(t, source.SetComponents(&input, 11, 113, true, -42, &raw))
	row, e := target.memory.Allocate(32)
	serviceMust(t, e)
	for i := uint64(0); i < 32; i += 4 {
		serviceMust(t, target.memory.WriteUint32(row, i, 0))
	}
	var receipt ArbitraryFloatValueCopyReceipt
	serviceMust(t, target.CopyToRuntimeFromOwnerWithReceipt(row, 0, source, &input, &receipt))
	got, e := target.CaptureRuntimeValue(row, 0)
	serviceMust(t, e)
	serviceMust(t, target.integer.check(&got.significand))
	actual := target.integer.Cell(got.significand.Cell)
	if got.EBits() != 11 || got.SBits() != 113 || !got.Sign() || got.Exponent() != -42 || actual.Size != uint32(len(digits)) {
		t.Fatal(got, actual)
	}
	for i := range digits {
		if target.integer.Digit(got.significand.Cell, uint32(i)) != digits[i] {
			t.Fatal(actual, digits)
		}
	}
	serviceMust(t, target.CleanupValueCopy(&receipt, false))
	serviceMust(t, target.DelRuntimeValue(row, 0))
	serviceMust(t, target.CleanupValueCopy(&receipt, true))
	serviceMust(t, target.memory.Deallocate(&row))
	serviceMust(t, target.Close())
	// Source remains independently usable after destination context end.
	serviceMust(t, source.integer.check(&input.significand))
	if got := source.integer.Cell(input.significand.Cell); got.Size != uint32(len(digits)) {
		t.Fatal(got)
	}
	serviceMust(t, source.Del(&input))
	serviceMust(t, source.integer.Del(&raw))
	serviceMust(t, source.Close())
	if sourceBudget.Used() != 0 || targetBudget.Used() != 0 {
		t.Fatal(sourceBudget.Used(), targetBudget.Used())
	}
}

func TestArbitraryFloatForeignSourceLifecycleDuringDestinationTransport(t *testing.T) {
	for _, copyValue := range []bool{false, true} {
		b := arena.NewBudget(1 << 28)
		sb := arena.NewBudget(1 << 28)
		provider, e := NewArbitraryFloatManager(serviceConfig(sb).Integers)
		serviceMust(t, e)
		digits := make([]uint32, 1024)
		for i := range digits {
			digits[i] = 0xffffffff
		}
		digits[len(digits)-1] = 0x7fffffff
		var raw Int
		serviceMust(t, provider.integer.SetLimbs(&raw, digits))
		var input ArbitraryFloat
		serviceMust(t, provider.SetComponents(&input, 11, 40000, true, -42, &raw))
		var m *ArbitraryFloatManager
		var refused [10]error
		active, calls := false, 0
		sourceUsed := sb.Used()
		r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error {
			if active {
				calls++
				refused[0] = provider.Close()
				refused[1] = provider.SourceFinalize()
				refused[2] = provider.rational.CloseChecked()
				refused[3] = provider.rational.SourceFinalize()
				refused[4] = provider.integer.CloseChecked()
				refused[5] = provider.integer.SourceFinalize()
				refused[6] = provider.integer.Del(&raw)
				refused[7] = provider.Del(&input)
				refused[8] = provider.CleanupValue(&input)
				// The provider's clean SET call is retired at its return
				// (util_numeral_03-F1); the pending refusal precedes lookup.
				refused[9] = provider.CleanupValueCopy(&ArbitraryFloatValueCopyReceipt{manager: provider}, true)
			}
			return f
		}})
		serviceMust(t, e)
		m, e = NewArbitraryFloatManager(IntegerConfig{Memory: r, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
		serviceMust(t, e)
		row, e := r.Allocate(32)
		serviceMust(t, e)
		for i := uint64(0); i < 32; i += 4 {
			serviceMust(t, r.WriteUint32(row, i, 0))
		}
		small := NewInt(7)
		serviceMust(t, m.SetComponentsToRuntime(row, 0, 8, 24, false, 99, &small))
		padding, e := r.Allocate(uint64(99990 - r.State().LocalSize - 8))
		serviceMust(t, e)
		serviceMust(t, r.SetMaxSize(1))
		var receipt ArbitraryFloatValueCopyReceipt
		active = true
		var body error
		if copyValue {
			body = m.CopyToRuntimeFromOwnerWithReceipt(row, 0, provider, &input, &receipt)
		} else {
			body = m.SetComponentsToRuntimeFromOwnerWithReceipt(row, 0, 11, 40000, true, -42, provider.integer, &raw, &receipt)
		}
		active = false
		if !errors.Is(body, umemory.Failure{Kind: umemory.FailureMemout}) || calls == 0 {
			t.Fatal(body, calls)
		}
		for i, e := range refused {
			if e != ErrNumericPending {
				t.Fatalf("foreign borrowed source lifecycle call %d mutated before refusing: %v; sourceEnded=%v closed=%v source bytes=%d/%d", i, e, provider.sourceEnded, provider.closed, sb.Used(), sourceUsed)
			}
		}
		if provider.sourceEnded || provider.closed || provider.ownCause != nil || provider.sourceCause != nil || provider.integer.SourceFinalizing || provider.rational.SourceFinalizing || sb.Used() != sourceUsed {
			t.Fatal("foreign read owner changed")
		}
		out, e := m.CaptureRuntimeValue(row, 0)
		serviceMust(t, e)
		integerPhysicalAuditWithRaw(t, m.integer, []umemory.Block{row, padding}, &out.significand)
		integerPhysicalAudit(t, provider.integer, &raw, &input.significand)
		serviceMust(t, m.CleanupValueCopy(&receipt, false))
		serviceMust(t, r.SetMaxSize(0))
		serviceMust(t, m.CleanupRuntimeValue(row, 0))
		serviceMust(t, m.CleanupValueCopy(&receipt, true))
		serviceMust(t, r.Deallocate(&row))
		serviceMust(t, r.Deallocate(&padding))
		serviceMust(t, m.Close())
		serviceMust(t, r.Close())
		serviceMust(t, provider.Del(&input))
		serviceMust(t, provider.integer.Del(&raw))
		serviceMust(t, provider.Close())
		if b.Used() != 0 || sb.Used() != 0 {
			t.Fatal(b.Used(), sb.Used())
		}
	}
}

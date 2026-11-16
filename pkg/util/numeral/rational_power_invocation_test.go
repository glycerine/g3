package numeral

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"testing"
)

// This is an explicit malformed deallocation guard, not a malloc campaign.
// It leaves a real accepted RAW entry available to verify receipt identity.
func TestRationalPowerReceiptActualGenerationAndRefusedEntry(t *testing.T) {
	m, b, runtime := rawArithmeticGuardOwner(t)
	a, out := NewRational(), NewRational()
	serviceMust(t, m.SetString(&a, "18446744073709551617"))
	serviceMust(t, m.SetString(&out, "18446744073709551619"))
	rawArithmeticCorrupt(t, m.Z, &out.Num)
	var first RationalPowerInvocationReceipt
	e := m.powerWithAllocationContextAndReceipt(&a, 0, &out, numeralContextRef(NumericAllocationContext{}), &first)
	if !errors.Is(e, arena.ErrBounds) || !m.MatchesRawPowerInvocation(&first) {
		t.Fatal("missing actual generation", e)
	}
	if m.RawPowerInvocationSourceCause(&first) != nil {
		t.Fatal("body error became source END stop")
	}
	var refused RationalPowerInvocationReceipt
	e = m.powerWithAllocationContextAndReceipt(&a, 0, &out, numeralContextRef(NumericAllocationContext{}), &refused)
	if !errors.Is(e, ErrRationalRawArithmeticPending) || m.MatchesRawPowerInvocation(&refused) {
		t.Fatal("refused entry stamped receipt", e)
	}
	serviceMust(t, m.CleanupRawPowerInvocation(&first))
	var second RationalPowerInvocationReceipt
	e = m.powerWithAllocationContextAndReceipt(&a, 0, &out, numeralContextRef(NumericAllocationContext{}), &second)
	if !errors.Is(e, arena.ErrBounds) || !m.MatchesRawPowerInvocation(&second) {
		t.Fatal("missing second generation", e)
	}
	state, used, stats := m.RawInvocationState(), b.Used(), runtime.Stats()
	serviceMust(t, m.CleanupRawPowerInvocation(&first))
	serviceMust(t, m.CleanupRawPowerInvocation(&refused))
	if state != m.RawInvocationState() || used != b.Used() || stats != runtime.Stats() || !m.MatchesRawPowerInvocation(&second) {
		t.Fatal("stale receipt consumed replacement")
	}
	serviceMust(t, m.CleanupRawPowerInvocation(&second))
}

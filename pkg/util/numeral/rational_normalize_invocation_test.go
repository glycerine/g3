package numeral

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"testing"
)

// An explicit malformed deallocation guard leaves the actual SYNCH RAW local
// live. This verifies ownership identity, not source-valid malloc exhaustion.
func TestRationalNormalizeReceiptActualEntryAndRefusal(t *testing.T) {
	m, b, runtime := rawArithmeticGuardOwner(t)
	a := NewRational()
	serviceMust(t, m.Z.SetLimbs(&a.Num, []uint32{17, 0, 1}))
	serviceMust(t, m.Z.Copy(&a.Den, &a.Num))
	rawArithmeticCorrupt(t, m.Z, &a.Num)
	var receipt RationalNormalizeInvocationReceipt
	e := m.normalizeWithAllocationContextAndReceipt(&a, numeralContextRef(NumericAllocationContext{}), &receipt)
	if !errors.Is(e, arena.ErrBounds) || !m.MatchesRawNormalizeInvocation(&receipt) {
		t.Fatal("missing actual normalization entry", e)
	}
	if m.RawNormalizeInvocationSourceCause(&receipt) != nil {
		t.Fatal("normalization body failure became source END stop")
	}
	var refused RationalNormalizeInvocationReceipt
	e = m.normalizeWithAllocationContextAndReceipt(&a, numeralContextRef(NumericAllocationContext{}), &refused)
	if !errors.Is(e, ErrRationalRawArithmeticPending) || m.MatchesRawNormalizeInvocation(&refused) {
		t.Fatal("refused normalization stamped receipt", e)
	}
	state, used, stats := m.RawInvocationState(), b.Used(), runtime.Stats()
	serviceMust(t, m.CleanupRawNormalizeInvocation(&refused))
	if state != m.RawInvocationState() || used != b.Used() || stats != runtime.Stats() {
		t.Fatal("refused receipt consumed older normalization")
	}
	serviceMust(t, m.CleanupRawNormalizeInvocation(&receipt))
	if m.MatchesRawNormalizeInvocation(&receipt) {
		t.Fatal("normalization remains after owning cleanup")
	}
}

func TestBinaryToRationalActualPowerAndNormalizeEntries(t *testing.T) {
	m, budget, _ := rawArithmeticGuardOwner(t)
	diagnostics, e := NewRationalService(serviceConfig(budget))
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, diagnostics.Close()) })
	b, e := NewBinaryRationalManager(m.Z, BinaryRationalConfig{Diagnostics: diagnostics})
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, b.CloseChecked()) })
	for _, tc := range []struct {
		n         int32
		k         uint32
		want      string
		normalize bool
	}{
		{3, 0, "3", false}, {3, 64, "3/18446744073709551616", true}, {-3, 64, "-3/18446744073709551616", true},
	} {
		a := NewBinaryIntPower(tc.n, tc.k)
		out, want := NewRational(), NewRational()
		var receipt BinaryToRationalInvocation
		serviceMust(t, b.ToRationalWithInvocation(m, &a, &out, NumericAllocationContext{}, &receipt))
		if receipt.Power.manager != m || receipt.Power.generation == 0 || m.MatchesRawPowerInvocation(&receipt.Power) {
			t.Fatal("power receipt missed accepted and completed entry")
		}
		if tc.normalize != (receipt.Normalize.manager == m && receipt.Normalize.generation != 0) || m.MatchesRawNormalizeInvocation(&receipt.Normalize) {
			t.Fatal("normalization receipt disagrees with actual rational branch", tc)
		}
		serviceMust(t, m.SetString(&want, tc.want))
		equal, e := m.Equal(&out, &want)
		serviceMust(t, e)
		if !equal {
			t.Fatal("conversion changed", tc.want)
		}
		serviceMust(t, m.CleanupRawPowerInvocation(&receipt.Power))
		serviceMust(t, m.CleanupRawNormalizeInvocation(&receipt.Normalize))
		serviceMust(t, m.Del(&out))
		serviceMust(t, m.Del(&want))
		serviceMust(t, b.Del(&a))
	}
}

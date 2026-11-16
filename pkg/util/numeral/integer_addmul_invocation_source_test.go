package numeral

// Source: src/util/mpz.h:393-407.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestIntegerAddMulSourceShortcutsLeaveReceiptUnchanged(t *testing.T) {
	m, _ := integerManager(t)
	a, c, d := NewInt(3), NewInt(4), NewInt(99)
	var receipt integerAddMulInvocationReceipt
	old := receipt
	for _, value := range []int32{1, -1} {
		b := NewInt(value)
		serviceMust(t, m.addMulWithAllocationContext(&a, &b, &c, &d, &receipt, numeralContextRef(integerAllocationInvocationContext{})))
		if receipt != old || d != NewInt(3+value*4) {
			t.Fatal("literal shortcut acquired RAW declaration", value, receipt, d)
		}
	}
}

func TestIntegerAddMulSourceRAWBodyFailureRetainsPublishedTemporary(t *testing.T) {
	found := false
	for after := int64(0); after < 32; after++ {
		m, budget := integerManager(t)
		a, b, d := NewInt(9), NewInt(3), NewInt(11)
		var c, product Int
		digits := make([]uint32, 64)
		digits[0] = 7
		digits[63] = 1
		serviceMust(t, m.SetLimbs(&c, digits))
		expected := make([]uint32, 64)
		expected[0] = 21
		expected[63] = 3
		serviceMust(t, m.SetLimbs(&product, expected))
		var receipt integerAddMulInvocationReceipt
		budget.FailAfter(after)
		e := m.addMulWithAllocationContext(&a, &b, &c, &d, &receipt, numeralContextRef(integerAllocationInvocationContext{}))
		selected := e != nil && receipt.Frame != nil && receipt.Frame.Stage == 1 && !receipt.Frame.Tmp.IsSmall()
		if selected {
			found = true
			t.Log("actual Mul publication before end-only DEL refusal prefix", after)
			if d != NewInt(11) || receipt.Frame.SourceCause != nil || receipt.Frame.ended {
				t.Fatal("body failure reached forbidden end-only DEL", after, d)
			}
			equal, e := m.Equal(&receipt.Frame.Tmp, &product)
			serviceMust(t, e)
			if !equal {
				t.Fatal("actual MUL publication is not the literal independent product", after)
			}
			integerPhysicalAudit(t, m, &a, &b, &c, &d, &product, &receipt.Frame.Tmp)
			old := receipt.Frame.Tmp
			used, stats, state := budget.Used(), m.Memory.Stats(), m.Memory.State()
			serviceMust(t, m.CleanupAddMulInvocation(&receipt, false))
			if receipt.Frame.Tmp != old || budget.Used() != used || m.Memory.Stats() != stats || m.Memory.State() != state {
				t.Fatal("body failure invented source RAW destructor", after)
			}
		}
		// Caller declarations complete before explicit unjoined RAW retirement.
		serviceMust(t, m.Del(&d))
		serviceMust(t, m.CleanupAddMulInvocation(&receipt, true))
		integerPhysicalAudit(t, m, &a, &b, &c, &d, &product)
		budget.FailAfter(-1)
		serviceMust(t, m.Del(&c))
		serviceMust(t, m.Del(&product))
		if found {
			break
		}
	}
	if !found {
		t.Fatal("actual prefix campaign never reached published Mul temporary before Add")
	}
}

// Actual public TLS accounting reaches the signed-domain guard after physical
// consumption. This is a source release-stop proof, not ordinary exhaustion.
func TestIntegerAddMulSourceEndDELStopsAfterCallerOutput(t *testing.T) {
	b := arena.NewBudget(1 << 26)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64})
	serviceMust(t, e)
	cfg := serviceConfig(b).Integers
	cfg.Memory = rt
	m, e := NewIntegerManager(cfg)
	serviceMust(t, e)
	var a, c, warm Int
	digits := make([]uint32, 64)
	digits[0] = 17
	digits[63] = 1
	serviceMust(t, m.SetLimbs(&c, digits))
	product := make([]uint32, 64)
	product[0] = 34
	product[63] = 2
	serviceMust(t, m.SetLimbs(&a, product))
	serviceMust(t, m.Neg(&a))
	prime := NewInt(251)
	serviceMust(t, m.Rem(&c, &prime, &warm))
	d, bval := NewInt(9), NewInt(2)
	var receipt integerAddMulInvocationReceipt
	defer func() {
		recovery, e := rt.Allocate(400000)
		serviceMust(t, e)
		contextTLSFinish(t, rt, func() error { return m.CleanupAddMulInvocation(&receipt, true) })
		for _, n := range []*Int{&a, &c, &warm, &d} {
			contextTLSFinish(t, rt, func() error { return m.Del(n) })
		}
		contextTLSFinish(t, rt, m.CloseChecked)
		e = rt.Deallocate(&recovery)
		if e != nil && (!errors.Is(e, arena.ErrBounds) || recovery != (umemory.Block{})) {
			t.Fatal("recovery physical consumption", e)
		}
		contextTLSFinish(t, rt, rt.Close)
		if b.Used() != 0 {
			t.Fatal("named graph remains", b.Used())
		}
	}()
	external, e := rt.Allocate(200000)
	serviceMust(t, e)
	contextTLSNearMinimum(t, rt)
	e = rt.Deallocate(&external)
	if e == nil || external != (umemory.Block{}) {
		t.Fatal("actual external signed-domain stop", e)
	}
	e = m.addMulWithAllocationContext(&a, &bval, &c, &d, &receipt, numeralContextRef(integerAllocationInvocationContext{}))
	if e == nil || receipt.Frame == nil || receipt.Frame.Stage != 3 || d != NewInt(0) {
		t.Fatal("caller output must precede reached end DEL", e, d, receipt)
	}
	cause := m.AddMulInvocationSourceCause(&receipt)
	if cause == nil || receipt.Frame.BodyCause != nil || receipt.Frame.ended || receipt.Frame.owned {
		t.Fatal("missing first strict source stop", cause)
	}
	integerPhysicalAudit(t, m, &a, &c, &warm, &d, &receipt.Frame.Tmp)
	used, state, stats := b.Used(), rt.State(), rt.Stats()
	if again := m.finishAddMulSource(receipt.Frame); again != cause || used != b.Used() || state != rt.State() || stats != rt.Stats() {
		t.Fatal("source retry performed another release", again)
	}
	serviceMust(t, m.CleanupAddMulInvocation(&receipt, false))
	if receipt.Frame.ended || !receipt.Frame.owned || m.AddMulInvocationSourceCause(&receipt) != cause || d != NewInt(0) {
		t.Fatal("own acknowledgment erased source stop")
	}
	serviceMust(t, m.Del(&d))
	serviceMust(t, m.CleanupAddMulInvocation(&receipt, true))
	if m.AddMulInvocationSourceCause(&receipt) != cause || receipt.Frame.ended || !receipt.Frame.owned {
		t.Fatal("abort erased historical source stop")
	}
	integerPhysicalAudit(t, m, &a, &c, &warm, &d)
}

func TestIntegerAddMulSourceStaleAndForeignReceiptsPreserveLaterRAW(t *testing.T) {
	m, budget := integerManager(t)
	a, b, d := NewInt(9), NewInt(3), NewInt(11)
	var c, product Int
	digits := make([]uint32, 64)
	digits[0] = 7
	digits[63] = 1
	serviceMust(t, m.SetLimbs(&c, digits))
	serviceMust(t, m.SetLimbs(&product, digits))
	var older integerAddMulInvocationReceipt
	serviceMust(t, m.addMulWithAllocationContext(&a, &b, &c, &d, &older, numeralContextRef(integerAllocationInvocationContext{})))
	if !older.Frame.ended {
		t.Fatal("real earlier source invocation did not reach end DEL")
	}
	serviceMust(t, m.Del(&d))
	digits = make([]uint32, 128)
	digits[0] = 7
	digits[127] = 1
	serviceMust(t, m.SetLimbs(&c, digits))
	serviceMust(t, m.SetLimbs(&product, digits))
	var later integerAddMulInvocationReceipt
	budget.FailAfter(1)
	if e := m.addMulWithAllocationContext(&a, &b, &c, &d, &later, numeralContextRef(integerAllocationInvocationContext{})); e == nil || later.Frame == nil || later.Frame.Tmp.IsSmall() || later.invocation == older.invocation {
		t.Fatal("actual later larger MUL publication", e)
	}
	tmp := later.Frame.Tmp
	used, state, stats := budget.Used(), m.Memory.State(), m.Memory.Stats()
	copiedFrame := *later.Frame
	wrongFrame := later
	wrongFrame.Frame = &copiedFrame
	other, _ := integerManager(t)
	wrongManager := later
	wrongManager.manager = other
	for _, r := range []*integerAddMulInvocationReceipt{&older, &wrongFrame, &wrongManager} {
		serviceMust(t, m.CleanupAddMulInvocation(r, false))
		serviceMust(t, m.CleanupAddMulInvocation(r, true))
		if tmp != later.Frame.Tmp || used != budget.Used() || state != m.Memory.State() || stats != m.Memory.Stats() {
			t.Fatal("foreign/stale receipt consumed actual later RAW")
		}
	}
	// Literal shortcuts do not replace an existing receipt or touch retained RAW.
	shortcut := NewInt(1)
	quickOut := NewInt(0)
	incoming := older
	serviceMust(t, m.addMulWithAllocationContext(&a, &shortcut, &a, &quickOut, &older, numeralContextRef(integerAllocationInvocationContext{})))
	if older != incoming || quickOut != NewInt(18) || later.Frame.Tmp != tmp {
		t.Fatal("shortcut changed incoming receipt")
	}
	integerPhysicalAudit(t, m, &a, &b, &c, &product, &d, &quickOut, &later.Frame.Tmp)
	serviceMust(t, m.CleanupAddMulInvocation(&later, false))
	if later.Frame.Tmp != tmp {
		t.Fatal("body failure reached RAW DEL")
	}
	serviceMust(t, m.Del(&d))
	serviceMust(t, m.CleanupAddMulInvocation(&later, true))
	integerPhysicalAudit(t, m, &a, &b, &c, &product, &d)
	budget.FailAfter(-1)
	serviceMust(t, m.Del(&c))
	serviceMust(t, m.Del(&product))
}

func TestIntegerAddMulSourceEveryActualReservationPrefix(t *testing.T) {
	firstSuccess := -1
	for prefix := int64(0); prefix < 64; prefix++ {
		m, budget := integerManager(t)
		a, b, d := NewInt(9), NewInt(3), NewInt(11)
		var c, expected Int
		digits := make([]uint32, 64)
		digits[0] = 7
		digits[63] = 1
		serviceMust(t, m.SetLimbs(&c, digits))
		result := make([]uint32, 64)
		result[0] = 30
		result[63] = 3
		serviceMust(t, m.SetLimbs(&expected, result))
		var receipt integerAddMulInvocationReceipt
		budget.FailAfter(prefix)
		e := m.addMulWithAllocationContext(&a, &b, &c, &d, &receipt, numeralContextRef(integerAllocationInvocationContext{}))
		if receipt.Frame == nil {
			t.Fatal("guarded RAW body did not acquire actual frame")
		}
		integerPhysicalAudit(t, m, &a, &b, &c, &d, &expected, &receipt.Frame.Tmp)
		if e == nil {
			equal, e := m.Equal(&d, &expected)
			serviceMust(t, e)
			if !equal || !receipt.Frame.ended || receipt.Frame.BodyCause != nil || receipt.Frame.SourceCause != nil {
				t.Fatal("literal independent addmul result/end", prefix)
			}
			firstSuccess = int(prefix)
		} else if receipt.Frame.Stage < 3 {
			tmp := receipt.Frame.Tmp
			used, state, stats := budget.Used(), m.Memory.State(), m.Memory.Stats()
			serviceMust(t, m.CleanupAddMulInvocation(&receipt, false))
			if receipt.Frame.Tmp != tmp || used != budget.Used() || state != m.Memory.State() || stats != m.Memory.Stats() {
				t.Fatal("failed body invented end DEL", prefix)
			}
		}
		// Immediate prefix evidence and all exact caller/RAW cleanup precede lift.
		serviceMust(t, m.Del(&d))
		serviceMust(t, m.CleanupAddMulInvocation(&receipt, true))
		integerPhysicalAudit(t, m, &a, &b, &c, &d, &expected)
		budget.FailAfter(-1)
		serviceMust(t, m.Del(&c))
		serviceMust(t, m.Del(&expected))
		if firstSuccess >= 0 {
			break
		}
	}
	if firstSuccess < 0 {
		t.Fatal("literal AddMul did not reach first success")
	}
	t.Log("first actual cold AddMul success prefix", firstSuccess)
}

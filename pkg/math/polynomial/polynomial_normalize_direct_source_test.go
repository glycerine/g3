package polynomial

// Source: src/math/polynomial/polynomial.cpp:5904-5940; src/util/mpzzp.h:211.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

func TestPolynomialNormalizeActualGCDCopyRefusalPreservesBodyHistory(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	p, e := c.MkTerms([]unumeral.Int{c.integer.Two64}, []Monomial{c.monomials.unit})
	serviceMust(t, e)
	p = polynomialCaller(t, c, p)
	full.originalBudget.FailAfter(0)
	var receipt polynomialNormalizeInvocationReceipt
	_, returned := full.normalizeWithReceipt(p, &receipt)
	// This first actual GCD array SET refusal has a constructed g, no a yet,
	// and no invented cheap-buffer reset or child numeral constructor.
	if !numericPhysicalReservationRefusal(returned) {
		t.Fatal("actual GCD initial SET refusal", returned)
	}
	polynomialImpPhysicalAuditValues(t, full, []Polynomial{p})
	body := full.normalization.bodyCause
	if body == nil || !errors.Is(returned, body) {
		t.Error("reached body history erased during source g destruction", returned, body)
	}
	full.originalBudget.FailAfter(-1)
	serviceMust(t, full.CleanupNormalization())
	if full.normalization.bodyCause != body {
		t.Error("own acknowledgment altered first body cause")
	}
}

func TestPolynomialNormalizeActualFailedGCDRawOwnedAfterOlderNUM(t *testing.T) {
	reached := false
	for prefix := int64(0); prefix < 96; prefix++ {
		full := polynomialPSCFullFixture(t)
		c := full.core
		var a, b unumeral.Int
		aw, bw := make([]uint32, 80), make([]uint32, 77)
		aw[0], aw[79] = 6, 1<<29
		bw[0], bw[76] = 10, 1<<27
		serviceMust(t, c.integer.SetLimbs(&a, aw))
		serviceMust(t, c.integer.SetLimbs(&b, bw))
		x, e := c.monomials.MkVariable(0, 1)
		serviceMust(t, e)
		p, e := c.MkTerms([]unumeral.Int{a, b}, []Monomial{x, c.monomials.unit})
		serviceMust(t, e)
		p = polynomialCaller(t, c, p)
		full.originalBudget.FailAfter(prefix)
		_, returned := full.Normalize(p)
		raw := &c.integer.GcdRaw
		if returned != nil && raw.Active && raw.BodyCause != nil {
			reached = true
			// The actual called Integer frame owns its five RAW fields and pending
			// lower child roots; no manager registry or before/after census authority.
			polynomialImpPhysicalAuditNamedRaw(t, full, []Polynomial{p}, []*unumeral.Int{&a, &b}, 0, nil)
			serviceMust(t, full.cleanupNormalizationPhase(false))
			if !raw.Active {
				t.Error("false phase aborted unjoined RAW before caller NUM suffix")
			}
			own := full.cleanupNormalizationPhase(true)
			full.originalBudget.FailAfter(-1)
			if own != nil {
				t.Error("exact Normalize owning continuation", prefix, own)
			}
			if raw.Active {
				t.Error("actual reached GCD RAW child omitted after older g NUM suffix", prefix)
			}
			serviceMust(t, c.integer.CleanupGCDRaw())
			polynomialImpPhysicalAuditNamedRaw(t, full, []Polynomial{p}, []*unumeral.Int{&a, &b}, 0, nil)
			serviceMust(t, c.integer.Del(&a))
			serviceMust(t, c.integer.Del(&b))
			break
		}
		full.originalBudget.FailAfter(-1)
		serviceMust(t, full.CleanupNormalization())
		serviceMust(t, c.integer.CleanupGCDRaw())
		serviceMust(t, c.integer.Del(&a))
		serviceMust(t, c.integer.Del(&b))
	}
	if !reached {
		t.Fatal("real failed GCD RAW source body not reached")
	}
}

func TestPolynomialCheapNormalizeActualGCDRawChild(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	b := &c.cheap
	var a, z unumeral.Int
	aw, bw := make([]uint32, 80), make([]uint32, 77)
	aw[0], aw[79] = 6, 1<<29
	bw[0], bw[76] = 10, 1<<27
	serviceMust(t, c.integer.SetLimbs(&a, aw))
	serviceMust(t, c.integer.SetLimbs(&z, bw))
	x, e := c.monomials.MkVariable(0, 1)
	serviceMust(t, e)
	serviceMust(t, b.add(&a, x, false))
	serviceMust(t, b.add(&z, c.monomials.unit, false))
	full.originalBudget.FailAfter(0)
	returned := b.normalize()
	if returned == nil || !c.integer.GcdRaw.Active {
		t.Fatal("actual cheap GCD refusal", returned, c.integer.GcdRaw.Active)
	}
	polynomialImpPhysicalAuditNamedRaw(t, full, nil, []*unumeral.Int{&a, &z}, 0, nil)
	body := b.gcdBodyCause
	serviceMust(t, b.finishNormalizeNumeralPhase(true, false))
	if !c.integer.GcdRaw.Active {
		t.Error("false child phase aborted unjoined RAW")
	}
	serviceMust(t, b.finishNormalizeNumeralPhase(true, true))
	if b.gcdBodyCause != body {
		t.Error("own acknowledgment erased body history")
	}
	full.originalBudget.FailAfter(-1)
	if c.integer.GcdRaw.Active {
		t.Error("cheap scoped g owning continuation omitted its actual younger GCD RAW child")
	}
	serviceMust(t, c.integer.CleanupGCDRaw())
	serviceMust(t, c.integer.Del(&a))
	serviceMust(t, c.integer.Del(&z))
}

func TestPolynomialNormalizeDirectCapturedInvocationGuards(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	z := c.integer
	serviceMust(t, full.beginNormalization())
	s := &full.normalization
	s.executing = true
	r := s.directReceipt
	ctx := numericIntegerModularManager_normalizationAllocationContext(c.modular, &s.directReceipt)
	if (ctx.roots(z)) != (&s.numeric) {
		t.Fatal("actual guarded Normalization frame")
	}
	copyScope := *s
	forged := r
	forged.frame = &copyScope
	if (numericIntegerManager_normalizationAllocationRoots(z, &forged)) != nil {
		t.Fatal("copied numeral frame accepted")
	}
	c.callbackDepth++
	if (ctx.roots(z)) != nil {
		t.Error("callback acquired owner context")
	}
	c.callbackDepth--
	foreign := *c.modular
	if (integerAllocationContextRef(numericIntegerModularManager_normalizationAllocationContext(&foreign, &r)).roots(z)) != nil {
		t.Error("foreign modular receiver accepted")
	}
	s.executing = false
	serviceMust(t, full.finishNormalizationSource())
	serviceMust(t, full.beginNormalization())
	s.executing = true
	if (ctx.roots(z)) != nil {
		t.Error("saved embedded receipt retargeted new invocation")
	}
	current := numericIntegerModularManager_normalizationAllocationContext(c.modular, &s.directReceipt)
	if (current.roots(z)) != (&s.numeric) {
		t.Error("new actual guarded invocation refused")
	}
	s.executing = false
	serviceMust(t, full.finishNormalizationSource())
	c.pscLeave()
	c.pscLeave()
}

func TestPolynomialSOMNormalizeDirectCapturedInvocationGuards(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	b := &c.cheap
	z := c.integer
	serviceMust(t, b.beginNormalizeNumeral(nil))
	s := &b.normalizeScope
	s.executing = true
	r := s.receipt
	ctx := numericIntegerModularManager_somNormalizeAllocationContext(c.modular, &s.receipt)
	if (ctx.roots(z)) != (&b.gcdNumeric) {
		t.Fatal("actual guarded SOM normalize frame")
	}
	forged := r
	copyBuffer := *b
	forged.buffer = &copyBuffer
	forged.frame = &copyBuffer.normalizeScope
	if (numericIntegerManager_somNormalizeAllocationRoots(z, &forged)) != nil {
		t.Fatal("copied source buffer accepted")
	}
	copyScope := *s
	forged = r
	forged.frame = &copyScope
	if (numericIntegerManager_somNormalizeAllocationRoots(z, &forged)) != nil {
		t.Fatal("copied source frame accepted")
	}
	c.callbackDepth++
	if (ctx.roots(z)) != nil {
		t.Error("callback acquired nested numeral context")
	}
	c.callbackDepth--
	s.executing = false
	serviceMust(t, b.finishNormalizeNumeral(false))
	serviceMust(t, b.beginNormalizeNumeral(nil))
	s.executing = true
	if (ctx.roots(z)) != nil {
		t.Error("saved child context retargeted embedded receipt")
	}
	current := numericIntegerModularManager_somNormalizeAllocationContext(c.modular, &s.receipt)
	if (current.roots(z)) != (&b.gcdNumeric) {
		t.Error("new real child refused")
	}
	s.executing = false
	serviceMust(t, b.finishNormalizeNumeral(false))
}

func TestPolynomialNormalizeRetainedFrameRejectsBeforeForeignCheapReset(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	var aa, bb unumeral.Int
	aw, bw := make([]uint32, 80), make([]uint32, 77)
	aw[0], aw[79] = 6, 1<<29
	bw[0], bw[76] = 10, 1<<27
	serviceMust(t, c.integer.SetLimbs(&aa, aw))
	serviceMust(t, c.integer.SetLimbs(&bb, bw))
	t.Cleanup(func() {
		full.originalBudget.FailAfter(-1)
		serviceMust(t, full.CleanupNormalization())
		serviceMust(t, c.integer.Del(&aa))
		serviceMust(t, c.integer.Del(&bb))
	})
	x0, e := c.monomials.MkVariable(0, 1)
	serviceMust(t, e)
	p, e := c.MkTerms([]unumeral.Int{aa, bb}, []Monomial{x0, c.monomials.unit})
	serviceMust(t, e)
	p = polynomialCaller(t, c, p)
	q := polynomialQueryTerms(t, c, []int32{9, -1}, [][]MonomialPower{{{0, 1}}, nil})
	full.originalBudget.FailAfter(0)
	_, first := full.Normalize(p)
	if !numericPhysicalReservationRefusal(first) || !full.normalization.active {
		t.Fatal("actual retained initial SET", first)
	}
	polynomialImpPhysicalAuditNamedRaw(t, full, []Polynomial{p, q}, []*unumeral.Int{&aa, &bb}, 0, nil)
	// Later independently invoked actions lift the fault, then append a real
	// foreign source row. They grant no authority to the stopped old frame.
	full.originalBudget.FailAfter(-1)
	serviceMust(t, c.modular.SetZPUint64(7))
	x, e := c.monomials.MkVariable(0, 1)
	serviceMust(t, e)
	a := unumeral.NewInt(3)
	serviceMust(t, c.cheap.add(&a, x, false))
	before, e := c.cheap.ms.size(c)
	serviceMust(t, e)
	value, e := c.cheap.as.coefficient(c, 0)
	serviceMust(t, e)
	inv := full.normalization.invocation
	_, second := full.Normalize(q)
	after, e := c.cheap.ms.size(c)
	serviceMust(t, e)
	if second != ErrPolynomialPending || after != before || full.normalization.invocation != inv {
		t.Error("retained frame rejection occurred after foreign Reset", second, before, after)
	}
	if after != 0 {
		got, e := c.cheap.as.coefficient(c, 0)
		serviceMust(t, e)
		if got != value {
			t.Error("foreign value changed")
		}
	}
	serviceMust(t, full.CleanupNormalization())
	after, e = c.cheap.ms.size(c)
	serviceMust(t, e)
	if after != before {
		t.Error("old own acknowledgment consumed foreign persistent row")
	}
}

func TestPolynomialSOMNormalizeCopiedReceiverCannotDestroyActualNUM(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	b := &c.cheap
	serviceMust(t, b.beginNormalizeNumeral(nil))
	serviceMust(t, c.integer.Copy(&b.gcd, &c.integer.Two64))
	before := b.gcd
	state := c.memory.State()
	used := full.originalBudget.Used()
	copied := *b
	got := copied.finishNormalizeNumeral(false)
	if got != ErrPolynomialOwner || b.gcd != before || c.memory.State() != state || full.originalBudget.Used() != used {
		t.Error("copied receiver destroyed source numeral", got, b.gcd, before)
	}
	// RED-only physical-consumption recovery of this exact known descriptor;
	// the production owning contract must refuse before touching it.
	if got == nil {
		b.gcd = unumeral.Int{}
	}
	serviceMust(t, b.finishNormalizeNumeral(true))
	polynomialImpPhysicalAuditValues(t, full, nil)
}

// The public source SOM class has a concrete separately charged wrapper/member,
// rather than one of the four persistent imp buffers.
func TestPolynomialSOMNormalizeActualStandaloneMember(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	b, e := NewPolynomialSOMBuffer(c)
	serviceMust(t, e)
	serviceMust(t, b.SetOwner(c))
	serviceMust(t, b.Add(&c.integer.Two64, c.monomials.unit))
	p, e := b.Mk(true)
	if e != nil {
		t.Error("actual standalone scoped g construction", e)
	} else {
		serviceMust(t, c.IncRef(p))
		a, e := c.Coefficient(p, 0)
		serviceMust(t, e)
		if a != unumeral.NewInt(1) {
			t.Error("source gcd division changed constant")
		}
		serviceMust(t, c.DecRef(p))
	}
	serviceMust(t, b.CloseChecked())
}

// Real default helper construction: the separately owned class must not start
// its older vector destruction while the younger method-local g is retained.
func TestPolynomialSOMNormalizeStandaloneSourceOwnerProtectsActualChild(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	b, e := NewPolynomialSOMBuffer(c)
	serviceMust(t, e)
	serviceMust(t, b.SetOwner(c))
	serviceMust(t, b.Add(&c.integer.Two64, c.monomials.unit))
	serviceMust(t, b.buffer.beginNormalizeNumeral(nil))
	serviceMust(t, c.integer.Copy(&b.buffer.gcd, &c.integer.Two64))
	beforeRaw, beforeStage, beforeNUM := append([]unumeral.Int(nil), b.buffer.as.values...), b.sourceStage, b.buffer.gcd
	beforeRuntime := c.memory.State()
	if e = b.SourceFinalize(); !errors.Is(e, ErrPolynomialPending) {
		t.Error("source owner passed retained younger method NUM", e)
	}
	if !polynomialEqualNumerals(b.buffer.as.values, beforeRaw) || b.sourceStage != beforeStage || b.sourceStarted || b.buffer.gcd != beforeNUM || c.memory.State() != beforeRuntime {
		t.Error("older class source prefix mutated")
	}
	serviceMust(t, b.CloseChecked())
}

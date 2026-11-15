package polynomial

// Source: src/math/polynomial/polynomial.cpp:5940-5951; src/util/mpzzp.h:58-68,131.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

func TestPolynomialNegDirectReceiptsValidateActualExecutingFrame(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	var receipt polynomialNegInvocationReceipt
	p, e := c.negWithReceipt(c.unit, &receipt)
	serviceMust(t, e)
	p = polynomialCaller(t, c, p)
	var out unumeral.Int
	value := unumeral.NewInt(7)
	serviceMust(t, numericIntegerModularManager_copyWithNegReceipt(c.modular, &out, &value, &receipt))
	serviceMust(t, numericIntegerModularManager_negWithNegReceipt(c.modular, &out, &receipt))
	if out != unumeral.NewInt(-7) {
		t.Fatal("same modular SET/NEG engine")
	}
	if (numericIntegerManager_negAllocationRoots(c.integer, &receipt)) != nil {
		t.Fatal("completed frame acquired a new numeric leaf")
	}
}

func TestPolynomialNegDirectContextRejectsCopiedForeignAndReusedFrame(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	receipt, e := c.beginNegInvocation(nil)
	serviceMust(t, e)
	context := numericIntegerModularManager_negAllocationContext(c.modular, receipt)
	if (context.roots(c.integer)) != (&c.numeric) {
		t.Fatal("actual guarded source NUM frame not acquired")
	}
	copied := *receipt.frame
	bad := *receipt
	bad.frame = &copied
	if (numericIntegerManager_negAllocationRoots(c.integer, &bad)) != nil {
		t.Fatal("copied frame acquired authority")
	}
	copiedCore := *c
	if e := copiedCore.finishNegSource(); !errors.Is(e, ErrPolynomialOwner) {
		t.Fatal("copied core entered actual source NUM destructor", e)
	}
	bad = *receipt
	bad.manager = &copiedCore
	bad.frame = &copiedCore.polynomialNegScope
	if (numericIntegerManager_negAllocationRoots(c.integer, &bad)) != nil {
		t.Fatal("copied core acquired authority")
	}
	c.callbackDepth++
	if (context.roots(c.integer)) != nil {
		t.Fatal("callback acquired numeric authority")
	}
	c.callbackDepth--
	c.negativeExecuting = false
	serviceMust(t, c.finishNegSource())
	serviceMust(t, c.cleanupNegInvocation(receipt, true))
	next, e := c.beginNegInvocation(nil)
	serviceMust(t, e)
	if (context.roots(c.integer)) != nil {
		t.Fatal("old embedded receipt pointer retargeted next invocation")
	}
	if (integerAllocationContextRef(numericIntegerModularManager_negAllocationContext(c.modular, next)).roots(c.integer)) != (&c.numeric) {
		t.Fatal("new actual invocation lost authority")
	}
	c.negativeExecuting = false
	serviceMust(t, c.finishNegSource())
	serviceMust(t, c.cleanupNegInvocation(next, true))
}

func TestPolynomialNegActualFactoryReturnAndBorrowedInputAuthority(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	c := full.core
	zeroRC, e := c.readWord(c.zero, 0)
	serviceMust(t, e)
	var zeroReceipt polynomialNegInvocationReceipt
	zero, e := c.negWithReceipt(c.zero, &zeroReceipt)
	serviceMust(t, e)
	zero = polynomialCaller(t, c, zero)
	size, e := c.readWord(zero, 8)
	serviceMust(t, e)
	if zero == c.zero || size != 0 || c.negFactory.invocation == 0 || c.negAppend.invocation != 0 || c.negativeResult != (Polynomial{}) {
		t.Fatal("literal empty cheap.mk must construct a new zero-size factory, without any append row")
	}
	serviceMust(t, c.cleanupNegInvocation(&zeroReceipt, false))
	serviceMust(t, c.cleanupNegInvocation(&zeroReceipt, true))
	after, e := c.readWord(c.zero, 0)
	serviceMust(t, e)
	if after != zeroRC {
		t.Fatal("owning acknowledgment decremented borrowed cached zero")
	}
	p := polynomialICCPUnownedTerms(t, c, []int32{7}, [][]MonomialPower{{{0, 1}}})
	p = polynomialCaller(t, c, p)
	for i := 0; i < 2; i++ {
		var receipt polynomialNegInvocationReceipt
		result, e := c.negWithReceipt(p, &receipt)
		serviceMust(t, e)
		if c.negFactory.invocation == 0 || c.negAppend.invocation == 0 || c.factoryInvocation.active || c.cheap.appendScope.active || c.negativeActive || c.negativeResult != (Polynomial{}) {
			t.Fatal("successful raw return retained actual callee factory/row authority")
		}
		polynomialImpPhysicalAuditNamed(t, full, []Polynomial{p, result}, nil, 0)
		result = polynomialCaller(t, c, result)
		polynomialQueryExpect(t, c, result, map[[3]uint32]int64{{1, 0, 0}: -7})
	}
}

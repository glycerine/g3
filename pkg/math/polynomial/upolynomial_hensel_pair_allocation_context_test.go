package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:583-674;
// src/util/mpz.h:124-149; src/util/mpzzp.h:154,230-233.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

func TestUPolynomialHenselPairNumericContextActualFrameAndCapturedNonce(t *testing.T) {
	q, s := mignotteDirectFixture(t)
	r, e := s.beginHenselPairInvocation()
	serviceMust(t, e)
	h := r.frame
	h.rm, e = NewRelatedUPolynomialManager(s.manager)
	serviceMust(t, e)
	h.constructed = 2
	c := numericIntegerModularManager_henselPairAllocationContext(s.manager.modular, &r)
	if c.henselPairReceipt(q.Integers()) == nil {
		t.Fatal("actual guarded frame lacks direct allocation authority")
	}
	old := c
	ownerCopy := *s
	badOwner := r
	badOwner.service = &ownerCopy
	if integerAllocationContextRef(numericIntegerModularManager_henselPairAllocationContext(s.manager.modular, &badOwner)).henselPairReceipt(q.Integers()) != nil {
		t.Fatal("copied actual service acquired authority")
	}
	foreign, e := unumeral.NewIntegerModularManager(q.Integers())
	serviceMust(t, e)
	if integerAllocationContextRef(numericIntegerModularManager_henselPairAllocationContext(foreign, &r)).henselPairReceipt(q.Integers()) != nil {
		t.Fatal("foreign modular receiver acquired pair roots")
	}
	serviceMust(t, foreign.Close())
	copied := *h
	bad := r
	bad.frame = &copied
	if integerAllocationContextRef(numericIntegerModularManager_henselPairAllocationContext(s.manager.modular, &bad)).henselPairReceipt(q.Integers()) != nil {
		t.Fatal("copied cold frame acquired authority")
	}
	s.manager.callbackDepth++
	if c.henselPairReceipt(q.Integers()) != nil {
		t.Fatal("external checkpoint callback acquired parent authority")
	}
	s.manager.callbackDepth--
	h.executing = false
	serviceMust(t, s.cleanupHenselPairInvocation(&r, false))
	serviceMust(t, s.cleanupHenselPairInvocation(&r, true))
	newer, e := s.beginHenselPairInvocation()
	serviceMust(t, e)
	newer.frame.rm, e = NewRelatedUPolynomialManager(s.manager)
	serviceMust(t, e)
	newer.frame.constructed = 2
	r = newer
	if old.henselPairReceipt(q.Integers()) != nil {
		t.Fatal("saved context retargeted through reused receipt pointer")
	}
	newer.frame.executing = false
	serviceMust(t, s.cleanupHenselPairInvocation(&newer, false))
	serviceMust(t, s.cleanupHenselPairInvocation(&newer, true))
	// Invalid Go metadata guard fixture: exhausting the actual stored nonce
	// must refuse before resetting the same cold frame or its history.
	newer.frame.invocation = ^uint64(0)
	used, frame := q.Budget().Used(), *newer.frame
	_, e = s.beginHenselPairInvocation()
	if e == nil || q.Budget().Used() != used || newer.frame.invocation != frame.invocation || newer.frame.active != frame.active || newer.frame.sourceCause != frame.sourceCause {
		t.Fatal("wrapped begin mutated actual frame", e)
	}
}

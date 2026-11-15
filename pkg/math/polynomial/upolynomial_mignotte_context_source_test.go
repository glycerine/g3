package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:979-1017;
// src/util/mpz.h:124-149.

import (
	"testing"
)

// Invalid Go owner copies are guard fixtures, not source allocation prefixes.
// The stale context comes from two real guarded invocations of the same member.
func TestUPolynomialMignotteExactContextAndReusedReceipt(t *testing.T) {
	q, s := mignotteDirectFixture(t)
	_, e := s.newSquareFreeScope()
	serviceMust(t, e)
	r, e := s.beginMignotteInvocation()
	serviceMust(t, e)
	f := r.frame
	if (integerAllocationContextRef(numericIntegerModularManager_mignotteAllocationContext(s.manager.modular, &f.receipt)).roots(q.Integers())) != nil {
		t.Fatal("pre-NUM metadata acquired numeric authority")
	}
	f.constructed = 1
	c := numericIntegerModularManager_mignotteAllocationContext(s.manager.modular, &f.receipt)
	if (c.roots(q.Integers())) != (&f.numeric) {
		t.Fatal("actual guarded embedded context")
	}
	copyFrame := *f
	bad := r
	bad.frame = &copyFrame
	if (integerAllocationContextRef((integerAllocationInvocationContext{mignotte: &bad, mignotteInvocation: bad.invocation})).roots(q.Integers())) != nil {
		t.Fatal("copied frame acquired authority")
	}
	copyService := *s
	bad = r
	bad.service = &copyService
	if (integerAllocationContextRef((integerAllocationInvocationContext{mignotte: &bad, mignotteInvocation: bad.invocation})).roots(q.Integers())) != nil {
		t.Fatal("copied owner acquired authority")
	}
	s.manager.callbackDepth++
	if (c.roots(q.Integers())) != nil {
		t.Fatal("foreign callback acquired authority")
	}
	s.manager.callbackDepth--
	f.executing = false
	serviceMust(t, s.finishMignotteSource(&r, nil))
	serviceMust(t, s.cleanupMignotteInvocation(&r, true))
	next, e := s.beginMignotteInvocation()
	serviceMust(t, e)
	next.frame.constructed = 1
	if next.invocation == r.invocation || (c.roots(q.Integers())) != nil {
		t.Fatal("saved context retargeted reused embedded receipt")
	}
	before := *next.frame
	serviceMust(t, s.cleanupMignotteInvocation(&r, true))
	if !testNumericScopeEqual(*next.frame, before) {
		t.Fatal("stale owning acknowledgment changed current invocation")
	}
	next.frame.executing = false
	serviceMust(t, s.finishMignotteSource(&next, nil))
	serviceMust(t, s.cleanupMignotteInvocation(&next, true))
	serviceMust(t, s.CleanupRetained())
	mignotteScopePhysicalAudit(t, q, s)
}

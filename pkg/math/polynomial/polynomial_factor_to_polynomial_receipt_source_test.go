package polynomial

// Source: src/math/polynomial/polynomial.cpp:6832-6841,2582-2594.

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestPolynomialFactorToPolynomialActualChildReceipt(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	values := []unumeral.Int{unumeral.NewInt(7), unumeral.NewInt(3)}
	var r polynomialToPolynomialInvocationReceipt
	p, e := full.toPolynomialWithReceipt(2, values, 0, &r)
	serviceMust(t, e)
	if r.invocation == 0 || r.manager != full || r.frame != &full.toPolynomial {
		t.Fatal("actual conversion child identity")
	}
	serviceMust(t, full.core.IncRef(p))
	serviceMust(t, full.cleanupToPolynomialInvocation(&r, false))
	serviceMust(t, full.cleanupToPolynomialInvocation(&r, true))
	expected := polynomialQueryTerms(t, full.core, []int32{7, 3}, [][]MonomialPower{{}, {{0, 1}}})
	same, e := full.core.Equal(p, expected)
	serviceMust(t, e)
	if !same {
		t.Fatal("literal coefficient copy and source factory")
	}
	polynomialImpPhysicalAuditNamed(t, full, []Polynomial{p, expected}, nil, 0)
	serviceMust(t, full.core.DecRef(p))
}

func TestPolynomialFactorToPolynomialEveryBigReservationPrefix(t *testing.T) {
	success := false
	for prefix := int64(0); prefix < 96 && !success; prefix++ {
		t.Run(strconv.FormatInt(prefix, 10), func(t *testing.T) {
			full := polynomialPSCFullFixture(t)
			z := full.core.integer
			var a unumeral.Int
			one := unumeral.NewInt(1)
			serviceMust(t, z.Mul2k(&one, 521, &a))
			values := []unumeral.Int{a, unumeral.NewInt(3)}
			var r polynomialToPolynomialInvocationReceipt
			z.Budget().FailAfter(prefix)
			p, cause := full.toPolynomialWithReceipt(2, values, 0, &r)
			var returned []Polynomial
			if p != (Polynomial{}) {
				returned = []Polynomial{p}
			}
			polynomialImpPhysicalAuditNamed(t, full, returned, []*unumeral.Int{&a}, 0)
			if cause != nil && !numericPhysicalReservationRefusal(cause) {
				t.Fatal(cause)
			}
			if z.PendingAllocationObject() != (umemory.ObjectRef{}) && full.toPolynomial.numeric.Cell.Object != z.PendingAllocationObject() {
				t.Fatal("actual copy descriptor not held by exact conversion frame")
			}
			z.Budget().FailAfter(-1)
			if cause == nil {
				success = true
				serviceMust(t, full.core.IncRef(p))
				serviceMust(t, full.core.DecRef(p))
			}
			serviceMust(t, full.cleanupToPolynomialInvocation(&r, false))
			serviceMust(t, full.cleanupToPolynomialInvocation(&r, true))
			serviceMust(t, z.Del(&a))
		})
	}
	if !success {
		t.Fatal("actual first success absent")
	}
}

func TestPolynomialFactorToPolynomialActualFailureHistoryAndStaleContext(t *testing.T) {
	full := polynomialPSCFullFixture(t)
	var old polynomialToPolynomialInvocationReceipt
	_, cause := full.toPolynomialWithReceipt(2, []unumeral.Int{unumeral.NewInt(7)}, 0, &old)
	if !errors.Is(cause, arena.ErrBounds) {
		t.Fatal("reached missing coefficient", cause)
	}
	body := old.frame.bodyCause
	if body == nil {
		t.Fatal("actual body cause missing")
	}
	serviceMust(t, full.cleanupToPolynomialInvocation(&old, false))
	serviceMust(t, full.cleanupToPolynomialInvocation(&old, true))
	serviceMust(t, full.beginToPolynomial())
	current := full.toPolynomial.direct
	stale := integerAllocationInvocationContext{toPolynomial: &full.toPolynomial.direct, toPolynomialInvocation: old.invocation}
	if (numericIntegerManager_toPolynomialAllocationRoots(full.core.integer, &stale)) != nil {
		t.Fatal("saved context retargeted through reused embedded receipt")
	}
	now := integerAllocationInvocationContext{toPolynomial: &full.toPolynomial.direct, toPolynomialInvocation: current.invocation}
	if (numericIntegerManager_toPolynomialAllocationRoots(full.core.integer, &now)) == nil {
		t.Fatal("actual reached current context absent")
	}
	if len(full.toPolynomialHistory) != 1 || full.toPolynomialHistory[0].bodyCause != body {
		t.Fatal("own ACK/new source constructor erased old actual body history")
	}
	before := full.toPolynomial
	serviceMust(t, full.cleanupToPolynomialInvocation(&old, true))
	if !testNumericScopeEqual(full.toPolynomial, before) {
		t.Fatal("stale ACK touched new source scope")
	}
	full.toPolynomial.executing = false
	serviceMust(t, full.finishToPolynomial(nil))
	serviceMust(t, full.cleanupToPolynomialInvocation(&current, true))
	polynomialImpPhysicalAuditNamed(t, full, nil, nil, 0)
}

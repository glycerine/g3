package polynomial

// Source: src/math/polynomial/polynomial.cpp:5094-5114;
// src/util/mpz.cpp:2054-2069; src/util/scoped_numeral.h:31-33.

import (
	"errors"
	"testing"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestPolynomialScalarExactDivCompiledAssertionCapturesActualBoolNUM(t *testing.T) {
	full := polynomialExactDivDirectPageOneFixture(t)
	c := full.core
	before := c.integer.DividesNumeral.Invocation
	one := unumeral.NewInt(1)
	result, e := c.ExactDivScalar(c.unit, &one)
	serviceMust(t, e)
	serviceMust(t, c.IncRef(result))
	defer func() { serviceMust(t, c.DecRef(result)) }()
	child := &c.integer.DividesNumeral
	if !polynomialSourceDebug {
		if child.Invocation != before {
			t.Fatal("erased scalar assertion constructed bool NUM")
		}
		return
	}
	parent := child.ParentContext
	if parent.Caller == nil || parent.Caller.Policy != &c.exactDivision.numeric || c.exactDivision.numeric.caller.scalarExact != &c.exactDivision.scalarReceipt || c.exactDivision.numeric.caller.scalarExactInvocation != c.exactDivision.invocation {
		t.Fatal("actual scalar assertion NUM lost its exact captured parent", parent.Caller, c.exactDivision.numeric.caller.scalarExactInvocation, c.exactDivision.invocation)
	}
	if child.Invocation == before || !child.Constructed || !child.Ended || child.Active {
		t.Fatal("literal successful bool NUM lexical destructor frontier")
	}
}

// The compiled source assertion reaches Rem's actual BIG copy before rejecting
// the nondivisible coefficient. Native source erases this assertion entirely.
func TestPolynomialScalarExactDivCompiledAssertionBIGRefusalPrefixes(t *testing.T) {
	if !polynomialSourceDebug {
		return
	}
	polynomialThrowAssertionFixture(t)
	saw := false
	for prefix := int64(0); prefix < 100; prefix++ {
		full := polynomialExactDivDirectPageOneFixture(t)
		c, z := full.core, full.core.integer
		var a, d unumeral.Int
		words := make([]uint32, 2048)
		words[0], words[len(words)-1] = 1, 1
		serviceMust(t, z.SetLimbs(&a, words))
		words[len(words)-1] = 2
		serviceMust(t, z.SetLimbs(&d, words))
		p, e := c.MkConstantTakeCopy(&a)
		serviceMust(t, e)
		serviceMust(t, c.IncRef(p))
		z.Budget().FailAfter(prefix)
		result, body := c.ExactDivScalar(p, &d)
		polynomialImpPhysicalAuditNamed(t, full, []Polynomial{p, result}, []*unumeral.Int{&a, &d}, 0)
		if body == nil {
			t.Fatal("nondivisible source assertion accepted", prefix)
		}
		s, child := &c.exactDivision, &z.DividesNumeral
		if child.ParentContext.Caller == nil || child.ParentContext.Caller.Policy != &s.numeric || s.numeric.caller.scalarExact != &s.scalarReceipt || s.numeric.caller.scalarExactInvocation != s.invocation || s.boolNumeral.Frame != child || s.boolNumeral.Invocation != child.Invocation {
			t.Fatal("actual BIG bool parent lost", prefix)
		}
		pending := z.PendingCell
		raw, e := z.SourceAllocator().PendingRoots()
		serviceMust(t, e)
		acquired := pending.Object != (umemory.ObjectRef{}) || raw != ([3]umemory.Block{})

		if acquired || child.Pending(z) {
			saw = true
			if !s.active || !z.MatchesDividesNumeralInvocation(&s.boolNumeral) || pending.Object != (umemory.ObjectRef{}) && child.Numeric.Cell != pending || raw != ([3]umemory.Block{}) && child.Numeric.Allocator != raw {
				t.Fatal("actual bool refused acquisition lacks active scalar child", prefix)
			}
		}

		if !errors.Is(body, ErrPolynomialAssertion) && !numericPhysicalReservationRefusal(body) {
			t.Fatal("actual refusal or source assertion required", prefix, body)
		}
		// A failed fresh-NULL BIG copy may already be consumed by the literal
		// source destructor; its captured body cause and published scalar remain.
		if numericPhysicalReservationRefusal(child.BodyCause) && child.Numeral.NativeScalar() != 0 {
			saw = true
			if !child.Ended && !child.Pending(z) {
				t.Fatal("failed bool NUM lost its source cleanup frontier", prefix)
			}
		}
		savedBody, savedChild := s.bodyCause, child.BodyCause
		z.Budget().FailAfter(-1)
		serviceMust(t, c.CleanupExactDivision())
		if s.active || child.Active || s.bodyCause != savedBody || child.BodyCause != savedChild {
			t.Fatal("matched cleanup or immutable call history", prefix)
		}
		serviceMust(t, c.DecRef(p))
		serviceMust(t, z.Del(&a))
		serviceMust(t, z.Del(&d))
		if errors.Is(body, ErrPolynomialAssertion) {
			if !saw {
				t.Fatal("actual bool BIG acquisition refusal not exercised")
			}
			t.Log("compiled assertion first complete false result", prefix)
			return
		}
	}
	t.Fatal("actual bool assertion prefix campaign did not complete")
}

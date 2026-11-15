package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1019-1299.

import (
	"fmt"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
	"testing"
)

func TestUPolynomialFactorSquareFreeExactInvocationPrefixes(t *testing.T) {
	complete := false
	for ordinal := int64(0); ordinal < 512; ordinal++ {
		done := false
		okay := t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			z, s := preparationPhysicalFixture(t)
			m := s.manager
			var f upolyVectorState
			serviceMust(t, m.set(&f, []unumeral.Int{unumeral.NewInt(-6), unumeral.NewInt(1), unumeral.NewInt(1)}))
			out, e := m.NewFactors()
			serviceMust(t, e)
			var r upolyFactorSquareFreeInvocationReceipt
			defer func() {
				z.Integers().Budget().FailAfter(-1)
				serviceMust(t, s.cleanupFactorSquareFreeInvocation(&r, true))
				serviceMust(t, out.Close())
				serviceMust(t, m.reset(&f))
				m.freeBacking(&f)
			}()
			z.Integers().Budget().FailAfter(ordinal)
			completed, body := s.factorSquareFreeWithReceipt(m.stateView(&f), out, 1, DefaultUPolynomialFactorParams(), &r)
			primeTrialPhysicalAuditAll(t, z, s, []*UPolynomialFactors{out}, nil)
			cause := s.factorSquareFreeInvocationSourceCause(&r)
			own := s.cleanupFactorSquareFreeInvocation(&r, false)
			if own != nil && r.frame != nil && r.frame.ownCause != own {
				t.Fatal("first exact matched owning cause not captured", own, r.frame.ownCause)
			}
			primeTrialPhysicalAuditAll(t, z, s, []*UPolynomialFactors{out}, nil)
			z.Integers().Budget().FailAfter(-1)
			serviceMust(t, s.cleanupFactorSquareFreeInvocation(&r, false))
			primeTrialPhysicalAuditAll(t, z, s, []*UPolynomialFactors{out}, nil)
			serviceMust(t, s.cleanupFactorSquareFreeInvocation(&r, true))
			if s.factorSquareFreeInvocationSourceCause(&r) != cause {
				t.Fatal("exact first source history changed")
			}
			if body == nil {
				if !completed || out.count != 2 {
					t.Fatal("actual independent squarefree factors", completed, out.count)
				}
				done = true
			}
		})
		if !okay {
			return
		}
		if done {
			complete = true
			break
		}
	}
	if !complete {
		t.Fatal("no actual whole squarefree completion")
	}
}

func TestUPolynomialFactorSquareFreeCallerCheckpointGuardsAndHistory(t *testing.T) {
	var service *UPolynomialFactorizationService
	var receipt upolyFactorSquareFreeInvocationReceipt
	reached := false
	z, s := henselDirectFixtureWithCheckpoint(t, func() bool {
		if service != nil && service.scope != nil && !reached {
			reached = true
			c := service.scope
			before := *c
			for _, f := range []func() error{func() error { return service.cleanupFactorSquareFreeInvocation(&receipt, true) }, service.CleanupRetained, service.finishSquareFreeScopeSource, service.manager.Close, service.manager.SourceFinalize, service.manager.SetZ} {
				if e := f(); e != ErrUPolynomialCleanupPending || !reflect.DeepEqual(before, *c) {
					t.Errorf("actual caller checkpoint mutation before refusal: %v", e)
				}
			}
			return false
		}
		return true
	})
	service = s
	var f upolyVectorState
	serviceMust(t, s.manager.set(&f, []unumeral.Int{unumeral.NewInt(-6), unumeral.NewInt(1), unumeral.NewInt(1)}))
	out, e := s.manager.NewFactors()
	serviceMust(t, e)
	defer func() {
		serviceMust(t, s.cleanupFactorSquareFreeInvocation(&receipt, true))
		serviceMust(t, out.Close())
		serviceMust(t, s.manager.reset(&f))
		s.manager.freeBacking(&f)
	}()
	_, body := s.factorSquareFreeWithReceipt(s.manager.stateView(&f), out, 1, DefaultUPolynomialFactorParams(), &receipt)
	if !reached || body == nil {
		t.Fatal("actual checkpoint body refusal absent", reached, body)
	}
	if receipt.frame == nil || receipt.frame.bodyCause == nil {
		t.Fatal("reached actual frame/body tuple")
	}
	c := receipt.frame
	oldBody := c.bodyCause
	serviceMust(t, s.cleanupFactorSquareFreeInvocation(&receipt, false))
	serviceMust(t, s.cleanupFactorSquareFreeInvocation(&receipt, true))
	if c.bodyCause != oldBody || !c.retired {
		t.Fatal("callee retirement erased body history")
	}
	var next upolyFactorSquareFreeInvocationReceipt
	done, e := s.factorSquareFreeWithReceipt(s.manager.stateView(&f), out, 1, DefaultUPolynomialFactorParams(), &next)
	serviceMust(t, e)
	if !done || next.frame == nil || next.invocation == receipt.invocation {
		t.Fatal("actual next invocation", done, next)
	}
	before := *next.frame
	serviceMust(t, s.cleanupFactorSquareFreeInvocation(&receipt, true))
	if !reflect.DeepEqual(before, *next.frame) || c.bodyCause != oldBody {
		t.Fatal("stale receipt acquired new invocation")
	}
	copied := *s
	if e := copied.cleanupFactorSquareFreeInvocation(&next, true); e != ErrUPolynomialOwner {
		t.Fatal("copied service owner", e)
	}
	copiedFrame := *next.frame
	fake := next
	fake.frame = &copiedFrame
	if e := s.cleanupFactorSquareFreeInvocation(&fake, true); e != ErrUPolynomialOwner {
		t.Fatal("copied frame owner", e)
	}
	_ = z
}

func TestUPolynomialFactorSquareFreeIssuedHistoryAndArchiveRefusal(t *testing.T) {
	z, s := preparationPhysicalFixture(t)
	m := s.manager
	var input upolyVectorState
	serviceMust(t, m.set(&input, []unumeral.Int{unumeral.NewInt(-6), unumeral.NewInt(1), unumeral.NewInt(1)}))
	out, e := m.NewFactors()
	serviceMust(t, e)
	defer func() {
		z.Integers().Budget().FailAfter(-1)
		serviceMust(t, s.CleanupRetained())
		serviceMust(t, out.Close())
		serviceMust(t, m.reset(&input))
		m.freeBacking(&input)
	}()
	var r upolyFactorSquareFreeInvocationReceipt
	_, e = s.factorSquareFreeWithReceipt(m.stateView(&input), out, 1, DefaultUPolynomialFactorParams(), &r)
	serviceMust(t, e)
	// AUDIT.md math_polynomial_09-F1: a clean issued scope is released at
	// once (factor_square_free keeps only scoped locals) but stays matched.
	if r.frame == nil || !r.frame.retired || r.frame.header != 0 || !r.frame.released || s.squareFreeHistory != nil || !s.matchesSquareFreeHistory(&r) {
		t.Fatal("clean issued history retained a cold charge")
	}
	before := r
	oldFrame := *r.frame
	// A matching-looking fabricated frame is neither the real active member nor an issued history record.
	fake := &upolyFactorSquareFreeScope{service: s, manager: m, invocation: r.invocation, retired: true, header: 1}
	fake.self = fake
	forged := r
	forged.frame = fake
	if e := s.cleanupFactorSquareFreeInvocation(&forged, true); e != ErrUPolynomialOwner {
		t.Fatal("fabricated retired frame accepted", e)
	}
	z.Integers().Budget().FailAfter(0)
	_, refused := s.factorSquareFreeWithReceipt(m.stateView(&input), out, 1, DefaultUPolynomialFactorParams(), &r)
	if refused == nil || !reflect.DeepEqual(before, r) || !reflect.DeepEqual(oldFrame, *before.frame) {
		t.Fatal("refused real next reservation erased old issued history", refused)
	}
	primeTrialPhysicalAuditAll(t, z, s, []*UPolynomialFactors{out}, nil)
	z.Integers().Budget().FailAfter(-1)
	serviceMust(t, s.cleanupFactorSquareFreeInvocation(&r, true))
	var next upolyFactorSquareFreeInvocationReceipt
	_, e = s.factorSquareFreeWithReceipt(m.stateView(&input), out, 1, DefaultUPolynomialFactorParams(), &next)
	serviceMust(t, e)
	if next.invocation == r.invocation || !s.matchesSquareFreeHistory(&r) {
		t.Fatal("successful reuse erased charged history")
	}
	oldNext := *next.frame
	serviceMust(t, s.cleanupFactorSquareFreeInvocation(&r, true))
	if !reflect.DeepEqual(oldNext, *next.frame) {
		t.Fatal("old issued readonly record acquired new roots")
	}
}

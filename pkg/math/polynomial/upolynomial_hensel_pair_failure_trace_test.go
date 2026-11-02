//go:build g3debug && g3trace

package polynomial

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"strings"
	"testing"
)

func TestUPolynomialHenselPairFailureTraceOriginalInputs(t *testing.T) {
	for _, correct := range []bool{false, true} {
		name := "invalid-bezout"
		if correct {
			name = "correct"
		}
		t.Run(name, func(t *testing.T) {
			polynomialThrowAssertionFixture(t)
			_, s := preparationPhysicalFixture(t)
			m := s.manager
			s.config.FactorizationDebugTag = true
			a, b, c := upolySourceNew(t, m, []int32{0, 1}), upolySourceNew(t, m, []int32{1, 1}), upolySourceNew(t, m, []int32{3, 1, 1})
			uc, vc := []int32(nil), []int32(nil)
			if correct {
				uc, vc = []int32{-1}, []int32{1}
			}
			u, v := upolySourceNew(t, m, uc), upolySourceNew(t, m, vc)
			al, bl := upolySourceNew(t, m, nil), upolySourceNew(t, m, nil)
			w := henselPairTraceProbe{t: t, s: s, stages: map[uint8]bool{}}
			tags := diagnostic.NewTraceTagsWithWriter(&w)
			tags.Enable("polynomial::factorization::bughunt")
			serviceMust(t, m.modular.BindTrace(tags))
			t.Cleanup(func() { serviceMust(t, m.modular.BindTrace(nil)) })
			one, three := unumeral.NewInt(1), unumeral.NewInt(3)
			body := s.HenselLiftPair(&one, &three, &three, mustUPolyView(t, m, u), mustUPolyView(t, m, a), mustUPolyView(t, m, v), mustUPolyView(t, m, b), mustUPolyView(t, m, c), al, bl)
			if !correct {
				if !errors.Is(body, ErrUPolynomialAssertion) {
					t.Fatal("actual reached final conditional assertion", body)
				}
			} else {
				serviceMust(t, body)
			}
			if correct {
				if strings.Contains(w.text.String(), ":536 ") {
					t.Fatal("unreached pair failure diagnostic")
				}
			} else {
				expected := "sage: R.<x> = ZZ['x']\nsage: A = x\nsage: B = x + 1\nsage: C = x^2 + x + 3\nsage: test1 = C - AB\nsage: print test1.change_ring(GF(9))\nsage: print 'expected 0'\n"
				if !strings.Contains(w.text.String(), ":536 ") || !strings.Contains(w.text.String(), expected) || !w.checkBorrowed {
					t.Fatal("whole diagnostic must print original inputs", w.text.String())
				}
			}
			serviceMust(t, s.CleanupRetained())
		})
	}
}
func TestUPolynomialHenselPairFailureTraceExactBIGChild(t *testing.T) {
	polynomialThrowAssertionFixture(t)
	q, s := preparationPhysicalFixtureWithSlots(t, 256)
	m, z := s.manager, q.Integers()
	s.config.FactorizationDebugTag = true
	var n, n1, n3 unumeral.Int
	t.Cleanup(func() { serviceMust(t, z.Del(&n3)); serviceMust(t, z.Del(&n1)); serviceMust(t, z.Del(&n)) })
	serviceMust(t, z.SetString(&n, "1267650600228229401496703205376"))
	one, three := unumeral.NewInt(1), unumeral.NewInt(3)
	serviceMust(t, z.Add(&n, &one, &n1))
	serviceMust(t, z.Add(&n, &three, &n3))
	a, e := m.NewPolynomial()
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, m.DelPolynomial(&a)) })
	serviceMust(t, m.SetInts(a, []unumeral.Int{n, one}))
	b := upolySourceNew(t, m, []int32{1, 1})
	c, e := m.NewPolynomial()
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, m.DelPolynomial(&c)) })
	serviceMust(t, m.SetInts(c, []unumeral.Int{n3, n1, one}))
	u, v, al, bl := upolySourceNew(t, m, nil), upolySourceNew(t, m, nil), upolySourceNew(t, m, nil), upolySourceNew(t, m, nil)
	w := factorDegreeTraceFailure{t: t, z: z, limit: 256, held: make([]unumeral.Int, 256), trigger: "sage: A = "}
	t.Cleanup(func() {
		serviceMust(t, s.CleanupRetained())
		for i := 0; i < w.count; i++ {
			serviceMust(t, z.Del(&w.held[i]))
		}
	})
	tags := diagnostic.NewTraceTagsWithWriter(&w)
	tags.Enable("polynomial::factorization::bughunt")
	serviceMust(t, m.modular.BindTrace(tags))
	t.Cleanup(func() { serviceMust(t, m.modular.BindTrace(nil)) })
	body := s.HenselLiftPair(&one, &three, &three, mustUPolyView(t, m, u), mustUPolyView(t, m, a), mustUPolyView(t, m, v), mustUPolyView(t, m, b), mustUPolyView(t, m, c), al, bl)
	h := s.henselPairs
	if !errors.Is(body, arena.ErrSlotLimit) || !w.armed || h == nil || h.constructed != 7 || h.sourceRemaining != 7 || h.check.constructed != 3 || h.check.sourceRemaining != 3 || h.check.brCore == nil || h.check.bCore != nil || !m.displayReceiptPending(&h.trace) {
		t.Fatal("checker child/frontier", body, w.armed)
	}
	roots := []*unumeral.Int{&n, &n1, &n3}
	for i := 0; i < w.count; i++ {
		roots = append(roots, &w.held[i])
	}
	upolyHenselAuditPhysical(t, []*UPolynomialManager{m}, nil, upolyPhysicalExtraOwners{Services: []*UPolynomialFactorizationService{s}, RationalManagers: []*unumeral.RationalManager{q}, Integers: roots})
	serviceMust(t, s.cleanupHenselPairInvocation(&h.receipt, false))
	serviceMust(t, s.cleanupHenselPairInvocation(&h.receipt, true))
	if h.active || m.displayReceiptPending(&h.trace) {
		t.Fatal("exact checker child cleanup retained pair")
	}
}

type pairCheckRefusedTrace struct {
	m           *UPolynomialManager
	failed      bool
	laterWrites int
	before      uint64
	cause       error
}

func (w *pairCheckRefusedTrace) Write(p []byte) (int, error) {
	if w.failed {
		w.laterWrites++
		return 0, w.cause
	}
	if string(p) == "sage: R.<x> = ZZ['x']" {
		w.failed = true
		w.before = w.m.displayNonce
		return 0, w.cause
	}
	return len(p), nil
}
func TestUPolynomialHenselPairFailureTraceStreamFailureStillComputes(t *testing.T) {
	polynomialThrowAssertionFixture(t)
	_, s := preparationPhysicalFixture(t)
	m := s.manager
	s.config.FactorizationDebugTag = true
	a, b, c := upolySourceNew(t, m, []int32{0, 1}), upolySourceNew(t, m, []int32{1, 1}), upolySourceNew(t, m, []int32{3, 1, 1})
	u, v, al, bl := upolySourceNew(t, m, nil), upolySourceNew(t, m, nil), upolySourceNew(t, m, nil), upolySourceNew(t, m, nil)
	marker := errors.New("refused pair failure diagnostic")
	w := pairCheckRefusedTrace{m: m, cause: marker}
	tags := diagnostic.NewTraceTagsWithWriter(&w)
	tags.Enable("polynomial::factorization::bughunt")
	serviceMust(t, m.modular.BindTrace(tags))
	t.Cleanup(func() { serviceMust(t, m.modular.BindTrace(nil)) })
	one, three := unumeral.NewInt(1), unumeral.NewInt(3)
	body := s.HenselLiftPair(&one, &three, &three, mustUPolyView(t, m, u), mustUPolyView(t, m, a), mustUPolyView(t, m, v), mustUPolyView(t, m, b), mustUPolyView(t, m, c), al, bl)
	if !errors.Is(body, ErrUPolynomialAssertion) || errors.Is(body, marker) {
		t.Fatal("ostream failure changed conditional assertion", body)
	}
	if !w.failed || w.laterWrites != 0 || m.displayNonce-w.before != 3 {
		t.Fatal("badbit skipped actual displays or retried output", w.failed, w.laterWrites, m.displayNonce-w.before)
	}
	h := s.henselPairs
	if h.check.sourceRemaining != 0 || h.sourceRemaining != 0 || m.displayReceiptPending(&h.trace) {
		t.Fatal("stream refusal retained numeric child")
	}
	serviceMust(t, s.CleanupRetained())
}

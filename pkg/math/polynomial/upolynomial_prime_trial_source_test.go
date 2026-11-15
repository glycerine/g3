package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1097-1145;
// src/math/polynomial/upolynomial_factorization_int.h:42-52.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestUPolynomialPrimeTrialLiteralRoutes(t *testing.T) {
	for _, tc := range []struct {
		prime   int32
		route   uint8
		factors uint32
	}{{2, 0, 0}, {3, 2, 0}, {7, 1, 2}} {
		q, s := mignotteDirectFixture(t)
		m := s.manager
		c, e := s.newSquareFreeScope()
		serviceMust(t, e)
		c.constructed = 1
		serviceMust(t, m.set(&c.fpp, []unumeral.Int{unumeral.NewInt(-2), unumeral.NewInt(0), unumeral.NewInt(1)}))
		c.constructed = 2
		serviceMust(t, m.modular.SetInt32(&c.p, tc.prime))
		c.zp, e = NewRelatedUPolynomialManager(m)
		serviceMust(t, e)
		c.constructed = 3
		serviceMust(t, c.zp.SetZP(&c.p))
		c.best, e = c.zp.NewFactors()
		serviceMust(t, e)
		c.constructed = 4
		c.constructed = 5
		serviceMust(t, m.modular.SetInt32(&c.bestP, 2))
		c.constructed = 6
		serviceMust(t, c.ds.init(c.best))
		out, e := m.NewFactors()
		serviceMust(t, e)
		trials := uint32(0)
		route, e := s.factorPrimeTrial(c.zp, &c.fpp, out, c.best, 1, &c.p, &c.bestP, &c.ds, &trials)
		serviceMust(t, e)
		if route != tc.route || c.best.count != tc.factors {
			t.Fatal("literal GF trial route/count", tc, route, c.best.count)
		}
		if tc.route == 2 {
			if out.count != 1 || out.factors[0].size != 3 {
				t.Fatal("irreducible source push", out.Stats())
			}
			for i, want := range []unumeral.Int{unumeral.NewInt(-2), unumeral.NewInt(0), unumeral.NewInt(1)} {
				got, e := m.read(m.stateView(&out.factors[0]), uint32(i))
				serviceMust(t, e)
				if *got != want {
					t.Fatal("original Z factor altered", i, *got, want)
				}
			}
		}
		if tc.route == 1 {
			seenPlus, seenMinus := false, false
			for i := uint32(0); i < c.best.count; i++ {
				v := &c.best.factors[i]
				if v.size != 2 {
					t.Fatal("independent GF7 linear factors", v.size)
				}
				a, x := c.zp.read(c.zp.stateView(v), 0)
				serviceMust(t, x)
				b, x := c.zp.read(c.zp.stateView(v), 1)
				serviceMust(t, x)
				if *b != unumeral.NewInt(1) {
					t.Fatal("independent monic linear factor", *b)
				}
				switch *a {
				case unumeral.NewInt(3):
					seenPlus = true
				case unumeral.NewInt(-3):
					seenMinus = true
				default:
					t.Fatal("independent roots +/-3", *a)
				}
			}
			if !seenPlus || !seenMinus {
				t.Fatal("missing independent GF7 factor pair")
			}
		}
		if tc.route == 1 && (trials != 1 || c.bestP != unumeral.NewInt(7)) {
			t.Fatal("actual trial/swap saved prime", trials, c.bestP)
		}
		if tc.route == 0 && trials != 0 {
			t.Fatal("continue updated trial", trials)
		}
		serviceMust(t, out.Close())
		primeTrialPhysicalAudit(t, q, s)
		serviceMust(t, s.CleanupRetained())
	}
}

// Every owner is supplied by an actual constructor/root in this test. The
// store directory is counted once even when the actual related core borrows it.
func primeTrialPhysicalAudit(t *testing.T, q *unumeral.RationalManager, s *UPolynomialFactorizationService) {
	primeTrialPhysicalAuditExtra(t, q, s)
}
func primeTrialPhysicalAuditExtra(t *testing.T, q *unumeral.RationalManager, s *UPolynomialFactorizationService, extra ...*unumeral.Int) {
	primeTrialPhysicalAuditOwners(t, q, s, nil, extra...)
}
func primeTrialPhysicalAuditOwners(t *testing.T, q *unumeral.RationalManager, s *UPolynomialFactorizationService, externalFactors []*UPolynomialFactors, extra ...*unumeral.Int) {
	primeTrialPhysicalAuditAll(t, q, s, externalFactors, nil, extra...)
}
func primeTrialPhysicalAuditExternal(t *testing.T, q *unumeral.RationalManager, s *UPolynomialFactorizationService, external []umemory.Block, extra ...*unumeral.Int) {
	primeTrialPhysicalAuditAll(t, q, s, nil, external, extra...)
}
func primeTrialPhysicalAuditAll(t *testing.T, q *unumeral.RationalManager, s *UPolynomialFactorizationService, externalFactors []*UPolynomialFactors, external []umemory.Block, extra ...*unumeral.Int) {
	roots := append(extra, []*unumeral.Int{&q.NTmp, &q.AddTmp1, &q.AddTmp2, &q.AddMulTmp.Num, &q.AddMulTmp.Den, &q.LtTmp1.Num, &q.LtTmp1.Den, &q.LtTmp2.Num, &q.LtTmp2.Den}...)
	n := q.HeaderBytes + integerActualOwnedBytes(q.Integers()) + s.header + s.config.Primes.Stats().TotalBytes
	for q := s.frontendConstantCalls; q != nil; q = q.next {
		n += q.header
	}
	for h := s.frontendHistory; h != nil; h = h.next {
		n += h.header
	}
	for c := s.squareFreeHistory; c != nil; c = c.historyNext {
		n += c.header
	}
	for h := s.squareFreeDiagnosticHistory; h != nil; h = h.next {
		n += h.header
	}
	c := s.scope
	managers := []*UPolynomialManager{s.manager}
	if c != nil {
		n += c.header
		managers = append(managers, c.zp, c.zpe)
		for _, m := range c.hensel.numericOwners() {
			if m == nil || m.closed {
				continue
			}
			seen := false
			for _, named := range managers {
				if named == m {
					seen = true
					break
				}
			}
			if !seen {
				managers = append(managers, m)
			}
		}
		roots = append(roots, &c.p, &c.bestP, &c.gcdTmp, &c.leading, &c.trialContent, &c.mignotte.norm, &c.mignotte.bound, &c.mignotte.tmp, &c.trialHelper.tmp)
	}
	for h := s.henselPairs; h != nil; h = h.next {
		n += h.header
		roots = append(roots, &h.am, &h.check.br)
		if h.check.transportBound {
			roots = append(roots, &h.check.transport)
		}
		for _, m := range []*UPolynomialManager{h.check.brCore, h.check.bCore} {
			if m != nil && !m.closed {
				managers = append(managers, m)
			}
		}
		if h.rm != nil && !h.rm.closed {
			managers = append(managers, h.rm)
		}
	}
	// Explicit external factors are actual caller-owned managers even when no
	// service lexical scope has been constructed at an early public refusal.
	for _, fs := range externalFactors {
		if fs == nil || fs.manager == nil || fs.manager.closed {
			continue
		}
		seen := false
		for _, named := range managers {
			if named == fs.manager {
				seen = true
				break
			}
		}
		if !seen {
			managers = append(managers, fs.manager)
		}
	}
	stores := map[*UPolynomialCoefficientStore]bool{}
	appendState := func(m *UPolynomialManager, v *upolyVectorState) {
		if v.backing != 0 {
			values := m.values(v)
			for i := uint32(0); uint64(i) < uint64(len(values)); i++ {
				roots = append(roots, &values[i])
			}
		}
	}
	roots = append(roots, &s.frontend.content, &s.frontend.vectorNormalize.g, &s.frontend.vectorNormalize.old, &s.frontend.vectorNormalize.tmp, &s.frontend.modularPower.power, &s.frontend.quadratic.values[0], &s.frontend.quadratic.values[1], &s.frontend.quadratic.values[2], &s.frontend.quadratic.values[3], &s.frontend.flip.c, &s.frontend.image.old, &s.frontend.image.c, &s.frontend.remainder.am, &s.frontend.monic.lc, &s.frontend.monic.inv, &s.frontend.derivative.d, &s.frontend.divisibility.ratio, &s.frontend.primitive.cont)
	for r := s.frontend.derivative.retired; r != nil; r = r.next {
		roots = append(roots, &r.value)
		n += r.header
	}
	for i := range s.frontend.cra.values {
		roots = append(roots, &s.frontend.cra.values[i])
	}
	for i := range s.frontend.modularGCD.values {
		roots = append(roots, &s.frontend.modularGCD.values[i])
	}
	for _, v := range []*upolyVectorState{&s.frontend.quadratic.f1, &s.frontend.quadratic.f2, &s.frontend.quadratic.product, &s.frontend.pp, &s.frontend.prime, &s.frontend.A, &s.frontend.B, &s.frontend.D} {
		appendState(s.manager, v)
	}
	for _, m := range managers {
		if m == nil || m.closed {
			continue
		}
		appendDisplay := func(f *upolyDisplayScope) {
			n += f.header
			roots = append(roots, &f.numeral)
		}
		for f := m.displayFrames; f != nil; f = f.next {
			appendDisplay(f)
		}
		appendDisplay(&m.display)
		n += m.header + m.modular.Header
		roots = append(roots, &(*m.modular.BorrowModulus()), &m.modular.Lower, &m.modular.Upper, &m.modular.InvTmp1, &m.modular.InvTmp2, &m.modular.InvTmp3, &m.modular.DivTmp)
		for _, v := range []*upolyVectorState{&m.db, &m.dbab1, &m.dbab2, &m.tr, &m.push, &m.basic, &m.div1, &m.div2, &m.exactDiv, &m.gcd1, &m.gcd2, &m.cra, &m.mgcd[0], &m.mgcd[1], &m.mgcd[2], &m.mgcd[3], &m.mgcd[4], &m.mgcd[5], &m.sqf1, &m.sqf2, &m.pw} {
			appendState(m, v)
		}
		if !stores[m.store] {
			stores[m.store] = true
			n += m.store.header
		}
	}
	for h := s.henselPairs; h != nil; h = h.next {
		for i := uint8(2); i <= h.constructed; i++ {
			m, v := h.vector(i)
			appendState(m, v)
		}
	}
	for h := s.henselPairs; h != nil; h = h.next {
		for i := uint8(3); i <= h.check.constructed; i++ {
			if v := h.checkVector(i); v != nil {
				appendState(s.manager, v)
			}
		}
	}
	if c != nil {
		roots = append(roots, &c.primeTrial.transport, &c.hensel.am, &c.hensel.monicRaw[0], &c.hensel.monicRaw[1], &c.hensel.extendedScoped[0], &c.hensel.extendedScoped[1], &c.hensel.initialLC, &c.hensel.finalLCInv, &c.hensel.checkPower)
		if c.hensel.transportBound {
			roots = append(roots, &c.hensel.transport)
		}
		for _, v := range []*upolyVectorState{&c.fpp, &c.trial, &c.quotient, &c.primeTrial.fzp} {
			appendState(s.manager, v)
		}
		for _, group := range [][]upolyVectorState{c.hensel.outer[:], c.hensel.quadratic[:], c.hensel.iteration[:], c.hensel.extended[:], c.hensel.extendedDebug[:], c.hensel.quadraticCheck[:], c.hensel.individual[:], c.hensel.checkVectors[:]} {
			for i := range group {
				appendState(s.manager, &group[i])
			}
		}
	}
	factorOwners := append([]*UPolynomialFactors{}, externalFactors...)
	if c != nil {
		factorOwners = append(factorOwners, c.best, c.lifted, c.primeTrial.current)
	}
	namedFactors := map[*UPolynomialFactors]bool{}
	for _, fs := range factorOwners {
		if fs == nil || namedFactors[fs] {
			continue
		}
		namedFactors[fs] = true
		n += fs.header
		roots = append(roots, &fs.constant)
		for i := uint32(0); i < fs.count; i++ {
			appendState(fs.manager, &fs.factors[i])
		}
	}

	roots = append(roots, &s.diagnostic.numeral, &s.preparation.numeral)
	mignotteNamedIntegerPhysicalAudit(t, q.Integers(), external, roots...)
	if n != q.Budget().Used() {
		t.Fatal("actual named prime trial cold ownership", n, q.Budget().Used())
	}
}

// The helper has a RAW p_i, not a scoped numeral. A failed SET or vector
// publication must retain its actual transport until explicit owning abort.
func TestUPolynomialPrimeTrialToZPRawEveryPrefix(t *testing.T) {
	for ordinal := int64(0); ordinal < 256; ordinal++ {
		q, s := mignotteDirectFixture(t)
		m := s.manager
		c, e := s.newSquareFreeScope()
		serviceMust(t, e)
		c.constructed = 1
		serviceMust(t, m.set(&c.fpp, []unumeral.Int{unumeral.NewInt(1), q.Integers().Two64, unumeral.NewInt(1)}))
		c.constructed = 2
		serviceMust(t, m.modular.SetInt32(&c.p, 7))
		c.zp, e = NewRelatedUPolynomialManager(m)
		serviceMust(t, e)
		c.constructed = 3
		serviceMust(t, c.zp.SetZP(&c.p))
		r, e := s.beginPrimeTrialInvocation(c.zp)
		serviceMust(t, e)
		r.frame.constructed = 1
		q.Budget().FailAfter(ordinal)
		e = c.zp.toZPCopyWithPrimeReceipt(m.stateView(&c.fpp), &r.frame.fzp, &r)
		r.frame.executing = false
		primeTrialPhysicalAudit(t, q, s)
		if e != nil && r.frame.transportBound && r.frame.fzp.size > 0 {
			first, x := c.zp.read(c.zp.stateView(&r.frame.fzp), 0)
			serviceMust(t, x)
			if *first != unumeral.NewInt(1) {
				t.Fatal("completed prefix lost before owning", *first)
			}
		}
		if e == nil && r.frame.transportBound {
			t.Fatal("successful move retained RAW authority")
		}
		rawBefore, boundBefore := r.frame.transport, r.frame.transportBound
		serviceMust(t, s.cleanupPrimeTrialInvocation(&r, false))
		if r.frame.transport != rawBefore || r.frame.transportBound != boundBefore {
			t.Fatal("source-completed phase consumed unjoined RAW")
		}
		primeTrialPhysicalAudit(t, q, s)
		q.Budget().FailAfter(-1)
		serviceMust(t, s.cleanupPrimeTrialInvocation(&r, true))
		primeTrialPhysicalAudit(t, q, s)
		serviceMust(t, s.CleanupRetained())
		if e == nil {
			t.Log("first actual completed prefix", ordinal)
			return
		}
	}
	t.Fatal("no actual completed source prefix")
}

// Actual destroyed-old/refused-new Copy qualifies the checked owner-primitive
// guard domain. It does not claim this dangling constant is mathematical trial
// failure. The younger current_fs source DT must preserve older f_pp_zp.
func TestUPolynomialPrimeTrialCurrentDestructorStopsOlderVector(t *testing.T) {
	q, s := mignotteDirectFixture(t)
	m := s.manager
	c, e := s.newSquareFreeScope()
	serviceMust(t, e)
	c.constructed = 1
	serviceMust(t, m.set(&c.fpp, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(1)}))
	c.constructed = 2
	serviceMust(t, m.modular.SetInt32(&c.p, 7))
	c.zp, e = NewRelatedUPolynomialManager(m)
	serviceMust(t, e)
	c.constructed = 3
	serviceMust(t, c.zp.SetZP(&c.p))
	r, e := s.beginPrimeTrialInvocation(c.zp)
	serviceMust(t, e)
	f := r.frame
	f.constructed = 1
	serviceMust(t, c.zp.toZPCopyWithPrimeReceipt(m.stateView(&c.fpp), &f.fzp, &r))
	f.current, e = c.zp.NewFactors()
	serviceMust(t, e)
	f.constructed = 2
	serviceMust(t, q.Integers().Copy(&f.current.constant, &q.Integers().Two64))
	var large unumeral.Int
	words := make([]uint32, 80)
	words[79] = 1
	serviceMust(t, q.Integers().SetLimbs(&large, words))
	old := f.fzp
	q.Budget().FailAfter(0)
	body := q.Integers().Copy(&f.current.constant, &large)
	if !numericPhysicalReservationRefusal(body) || (f.current.constant).Cell != q.Integers().FailedRef {
		t.Fatal("actual destroyed old", body)
	}
	f.executing = false
	stop := s.finishPrimeTrialSource(&r, body)
	if !errors.Is(stop, body) || !errors.Is(stop, arena.ErrInvalidSlot) || f.fzp != old || f.current == nil {
		t.Fatal("younger current DT drained older fzp", stop)
	}
	// Immediate graph includes the real caller-owned larger input.
	primeTrialPhysicalAuditExtra(t, q, s, &large)
	q.Budget().FailAfter(-1)
	serviceMust(t, s.cleanupPrimeTrialInvocation(&r, false))
	serviceMust(t, s.cleanupPrimeTrialInvocation(&r, true))
	if !errors.Is(stop, body) || f.sourceCause == nil {
		t.Fatal("owning erased immutable history")
	}
	if s.primeTrialInvocationSourceCause(&r) != f.sourceCause {
		t.Fatal("inactive exact history unavailable")
	}
	if repeated := s.finishPrimeTrialSource(&r, nil); !errors.Is(repeated, body) || !errors.Is(repeated, arena.ErrInvalidSlot) {
		t.Fatal("source retry erased first history", repeated)
	}
	next, x := s.beginPrimeTrialInvocation(c.zp)
	serviceMust(t, x)
	next.frame.constructed = 1
	serviceMust(t, c.zp.toZPCopyWithPrimeReceipt(m.stateView(&c.fpp), &next.frame.fzp, &next))
	next.frame.executing = false
	serviceMust(t, s.finishPrimeTrialSource(&next, nil))
	if !errors.Is(stop, body) || !errors.Is(stop, arena.ErrInvalidSlot) {
		t.Fatal("actual new invocation mutated earlier returned diagnostic", stop)
	}
	serviceMust(t, q.Integers().Del(&large))
	primeTrialPhysicalAudit(t, q, s)
	serviceMust(t, s.CleanupRetained())
}

func TestUPolynomialPrimeTrialEveryActualReservation(t *testing.T) {
	for ordinal := int64(0); ordinal < 2048; ordinal++ {
		q, s := mignotteDirectFixture(t)
		m := s.manager
		c, e := s.newSquareFreeScope()
		serviceMust(t, e)
		c.constructed = 1
		serviceMust(t, m.set(&c.fpp, []unumeral.Int{unumeral.NewInt(-2), unumeral.NewInt(0), unumeral.NewInt(1)}))
		c.constructed = 2
		serviceMust(t, m.modular.SetInt32(&c.p, 7))
		c.zp, e = NewRelatedUPolynomialManager(m)
		serviceMust(t, e)
		c.constructed = 3
		serviceMust(t, c.zp.SetZP(&c.p))
		c.best, e = c.zp.NewFactors()
		serviceMust(t, e)
		c.constructed = 4
		c.constructed = 5
		serviceMust(t, m.modular.SetInt32(&c.bestP, 2))
		c.constructed = 6
		serviceMust(t, c.ds.init(c.best))
		out, e := m.NewFactors()
		serviceMust(t, e)
		trials := uint32(0)
		q.Budget().FailAfter(ordinal)
		route, body := s.factorPrimeTrial(c.zp, &c.fpp, out, c.best, 1, &c.p, &c.bestP, &c.ds, &trials)
		primeTrialPhysicalAuditOwners(t, q, s, []*UPolynomialFactors{out})
		q.Budget().FailAfter(-1)
		serviceMust(t, out.Close())
		serviceMust(t, s.CleanupRetained())
		if body == nil {
			if route != 1 || trials != 1 {
				t.Fatal(route, trials)
			}
			t.Log("first actual full trial success", ordinal)
			return
		}
	}
	t.Fatal("no actual whole trial completion")
}

func TestUPolynomialPrimeTrialCapturedContextRejectsReuseCopiesAndForeign(t *testing.T) {
	q, s := mignotteDirectFixture(t)
	m := s.manager
	c, e := s.newSquareFreeScope()
	serviceMust(t, e)
	c.zp, e = NewRelatedUPolynomialManager(m)
	serviceMust(t, e)
	c.constructed = 3
	r, e := s.beginPrimeTrialInvocation(c.zp)
	serviceMust(t, e)
	r.frame.constructed = 1
	r.frame.transportBound = true
	context := numericIntegerModularManager_primeTrialAllocationContext(c.zp.modular, &r)
	if (context.roots(q.Integers())) != (&r.frame.numeric) {
		t.Fatal("actual embedded context refused")
	}
	copied := *r.frame
	bad := r
	bad.frame = &copied
	if (integerAllocationContextRef((integerAllocationInvocationContext{primeTrial: &bad, primeTrialInvocation: bad.invocation})).roots(q.Integers())) != nil {
		t.Fatal("copied frame acquired")
	}
	c.zp.callbackDepth++
	if (context.roots(q.Integers())) != nil {
		t.Fatal("callback acquired")
	}
	c.zp.callbackDepth--
	foreign, e := NewRelatedUPolynomialManager(m)
	serviceMust(t, e)
	if (integerAllocationContextRef(numericIntegerModularManager_primeTrialAllocationContext(foreign.modular, &r)).roots(q.Integers())) != nil {
		t.Fatal("foreign receiver acquired")
	}
	serviceMust(t, foreign.Close())
	r.frame.executing = false
	serviceMust(t, s.cleanupPrimeTrialInvocation(&r, false))
	serviceMust(t, s.cleanupPrimeTrialInvocation(&r, true))
	next, e := s.beginPrimeTrialInvocation(c.zp)
	serviceMust(t, e)
	next.frame.constructed = 1
	next.frame.transportBound = true
	// The context retained the old actual receipt address and immutable nonce.
	r = next
	if (context.roots(q.Integers())) != nil {
		t.Fatal("reused receipt pointer retargeted old context")
	}
	next.frame.executing = false
	serviceMust(t, s.cleanupPrimeTrialInvocation(&next, false))
	serviceMust(t, s.cleanupPrimeTrialInvocation(&next, true))
	serviceMust(t, s.CleanupRetained())
}

// The generic source helper accepts a true large positive modulus. The trial
// loop's uint32 prime selection cannot itself produce this modulus, so this
// proves reached helper RAW publication, not ordinary large-prime trial input.
func TestUPolynomialPrimeToZPNativePushAndPublishedRaw(t *testing.T) {
	found := false
	for ordinal := int64(0); ordinal < 1024; ordinal++ {
		q, s := mignotteDirectFixture(t)
		m := s.manager
		c, e := s.newSquareFreeScope()
		serviceMust(t, e)
		c.constructed = 1
		serviceMust(t, m.set(&c.fpp, []unumeral.Int{unumeral.NewInt(1), unumeral.NewInt(0), q.Integers().Two64}))
		c.constructed = 2
		serviceMust(t, q.Integers().SetLimbs(&c.p, []uint32{0xffffffff, 0xffffffff, 0xffffffff, 0x7fffffff}))
		c.zp, e = NewRelatedUPolynomialManager(m)
		serviceMust(t, e)
		c.constructed = 3
		serviceMust(t, c.zp.SetZP(&c.p))
		r, e := s.beginPrimeTrialInvocation(c.zp)
		serviceMust(t, e)
		r.frame.constructed = 1
		q.Budget().FailAfter(ordinal)
		body := c.zp.toZPCopyWithPrimeReceipt(m.stateView(&c.fpp), &r.frame.fzp, &r)
		r.frame.executing = false
		primeTrialPhysicalAudit(t, q, s)
		if body == nil {
			if r.frame.fzp.size != 3 || r.frame.transportBound || r.frame.transport != (unumeral.Int{}) {
				t.Fatal("native push must move RAW into coefficient ownership", r.frame.fzp, r.frame.transport)
			}
			actual, x := c.zp.read(c.zp.stateView(&r.frame.fzp), 2)
			serviceMust(t, x)
			if !q.Integers().IsLiveCellToken(actual.NativeCellToken()) {
				t.Fatal("BIG coefficient not owned", *actual)
			}
			same, x := q.Integers().Equal(actual, &q.Integers().Two64)
			serviceMust(t, x)
			if !same {
				t.Fatal("actual moved RAW value", *actual)
			}
			found = true
		}
		q.Budget().FailAfter(-1)
		serviceMust(t, s.cleanupPrimeTrialInvocation(&r, false))
		serviceMust(t, s.cleanupPrimeTrialInvocation(&r, true))
		primeTrialPhysicalAudit(t, q, s)
		serviceMust(t, s.CleanupRetained())
		if body == nil {
			if !found {
				t.Fatal("no actual BIG RAW publication at native push completion", ordinal)
			}
			t.Log("completed helper success", ordinal)
			return
		}
	}
	t.Fatal("no actual helper completion")
}

func TestUPolynomialPrimeTrialEnclosingOwnContinuationEveryHelperPrefix(t *testing.T) {
	for ordinal := int64(0); ordinal < 512; ordinal++ {
		q, s := mignotteDirectFixture(t)
		m := s.manager
		c, e := s.newSquareFreeScope()
		serviceMust(t, e)
		c.constructed = 1
		serviceMust(t, m.set(&c.fpp, []unumeral.Int{unumeral.NewInt(1), q.Integers().Two64, unumeral.NewInt(1)}))
		c.constructed = 2
		serviceMust(t, m.modular.SetInt32(&c.p, 7))
		c.zp, e = NewRelatedUPolynomialManager(m)
		serviceMust(t, e)
		c.constructed = 3
		serviceMust(t, c.zp.SetZP(&c.p))
		r, e := s.beginPrimeTrialInvocation(c.zp)
		serviceMust(t, e)
		r.frame.constructed = 1
		q.Budget().FailAfter(ordinal)
		body := c.zp.toZPCopyWithPrimeReceipt(m.stateView(&c.fpp), &r.frame.fzp, &r)
		r.frame.executing = false
		_ = s.finishPrimeTrialSource(&r, body)
		primeTrialPhysicalAudit(t, q, s)
		q.Budget().FailAfter(-1)
		if e = s.CleanupRetained(); e != nil {
			t.Fatal("actual enclosing owning continuation", ordinal, e, r.frame.normalize, r.frame.numeric, c.constructed)
		}
		if body == nil {
			t.Log("first source helper completion", ordinal)
			return
		}
	}
	t.Fatal("no helper completion")
}

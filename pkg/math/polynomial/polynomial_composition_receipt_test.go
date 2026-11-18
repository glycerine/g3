package polynomial

// Source: src/math/polynomial/polynomial.cpp:2481-2495,4693-4890.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestPolynomialCompositionEveryReservation(t *testing.T) {
	for op := 0; op < 7; op++ {
		t.Run(fmt.Sprint(op), func(t *testing.T) {
			success := false
			for ordinal := int64(0); ordinal < 150; ordinal++ {
				t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
					m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{2, [3]uint32{0, 3, 0}}, {-3, [3]uint32{0, 1, 0}}, {5, [3]uint32{}}})
					r, e := NewPolynomialRef(m.core)
					serviceMust(t, e)
					defer func() {
						m.core.integer.Budget().FailAfter(-1)
						serviceMust(t, m.CleanupComposition())
						serviceMust(t, r.Close())
					}()
					var got Polynomial
					c := unumeral.NewInt(3)
					m.core.integer.Budget().FailAfter(ordinal)
					switch op {
					case 0:
						got, e = m.ComposeReciprocal(p)
					case 1:
						got, e = m.ComposeXDivY(p, 0)
					case 2:
						got, e = m.ComposeVariable(p, 0)
					case 3:
						got, e = m.ComposeMinusX(p)
					case 4:
						e = m.ComposeXMinusY(p, 0, r)
					case 5:
						e = m.ComposeXPlusY(p, 2, r)
					case 6:
						e = m.Compose(p, m.core.unit, r)
					}
					polys := []Polynomial{p, r.root.value, r.root.incoming}
					if got != (Polynomial{}) {
						polys = append(polys, got)
					}
					polynomialImpPhysicalAuditNamed(t, m, polys, []*unumeral.Int{&c}, r.header)
					if e == nil {
						if op == 6 {
							a, x := m.core.coefficientCore(r.root.value, 0)
							serviceMust(t, x)
							if (a).Cell != 0 || a.NativeScalar() != 4 {
								t.Fatal("independent p(1)=2-3+5", a)
							}
						}
						success = true
					} else if !numericPhysicalReservationRefusal(e) {
						t.Fatal("exact operation reservation transport", e)
					}
					m.core.integer.Budget().FailAfter(-1)
					serviceMust(t, m.CleanupComposition())
					if m.core.pending.value != (Polynomial{}) || m.core.pending.raw != (umemory.ObjectRef{}) || m.core.singletonReference || m.core.constantScope.active || m.core.generatedActive || m.core.integer.PendingAllocationObject() != (umemory.ObjectRef{}) {
						t.Fatal("operation abort omitted reached callee", m.core.pending, m.core.singletonReference, m.core.constantScope.active)
					}
					if got != (Polynomial{}) {
						serviceMust(t, m.core.IncRef(got))
						serviceMust(t, m.core.DecRef(got))
					}
					polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, r.root.value, r.root.incoming}, []*unumeral.Int{&c}, r.header)
				})
				if success {
					break
				}
			}
			if !success {
				t.Fatal("complete source operation never reached success")
			}
		})
	}
}

func TestPolynomialCompositionBigScalarOverloadEveryReservation(t *testing.T) {
	success, physical := false, false
	for ordinal := int64(0); ordinal < 180; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{2, 0, 0}}, {7, [3]uint32{}}})
			as := [3]unumeral.Int{unumeral.NewInt(5), {}, {}}
			digits := make([]uint32, 80)
			digits[0], digits[79] = 7, 1
			serviceMust(t, m.core.integer.SetLimbs(&as[1], digits))
			digits[0] = 11
			serviceMust(t, m.core.integer.SetLimbs(&as[2], digits))
			big, e := m.core.MkUnivariateTake(0, 2, as[:])
			serviceMust(t, e)
			serviceMust(t, m.core.IncRef(big))
			r, e := NewPolynomialRef(m.core)
			serviceMust(t, e)
			holder := unumeral.Int{}
			serviceMust(t, m.core.integer.SetLimbs(&holder, digits))
			defer func() {
				m.core.integer.Budget().FailAfter(-1)
				serviceMust(t, m.CleanupComposition())
				serviceMust(t, r.Close())
				serviceMust(t, m.core.integer.Del(&holder))
				serviceMust(t, m.core.CleanupValue(&big))
			}()
			m.core.integer.Budget().FailAfter(ordinal)
			e = m.Compose(big, m.core.zero, r)
			polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, big, r.root.value, r.root.incoming}, []*unumeral.Int{&holder}, r.header)
			if e == nil {
				success = true
			} else if !numericPhysicalReservationRefusal(e) {
				t.Fatal("literal scalar overload refusal", e)
			}
			if m.core.integer.PendingAllocationObject() != (umemory.ObjectRef{}) {
				physical = true
			}
			m.core.integer.Budget().FailAfter(-1)
			serviceMust(t, m.CleanupComposition())
			if m.core.integer.PendingAllocationObject() != (umemory.ObjectRef{}) || m.core.constantScope.active || m.core.pending.value != (Polynomial{}) || m.core.singletonReference {
				t.Fatal("operation abort lacks actual scalar overload leaf", m.core.integer.PendingAllocationObject(), m.core.constantScope.active)
			}
			polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, big, r.root.value, r.root.incoming}, []*unumeral.Int{&holder}, r.header)
			if success {
				a, e := m.core.coefficientCore(r.root.value, 0)
				serviceMust(t, e)
				n, e := m.core.integer.Int64(&a)
				serviceMust(t, e)
				if n != 5 {
					t.Fatal("p(0) independent constant", n)
				}
			}
		})
		if success {
			break
		}
	}
	if !success || !physical {
		t.Fatal("full big scalar prefix", success, physical)
	}
}

func TestPolynomialCompositionRawZeroCohortAndBorrowedExisting(t *testing.T) {
	for _, borrowed := range []bool{false, true} {
		t.Run(fmt.Sprint(borrowed), func(t *testing.T) {
			m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{}}})
			serviceMust(t, m.core.modular.SetZPUint64(2))
			prior := Monomial{}
			if borrowed {
				var e error
				prior, e = m.core.monomials.MkVariable(0, 3)
				serviceMust(t, e)
			}
			as := [4]unumeral.Int{{}, {}, {}, unumeral.NewInt(1)}
			ms := [4]Monomial{}
			for i, d := range []uint32{5, 2, 1, 0} {
				var e error
				ms[i], e = m.core.monomials.MkVariable(0, d)
				serviceMust(t, e)
				serviceMust(t, m.core.monomials.IncRef(ms[i]))
			}
			raw, e := m.core.MkCoreTake(as[:], ms[:])
			serviceMust(t, e)
			serviceMust(t, m.core.IncRef(raw))
			got, e := m.ComposeReciprocal(raw)
			serviceMust(t, e)
			serviceMust(t, m.core.IncRef(got))
			t.Cleanup(func() {
				serviceMust(t, m.CleanupComposition())
				serviceMust(t, m.core.DecRef(got))
				serviceMust(t, m.core.DecRef(raw))
				if prior != (Monomial{}) {
					serviceMust(t, m.core.monomials.CleanupValue(&prior))
				}
			})
			want := 2
			if borrowed {
				want = 1
			}
			if len(m.composition.orphans) != want {
				t.Fatal("created-only zero cohort", len(m.composition.orphans), want)
			}
			polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, raw, got}, nil, 0)
			for _, a := range m.composition.orphans {
				rc, e := m.core.monomialRefCount(a)
				serviceMust(t, e)
				if rc != 0 {
					t.Fatal("source zero add acquired INC", rc)
				}
			}
			if prior != (Monomial{}) {
				rc, e := m.core.monomialRefCount(prior)
				serviceMust(t, e)
				if rc != 0 {
					t.Fatal("borrowed caller zero-ref changed")
				}
			}
			transferred := m.composition.orphans[0]
			serviceMust(t, m.core.monomials.IncRef(transferred))
			if m.composition.orphans[0] != (Monomial{}) {
				t.Fatal("later actual INC did not transfer exact created authority")
			}
			serviceMust(t, m.CleanupComposition())
			rc, e := m.core.monomialRefCount(transferred)
			serviceMust(t, e)
			if rc != 1 {
				t.Fatal("own abort destroyed transferred caller root", rc)
			}
			serviceMust(t, m.core.monomials.DecRef(transferred))
			if prior != (Monomial{}) {
				rc, e := m.core.monomialRefCount(prior)
				serviceMust(t, e)
				if rc != 0 {
					t.Fatal("own abort swept preexisting zero-ref caller")
				}
			}
			polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, raw, got}, nil, 0)
		})
	}
}

func TestPolynomialCompositionCancellationRestoresActualDegreeMap(t *testing.T) {
	success := false
	for limit := uint32(0); limit < 40; limit++ {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			m, p, control := polynomialEvalFixture(t, []polynomialEvalTerm{{3, [3]uint32{4, 0, 0}}, {-2, [3]uint32{2, 0, 0}}, {5, [3]uint32{}}})
			q := polynomialCompositionTerms(t, m, []polynomialEvalTerm{{1, [3]uint32{1, 0, 0}}, {1, [3]uint32{0, 1, 0}}})
			r, e := NewPolynomialRef(m.core)
			serviceMust(t, e)
			defer func() { serviceMust(t, m.CleanupComposition()); serviceMust(t, r.Close()) }()
			control.checkpoints, control.cooperation, control.limit = 0, 0, limit
			cause := m.Compose(p, q, r)
			polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, q, r.root.value, r.root.incoming}, nil, r.header)
			if cause == nil {
				success = true
			} else if cause != polynomialCancellationError {
				t.Fatal("actual checkpoint frontier", cause)
			}
			if control.cooperation > control.checkpoints {
				t.Fatal("cooperation without checkpoint")
			}
			n := uint32(len(m.degree2pos))
			for i := uint32(0); i < n; i++ {
				pos, e := m.compositionDegreePosition(i)
				serviceMust(t, e)
				if pos != ^uint32(0) {
					t.Fatal("completed source scoped degree destructor omitted", i, pos)
				}
			}
			if m.composition.degreeCompleted || m.composition.aCompleted {
				t.Fatal("completed source lexical fields remain")
			}
			serviceMust(t, m.CleanupComposition())
		})
		if success {
			break
		}
	}
	if !success {
		t.Fatal("all checkpoint prefixes require final success")
	}
}

func TestPolynomialCompositionSameVariableBigCopyEveryReservation(t *testing.T) {
	success, physical := false, false
	for ordinal := int64(0); ordinal < 70; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{1, 0, 0}}})
			digits := make([]uint32, 80)
			digits[0], digits[79] = 5, 1
			as := [2]unumeral.Int{{}, unumeral.NewInt(1)}
			serviceMust(t, m.core.integer.SetLimbs(&as[0], digits))
			big, e := m.core.MkUnivariateTake(0, 1, as[:])
			serviceMust(t, e)
			serviceMust(t, m.core.IncRef(big))
			holder := unumeral.Int{}
			serviceMust(t, m.core.integer.SetLimbs(&holder, digits))
			r, e := NewPolynomialRef(m.core)
			serviceMust(t, e)
			defer func() {
				m.core.integer.Budget().FailAfter(-1)
				serviceMust(t, m.CleanupComposition())
				serviceMust(t, r.Close())
				serviceMust(t, m.core.integer.Del(&holder))
				serviceMust(t, m.core.CleanupValue(&big))
			}()
			m.core.integer.Budget().FailAfter(ordinal)
			e = m.ComposeXMinusY(big, 0, r)
			polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, big, r.root.value, r.root.incoming}, []*unumeral.Int{&holder}, r.header)
			if e == nil {
				success = true
			} else if !numericPhysicalReservationRefusal(e) {
				t.Fatal(e)
			}
			if m.core.integer.PendingAllocationObject() != (umemory.ObjectRef{}) {
				physical = true
			}
			m.core.integer.Budget().FailAfter(-1)
			serviceMust(t, m.CleanupComposition())
			if m.core.integer.PendingAllocationObject() != (umemory.ObjectRef{}) || m.core.pending.value != (Polynomial{}) {
				t.Fatal("same-variable actual Coeff leaf omitted by operation abort")
			}
			if e == nil {
				a, e := m.core.coefficientCore(r.root.value, 0)
				serviceMust(t, e)
				for i, d := range digits {
					v, e := m.core.integer.Cell((a).Cell).Backing.ReadWord(m.core.integer.SourceAllocator(), 8+4*uint64(i))
					serviceMust(t, e)
					if v != d {
						t.Fatal("literal constant copied coefficient", i, v, d)
					}
				}
			}
		})
		if success {
			break
		}
	}
	if !success || !physical {
		t.Fatal("complete reached Coeff prefix", success, physical)
	}
}

// Actual lexical owner primitives select the bounded signed-host TLS domain;
// this is not ordinary exhaustion or an allocation-limit free refusal.
func TestPolynomialCompositionStoppedAuxiliaryPreservesDegreeScope(t *testing.T) {
	budget := arena.NewBudget(1 << 25)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, SizeTBits: 64, ThreadLocal: true})
	serviceMust(t, e)
	cfg := serviceConfig(budget).Integers
	cfg.Memory = rt
	cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
	serviceMust(t, e)
	t.Cleanup(func() {
		contextTLSFinish(t, rt, q.CloseChecked)
		serviceMust(t, rt.Close())
		if budget.Used() != 0 {
			t.Fatal("whole actual graph refund", budget.Used())
		}
	})
	m, p, _ := polynomialEvalFixtureWithRationals(t, q, []polynomialEvalTerm{{1, [3]uint32{3, 0, 0}}, {2, [3]uint32{}}})
	defer func() {
		if !m.closed {
			contextTLSFinish(t, rt, m.CleanupComposition)
			contextTLSFinish(t, rt, func() error { return m.core.CleanupValue(&p) })
			contextTLSFinish(t, rt, m.CloseChecked)
		}
	}()
	// Source successful buffers must be warm before signed-domain free. These
	// real source calls isolate the lexical numeral free from cold freelist growth.
	warm, e := m.core.MkVariable(1, 1)
	serviceMust(t, e)
	serviceMust(t, m.core.IncRef(warm))
	serviceMust(t, m.core.DecRef(warm))
	serviceMust(t, m.beginComposition())
	s := &m.composition
	serviceMust(t, m.saveCompositionDegrees(p))
	s.degreeInput = p
	s.degreeCompleted = true
	s.aCompleted = true
	serviceMust(t, q.Integers().SetLimbs(&s.a, []uint32{1, 1}))
	serviceMust(t, q.Integers().EnsureOwnedCapacity(&s.a, 40000))
	contextTLSNearMinimum(t, rt)
	beforeDegree := s.degreeStored
	cause := m.finishComposition(nil)
	if !errors.Is(cause, arena.ErrBounds) || !s.stopped || s.sourceCause == nil {
		t.Fatal("real postphysical lexical stop", cause)
	}
	if !s.degreeCompleted || s.degreeStored != beforeDegree {
		t.Fatal("older scoped degree destructor ran after auxiliary stop")
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, nil, 0)
	state := rt.State()
	first := s.sourceCause
	if e = m.finishComposition(nil); !errors.Is(e, first) || rt.State() != state || !s.degreeCompleted {
		t.Fatal("source stopped frontier replayed", e)
	}
	contextTLSFinish(t, rt, m.CleanupComposition)
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, nil, 0)
	if s.sourceCause != first {
		t.Fatal("owning abort erased source history")
	}
}

func TestPolynomialCompositionEarlyRoutesPreserveOlderContext(t *testing.T) {
	m, p, control := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{3, 0, 0}}, {2, [3]uint32{}}})
	q := polynomialCompositionTerms(t, m, []polynomialEvalTerm{{1, [3]uint32{1, 0, 0}}, {1, [3]uint32{0, 1, 0}}})
	r, e := NewPolynomialRef(m.core)
	serviceMust(t, e)
	defer func() { serviceMust(t, m.CleanupComposition()); serviceMust(t, r.Close()) }()
	control.checkpoints, control.limit = 0, 0
	if e = m.Compose(p, q, r); e != polynomialCancellationError || !m.composition.active {
		t.Fatal("actual older source body cancellation", e)
	}
	old := m.composition.bodyCause
	serviceMust(t, m.Compose(m.core.unit, Polynomial{}, r)) // q is actually unread.
	if r.root.value != m.core.unit || m.composition.bodyCause != old || !m.composition.active {
		t.Fatal("constant early route read/replaced old context")
	}
	serviceMust(t, m.ComposeXMinusY(p, 0, r))
	a, e := m.core.coefficientCore(r.root.value, 0)
	serviceMust(t, e)
	if a.NativeScalar() != 2 || m.composition.bodyCause != old {
		t.Fatal("same-var actual independent output/context", a)
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, q, r.root.value, r.root.incoming}, nil, r.header)
}

func TestPolynomialCompositionScalarForeignReplacement(t *testing.T) {
	m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{1, 0, 0}}})
	a := unumeral.Int{}
	digits := make([]uint32, 80)
	digits[0], digits[79] = 5, 1
	serviceMust(t, m.core.integer.SetLimbs(&a, digits))
	defer func() {
		m.core.integer.Budget().FailAfter(-1)
		serviceMust(t, m.CleanupComposition())
		serviceMust(t, m.core.cleanupConstantScope())
		serviceMust(t, m.core.integer.Del(&a))
	}()
	serviceMust(t, m.beginComposition())
	m.core.integer.Budget().FailAfter(0)
	_, cause := m.core.MkConstantTakeCopy(&a)
	m.composition.executing = false
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, []*unumeral.Int{&a}, 0)
	if !numericPhysicalReservationRefusal(cause) || !m.core.constantScope.active || m.composition.constantGeneration == 0 {
		t.Fatal("actual old scalar ctor refusal", cause)
	}
	old := m.composition.constantGeneration
	m.core.integer.Budget().FailAfter(-1)
	serviceMust(t, m.core.cleanupConstantScope())
	m.core.integer.Budget().FailAfter(0)
	_, cause = m.core.MkConstantTakeCopy(&a)
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, []*unumeral.Int{&a}, 0)
	if !numericPhysicalReservationRefusal(cause) || !m.core.constantScope.active || m.core.constantGeneration == old {
		t.Fatal("real later scalar ctor", cause)
	}
	later := m.core.constantScope
	generation := m.core.constantGeneration
	m.core.integer.Budget().FailAfter(-1)
	serviceMust(t, m.CleanupComposition())
	if !testNumericScopeEqual(m.core.constantScope, later) || m.core.constantGeneration != generation {
		t.Fatal("old receipt swept foreign scalar group")
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, []*unumeral.Int{&a}, 0)
}

func TestPolynomialCompositionPostphysicalCellRawReceipt(t *testing.T) {
	b := arena.NewBudget(1 << 26)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true})
	serviceMust(t, e)
	cfg := serviceConfig(b).Integers
	cfg.Memory = rt
	cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
	serviceMust(t, e)
	t.Cleanup(func() {
		serviceMust(t, q.CloseChecked())
		serviceMust(t, rt.Close())
		if b.Used() != 0 {
			t.Fatal("actual full refund", b.Used())
		}
	})
	m, p, _ := polynomialEvalFixtureWithRationals(t, q, []polynomialEvalTerm{{1, [3]uint32{}}})
	unit, e := m.core.monomials.Unit()
	serviceMust(t, e)
	as := make([]unumeral.Int, 4000)
	ms := make([]Monomial, 4000)
	for i := range ms {
		ms[i] = unit
		serviceMust(t, m.core.monomials.IncRef(unit))
	}
	defer func() {
		serviceMust(t, rt.SetMaxSize(0))
		serviceMust(t, m.CleanupComposition())
		for range ms {
			serviceMust(t, m.core.monomials.DecRef(unit))
		}
	}()
	serviceMust(t, m.beginComposition())
	serviceMust(t, rt.SetMaxSize(1))
	_, cause := m.core.MkCoreTake(as, ms)
	m.composition.executing = false
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, nil, 0)
	roots, e := m.core.monomials.allocator.PendingRoots()
	serviceMust(t, e)
	if !numericPhysicalReservationRefusal(cause) || roots == ([3]umemory.Block{}) || m.core.pending.raw != (umemory.ObjectRef{}) {
		t.Fatal("real shared allocator postphysical prefix", cause, roots, m.core.pending)
	}
	serviceMust(t, rt.SetMaxSize(0))
	serviceMust(t, m.CleanupComposition())
	after, e := m.core.monomials.allocator.PendingRoots()
	serviceMust(t, e)
	if after != ([3]umemory.Block{}) {
		t.Fatal("operation abort omitted shared physical cell raw", after)
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, nil, 0)
}

var polynomialCompositionCallbackState struct {
	target  Polynomial
	runtime *umemory.Runtime
	inputs  [4000]Monomial
	invoked bool
}

func polynomialCompositionForeignDeletionCall(h *PolynomialDeletionHandler, c *MultivariatePolynomialCore, p Polynomial) error {
	if p != polynomialCompositionCallbackState.target || polynomialCompositionCallbackState.invoked {
		return nil
	}
	polynomialCompositionCallbackState.invoked = true
	unit, e := c.monomials.Unit()
	if e != nil {
		return e
	}
	for i := range polynomialCompositionCallbackState.inputs {
		polynomialCompositionCallbackState.inputs[i] = unit
		if e = c.monomials.IncRef(unit); e != nil {
			return e
		}
	}
	if e = polynomialCompositionCallbackState.runtime.SetMaxSize(1); e != nil {
		return e
	}
	as := [4000]unumeral.Int{}
	_, e = c.MkCoreTake(as[:], polynomialCompositionCallbackState.inputs[:])
	return e
}
func TestPolynomialCompositionDeletionCallbackOwnsForeignContinuation(t *testing.T) {
	b := arena.NewBudget(1 << 26)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true})
	serviceMust(t, e)
	cfg := serviceConfig(b).Integers
	cfg.Memory = rt
	cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
	serviceMust(t, e)
	t.Cleanup(func() {
		serviceMust(t, q.CloseChecked())
		serviceMust(t, rt.Close())
		if b.Used() != 0 {
			t.Fatal("whole callback graph refund", b.Used())
		}
	})
	m, p, _ := polynomialEvalFixtureWithRationals(t, q, []polynomialEvalTerm{{1, [3]uint32{}}})
	r, e := NewPolynomialRef(m.core)
	serviceMust(t, e)
	old, e := m.core.MkVariable(0, 9)
	serviceMust(t, e)
	serviceMust(t, r.Assign(old))
	h, e := NewPolynomialDeletionHandler(b, polynomialCompositionForeignDeletionCall)
	serviceMust(t, e)
	serviceMust(t, m.core.AddDeletionHandler(h))
	defer func() {
		serviceMust(t, rt.SetMaxSize(0))
		serviceMust(t, m.CleanupComposition())
		serviceMust(t, m.core.CleanupConstruction())
		serviceMust(t, m.core.monomials.allocator.CleanupPending())
		for _, a := range polynomialCompositionCallbackState.inputs {
			if a != (Monomial{}) {
				serviceMust(t, m.core.monomials.DecRef(a))
			}
		}
		serviceMust(t, m.core.RemoveDeletionHandler(h))
		serviceMust(t, h.CloseChecked())
		serviceMust(t, r.Close())
	}()
	polynomialCompositionCallbackState.target = old
	polynomialCompositionCallbackState.runtime = rt
	polynomialCompositionCallbackState.invoked = false
	polynomialCompositionCallbackState.inputs = [4000]Monomial{}
	serviceMust(t, m.beginComposition())
	cause := m.compositionAssign(r, m.core.unit)
	cause = m.finishComposition(cause)
	roots, e := m.core.monomials.allocator.PendingRoots()
	serviceMust(t, e)
	if !polynomialCompositionCallbackState.invoked || !numericPhysicalReservationRefusal(cause) || roots == ([3]umemory.Block{}) {
		t.Fatal("real callback constructor physical stop", cause, roots)
	}
	if m.core.pending.compositionParent != nil || m.composition.monomialRaw != ([3]umemory.Block{}) {
		t.Fatal("callback continuation attributed to composition")
	}
	pending := m.core.pending
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, r.root.value, r.root.incoming}, nil, r.header+h.header)
	serviceMust(t, rt.SetMaxSize(0))
	serviceMust(t, m.CleanupComposition())
	after, e := m.core.monomials.allocator.PendingRoots()
	serviceMust(t, e)
	if after != roots || m.core.pending != pending {
		t.Fatal("owning composition abort consumed foreign callback state")
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, r.root.value, r.root.incoming}, nil, r.header+h.header)
}

func TestPolynomialCompositionUnsignedDegreeConstructorStop(t *testing.T) {
	m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{1, 0, 0}}})
	term, e := m.core.monomials.MkVariable(0, ^uint32(0))
	serviceMust(t, e)
	serviceMust(t, m.core.monomials.IncRef(term))
	large, e := m.core.MkCoreTake([]unumeral.Int{unumeral.NewInt(1)}, []Monomial{term})
	serviceMust(t, e)
	serviceMust(t, m.core.IncRef(large))
	r, e := NewPolynomialRef(m.core)
	serviceMust(t, e)
	serviceMust(t, r.Assign(m.core.unit))
	defer func() {
		serviceMust(t, m.CleanupComposition())
		serviceMust(t, r.Close())
		serviceMust(t, m.core.DecRef(large))
	}()
	before := m.core.memory.State()
	cause := m.Compose(large, m.core.unit, r)
	expected := error(arena.ErrBounds)
	if polynomialSourceDebug {
		expected = ErrPolynomialAssertion
	}
	if !errors.Is(cause, expected) || m.composition.degreeCompleted || m.composition.aCompleted || m.composition.degreeStored != 0 || r.root.value != m.core.unit {
		t.Fatal("unsigned n+1 wrap at reached indexed access", cause, m.composition.degreeCompleted)
	}
	if m.core.memory.State() != before || m.degree2pos != nil {
		t.Fatal("wrapped zero reserve manufactured an allocation")
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p, large, r.root.value}, nil, r.header)
	serviceMust(t, m.CleanupComposition())
	if m.composition.degreeCompleted || m.composition.aCompleted || m.degree2pos != nil {
		t.Fatal("owning abort fabricated failed constructor completion")
	}
}

// A real destructive Copy refusal records the dangling source pointer. This
// proves the owner-primitive destructor boundary, not a composed arithmetic call.
func TestPolynomialCompositionStoppedConstantPrecedesOuterLocals(t *testing.T) {
	for _, rational := range []bool{false, true} {
		t.Run(fmt.Sprint(rational), func(t *testing.T) {
			m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{2, 0, 0}}})
			serviceMust(t, m.beginComposition())
			serviceMust(t, m.saveCompositionDegrees(p))
			s := &m.composition
			s.degreeInput = p
			s.degreeCompleted = true
			s.aCompleted = true
			s.a = unumeral.NewInt(7)
			serviceMust(t, m.core.beginConstantScope())
			defer func() { serviceMust(t, m.CleanupComposition()) }()
			c := &m.core.constantScope
			z := m.core.integer
			large := unumeral.Int{}
			limbs := make([]uint32, 80)
			for i := range limbs {
				limbs[i] = 1
			}
			serviceMust(t, z.SetLimbs(&large, limbs))
			defer func() { serviceMust(t, z.Del(&large)) }()
			target := &c.numeral
			if rational {
				c.rationalCompleted = true
				target = &(*c.rational.BorrowNumerator())
				(*c.rational.BorrowDenominator()) = unumeral.NewInt(13)
			} else {
				c.numeralCompleted = true
			}
			serviceMust(t, z.Copy(target, &z.Two64))
			z.Budget().FailAfter(0)
			body := z.Copy(target, &large)
			if !numericPhysicalReservationRefusal(body) || (target).Cell != z.FailedRef {
				t.Fatal("actual destructive Copy prefix", body, (target).Cell)
			}
			polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, []*unumeral.Int{&large}, 0)
			z.Budget().FailAfter(-1)
			first := m.core.finishConstantScope(body)
			if !errors.Is(first, arena.ErrInvalidSlot) {
				t.Fatal("source dangling guard", first)
			}
			polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, []*unumeral.Int{&large}, 0)
			firstSource := c.sourceCause
			if c.cause != body {
				t.Fatal("original Copy body cause overwritten")
			}
			state := m.core.memory.State()
			if e := m.core.finishConstantScope(nil); !errors.Is(e, arena.ErrInvalidSlot) || m.core.memory.State() != state {
				t.Fatal("stopped younger source destructor replayed", e)
			}
			if e := m.finishComposition(first); !errors.Is(e, arena.ErrInvalidSlot) || !s.aCompleted || !s.degreeCompleted {
				t.Fatal("older destructors ran after younger stop", e)
			}
			if c.cause != body || c.sourceCause != firstSource {
				t.Fatal("callee source/body cause changed")
			}
			outerBody, outerSource := s.bodyCause, s.sourceCause
			if rational && (*c.rational.BorrowDenominator()).NativeScalar() != 13 {
				t.Fatal("denominator reached after numerator stop")
			}
			serviceMust(t, m.CleanupComposition())
			if s.bodyCause != outerBody || s.sourceCause != outerSource || c.cause != body || c.sourceCause != firstSource {
				t.Fatal("owning cleanup erased source history")
			}
			polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, []*unumeral.Int{&large}, 0)
		})
	}
}

// Genuine public TLS signed-counter guard; owner primitive, not ordinary OOM.
func TestPolynomialCompositionStoppedConstantPhysicalReleasePrecedesOuter(t *testing.T) {
	budget := arena.NewBudget(1 << 25)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, SizeTBits: 64, ThreadLocal: true})
	serviceMust(t, e)
	cfg := serviceConfig(budget).Integers
	cfg.Memory = rt
	cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
	serviceMust(t, e)
	t.Cleanup(func() {
		contextTLSFinish(t, rt, q.CloseChecked)
		serviceMust(t, rt.Close())
		if budget.Used() != 0 {
			t.Fatal("whole actual graph refund", budget.Used())
		}
	})
	m, p, _ := polynomialEvalFixtureWithRationals(t, q, []polynomialEvalTerm{{1, [3]uint32{3, 0, 0}}, {2, [3]uint32{}}})
	defer func() {
		if !m.closed {
			contextTLSFinish(t, rt, m.CleanupComposition)
			contextTLSFinish(t, rt, func() error { return m.core.CleanupValue(&p) })
			contextTLSFinish(t, rt, m.CloseChecked)
		}
	}()
	// Source successful buffers must be warm before signed-domain free. These
	// real source calls isolate the lexical numeral free from cold freelist growth.
	warm, e := m.core.MkVariable(1, 1)
	serviceMust(t, e)
	serviceMust(t, m.core.IncRef(warm))
	serviceMust(t, m.core.DecRef(warm))
	serviceMust(t, m.beginComposition())
	s := &m.composition
	serviceMust(t, m.saveCompositionDegrees(p))
	s.degreeInput = p
	s.degreeCompleted = true
	s.aCompleted = true
	s.a = unumeral.NewInt(7)
	serviceMust(t, m.core.beginConstantScope())
	c := &m.core.constantScope
	c.numeralCompleted = true
	serviceMust(t, q.Integers().SetLimbs(&c.numeral, []uint32{1, 1}))
	serviceMust(t, q.Integers().EnsureOwnedCapacity(&c.numeral, 40000))
	contextTLSNearMinimum(t, rt)
	beforeDegree := s.degreeStored
	cause := m.core.finishConstantScope(nil)
	if !errors.Is(cause, arena.ErrBounds) || !c.stopped || !c.numeralCompleted {
		t.Fatal("actual younger physical release stop", cause)
	}
	stateBefore := rt.State()
	firstC := c.sourceCause
	if e = m.core.finishConstantScope(nil); !errors.Is(e, firstC) || rt.State() != stateBefore {
		t.Fatal("younger consumed free replay", e)
	}
	cause = m.finishComposition(cause)
	if !errors.Is(cause, arena.ErrBounds) || !s.stopped || s.sourceCause == nil {
		t.Fatal("real postphysical lexical stop", cause)
	}
	if !s.aCompleted || !s.degreeCompleted || s.degreeStored != beforeDegree {
		t.Fatal("older scoped degree destructor ran after auxiliary stop")
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, nil, 0)
	state := rt.State()
	first := s.sourceCause
	if e = m.finishComposition(nil); !errors.Is(e, first) || rt.State() != state || !s.degreeCompleted {
		t.Fatal("source stopped frontier replayed", e)
	}
	contextTLSFinish(t, rt, m.CleanupComposition)
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, nil, 0)
	if s.sourceCause != first {
		t.Fatal("owning abort erased source history")
	}
}

func TestPolynomialCompositionForeignStoppedConstantDoesNotPoisonOuter(t *testing.T) {
	m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{2, 0, 0}}})
	c := &m.core.constantScope
	z := m.core.integer
	serviceMust(t, m.core.beginConstantScope()) // No executing composition receives this invocation.
	c.numeralCompleted = true
	large := unumeral.Int{}
	limbs := make([]uint32, 80)
	for i := range limbs {
		limbs[i] = 1
	}
	serviceMust(t, z.SetLimbs(&large, limbs))
	defer func() { serviceMust(t, z.Del(&large)) }()
	serviceMust(t, z.Copy(&c.numeral, &z.Two64))
	z.Budget().FailAfter(0)
	body := z.Copy(&c.numeral, &large)
	if !numericPhysicalReservationRefusal(body) || (c.numeral).Cell != z.FailedRef {
		t.Fatal("actual foreign destructive prefix", body)
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, []*unumeral.Int{&large}, 0)
	z.Budget().FailAfter(-1)
	if e := m.core.finishConstantScope(body); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal("foreign source stop", e)
	}
	defer func() { serviceMust(t, m.core.cleanupConstantScope()) }()
	serviceMust(t, m.beginComposition())
	s := &m.composition
	s.a = unumeral.NewInt(7)
	s.aCompleted = true
	serviceMust(t, m.finishComposition(nil))
	if s.stopped || s.aCompleted || !c.stopped || !c.active || c.cause != body {
		t.Fatal("foreign stopped generation attributed to outer")
	}
	serviceMust(t, m.CleanupComposition())
	if !c.active || !c.numeralCompleted {
		t.Fatal("outer owning cleanup touched foreign callee")
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, []*unumeral.Int{&large}, 0)
}

// Actual returned-placement and public TLS leaf stop; helper-domain proof.
func TestPolynomialCompositionStoppedConstantResultHasSingleOwningCleanup(t *testing.T) {
	budget := arena.NewBudget(1 << 25)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, SizeTBits: 64, ThreadLocal: true})
	serviceMust(t, e)
	cfg := serviceConfig(budget).Integers
	cfg.Memory = rt
	cfg.Profile = umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
	serviceMust(t, e)
	t.Cleanup(func() {
		contextTLSFinish(t, rt, q.CloseChecked)
		serviceMust(t, rt.Close())
		if budget.Used() != 0 {
			t.Fatal("whole actual graph refund", budget.Used())
		}
	})
	m, p, _ := polynomialEvalFixtureWithRationals(t, q, []polynomialEvalTerm{{1, [3]uint32{3, 0, 0}}, {2, [3]uint32{}}})
	defer func() {
		if !m.closed {
			contextTLSFinish(t, rt, m.CleanupComposition)
			contextTLSFinish(t, rt, func() error { return m.core.CleanupValue(&p) })
			contextTLSFinish(t, rt, m.CloseChecked)
		}
	}()
	// Source successful buffers must be warm before signed-domain free. These
	// real source calls isolate the lexical numeral free from cold freelist growth.
	warm, e := m.core.MkVariable(1, 1)
	serviceMust(t, e)
	serviceMust(t, m.core.IncRef(warm))
	serviceMust(t, m.core.DecRef(warm))
	serviceMust(t, m.beginComposition())
	s := &m.composition
	serviceMust(t, m.saveCompositionDegrees(p))
	s.degreeInput = p
	s.degreeCompleted = true
	s.aCompleted = true
	s.a = unumeral.NewInt(7)
	serviceMust(t, m.core.beginConstantScope())
	c := &m.core.constantScope
	c.rationalCompleted = true
	serviceMust(t, q.Integers().SetLimbs(&(*c.rational.BorrowNumerator()), []uint32{1, 1}))
	serviceMust(t, q.Integers().EnsureOwnedCapacity(&(*c.rational.BorrowNumerator()), 40000))
	value := unumeral.NewInt(3)
	result, e := m.core.MkConstantTake(&value)
	serviceMust(t, e)
	c.result = result // Exact constantFromRationalScope returned-result assignment.
	if s.freshResult != result {
		t.Fatal("actual placement result receipt absent")
	}
	contextTLSNearMinimum(t, rt)
	beforeDegree := s.degreeStored
	cause := m.core.finishConstantScope(nil)
	if !errors.Is(cause, arena.ErrBounds) || !c.stopped || !c.rationalCompleted {
		t.Fatal("actual younger physical release stop", cause)
	}
	stateBefore := rt.State()
	firstC := c.sourceCause
	if e = m.core.finishConstantScope(nil); !errors.Is(e, firstC) || rt.State() != stateBefore {
		t.Fatal("younger consumed free replay", e)
	}
	cause = m.finishComposition(cause)
	if !errors.Is(cause, arena.ErrBounds) || !s.stopped || s.sourceCause == nil {
		t.Fatal("real postphysical lexical stop", cause)
	}
	if !s.aCompleted || !s.degreeCompleted || s.degreeStored != beforeDegree {
		t.Fatal("older scoped degree destructor ran after auxiliary stop")
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, nil, 0)
	state := rt.State()
	first := s.sourceCause
	if e = m.finishComposition(nil); !errors.Is(e, first) || rt.State() != state || !s.degreeCompleted {
		t.Fatal("source stopped frontier replayed", e)
	}
	contextTLSFinish(t, rt, m.CleanupComposition)
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, nil, 0)
	if s.sourceCause != first {
		t.Fatal("owning abort erased source history")
	}
}

package polynomial

// Source: src/math/polynomial/polynomial.cpp:7439-7476.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestPolynomialGlobalConvertSelfBorrow(t *testing.T) {
	m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{3, [3]uint32{2, 0, 0}}, {-1, [3]uint32{0, 0, 0}}})
	before := m.core.integer.Budget().Used()
	out, e := ConvertPolynomial(m, p, m, ^uint32(0), 0)
	serviceMust(t, e)
	if out != p || m.core.integer.Budget().Used() != before {
		t.Fatal("same manager returns borrowed source", out, p)
	}
	serviceMust(t, m.CleanupConversion())
}
func TestPolynomialGlobalConvertDistinctOwnerFilter(t *testing.T) {
	sm, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{3, [3]uint32{2, 0, 0}}, {-2, [3]uint32{1, 0, 0}}, {5, [3]uint32{0, 0, 0}}})
	tm, _, _ := polynomialEvalFixture(t, nil)
	before := sm.core.integer.Budget().Used()
	out, e := ConvertPolynomial(sm, p, tm, 0, 1)
	serviceMust(t, e)
	serviceMust(t, tm.core.IncRef(out))
	defer func() { serviceMust(t, tm.core.DecRef(out)) }()
	if sm.core.integer.Budget().Used() != before {
		t.Fatal("source owner modified")
	}
	for x := int32(-3); x <= 3; x++ {
		in, v := unumeral.NewInt(x), unumeral.Int{}
		serviceMust(t, tm.UnivariateEvaluate(out, 0, &in, &v))
		got, e := tm.core.integer.Int64(&v)
		serviceMust(t, e)
		if got != 5-2*int64(x) {
			t.Fatal("independent filtered value", x, got)
		}
		serviceMust(t, tm.core.integer.Del(&v))
	}
	serviceMust(t, tm.CleanupConversion())
}

func TestPolynomialGlobalConvertEveryActualPrefix(t *testing.T) {
	success := false
	for ordinal := int64(0); ordinal < 256; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			sm, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{3, [3]uint32{2, 0, 0}}, {-2, [3]uint32{1, 0, 0}}, {5, [3]uint32{0, 0, 0}}})
			tm, _, _ := polynomialEvalFixture(t, nil)
			before := sm.core.integer.Budget().Used()
			tm.core.integer.Budget().FailAfter(ordinal)
			out, e := ConvertPolynomial(sm, p, tm, ^uint32(0), 0)
			if e != nil && !numericPhysicalReservationRefusal(e) {
				t.Fatal("actual source refusal", ordinal, e)
			}
			polynomialImpPhysicalAuditNamed(t, tm, []Polynomial{out}, nil, 0)
			polynomialImpPhysicalAuditValues(t, sm, []Polynomial{p})
			if sm.core.integer.Budget().Used() != before {
				t.Fatal("source was mutated")
			}
			tm.core.integer.Budget().FailAfter(-1)
			serviceMust(t, tm.CleanupConversion())
			if e == nil {
				success = true
				serviceMust(t, tm.core.IncRef(out))
				in, value := unumeral.NewInt(2), unumeral.Int{}
				serviceMust(t, tm.UnivariateEvaluate(out, 0, &in, &value))
				got, e := tm.core.integer.Int64(&value)
				serviceMust(t, e)
				if got != 13 {
					t.Fatal("independent unfiltered value", got)
				}
				serviceMust(t, tm.core.integer.Del(&value))
				serviceMust(t, tm.core.DecRef(out))
			}
		})
		if success {
			t.Log("first success", ordinal)
			break
		}
	}
	if !success {
		t.Fatal("actual prefix horizon exhausted")
	}
}

func TestPolynomialGlobalConvertOverflowingBuffersEveryPrefix(t *testing.T) {
	success := false
	for ordinal := int64(0); ordinal < 1024; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			sm, _, _ := polynomialEvalFixture(t, nil)
			tm, _, _ := polynomialEvalFixture(t, nil)
			values := make([]unumeral.Int, 130)
			for i := range values {
				values[i] = unumeral.NewInt(1)
			}
			p, e := sm.ToPolynomial(130, values, 0)
			serviceMust(t, e)
			serviceMust(t, sm.core.IncRef(p))
			defer func() { serviceMust(t, sm.core.DecRef(p)) }()
			tm.core.integer.Budget().FailAfter(ordinal)
			out, e := ConvertPolynomial(sm, p, tm, ^uint32(0), 0)
			if e != nil && !numericPhysicalReservationRefusal(e) {
				t.Fatal("actual overflow source prefix", ordinal, e)
			}
			polynomialImpPhysicalAuditValues(t, tm, []Polynomial{out})
			polynomialImpPhysicalAuditValues(t, sm, []Polynomial{p})
			tm.core.integer.Budget().FailAfter(-1)
			serviceMust(t, tm.CleanupConversion())
			if e == nil {
				success = true
				serviceMust(t, tm.core.IncRef(out))
				in, value := unumeral.NewInt(1), unumeral.Int{}
				serviceMust(t, tm.UnivariateEvaluate(out, 0, &in, &value))
				got, e := tm.core.integer.Int64(&value)
				serviceMust(t, e)
				if got != 130 {
					t.Fatal("independent sum130", got)
				}
				serviceMust(t, tm.core.integer.Del(&value))
				serviceMust(t, tm.core.DecRef(out))
			}
		})
		if success {
			t.Log("first overflow success", ordinal)
			break
		}
	}
	if !success {
		t.Fatal("actual overflow horizon exhausted")
	}
}

func TestPolynomialGlobalConvertBigCoefficientDistinctInteger(t *testing.T) {
	sm, _, _ := polynomialEvalFixture(t, nil)
	tm, _, _ := polynomialEvalFixture(t, nil)
	input := []unumeral.Int{sm.core.integer.Two64, unumeral.NewInt(1)}
	p, e := sm.ToPolynomial(2, input, 0)
	serviceMust(t, e)
	serviceMust(t, sm.core.IncRef(p))
	defer func() { serviceMust(t, sm.core.DecRef(p)) }()
	out, e := ConvertPolynomial(sm, p, tm, ^uint32(0), 0)
	serviceMust(t, e)
	serviceMust(t, tm.core.IncRef(out))
	defer func() { serviceMust(t, tm.core.DecRef(out)) }()
	polynomialImpPhysicalAuditValues(t, sm, []Polynomial{p})
	polynomialImpPhysicalAuditValues(t, tm, []Polynomial{out})
	in, value := unumeral.NewInt(1), unumeral.Int{}
	serviceMust(t, tm.UnivariateEvaluate(out, 0, &in, &value))
	text, e := tm.core.integer.String(&value)
	serviceMust(t, e)
	if text != "18446744073709551617" {
		t.Fatal("distinct actual BIG owner", text)
	}
	serviceMust(t, tm.core.integer.Del(&value))
	serviceMust(t, tm.CleanupConversion())
}

func TestPolynomialGlobalConvertExactNumericForeignReplacement(t *testing.T) {
	sm, _, _ := polynomialEvalFixture(t, nil)
	a := unumeral.Int{}
	serviceMust(t, sm.core.integer.Copy(&a, &sm.core.integer.Two64))
	p, e := sm.core.MkConstantTake(&a)
	serviceMust(t, e)
	serviceMust(t, sm.core.IncRef(p))
	defer func() { serviceMust(t, sm.core.DecRef(p)) }()
	budget := arena.NewBudget(1 << 24)
	z, e := unumeral.NewIntegerManager(unumeral.IntegerConfig{Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{PageSize: 8, SlotLimit: 8, Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}})
	serviceMust(t, e)
	tm, e := NewMultivariatePolynomialManager(MultivariatePolynomialManagerConfig{Core: MultivariatePolynomialConfig{Integer: z, Checkpoint: func() bool { panic("source convert has no checkpoint") }, Cooperation: func() error { panic("source convert has no cooperation") }}})
	serviceMust(t, e)
	foreign := unumeral.Int{}
	defer func() {
		serviceMust(t, tm.CleanupConversion())
		serviceMust(t, z.CleanupPendingAllocation())
		serviceMust(t, z.Del(&foreign))
		serviceMust(t, tm.CloseChecked())
		serviceMust(t, z.CloseChecked())
		if budget.Used() != 0 {
			t.Fatal("whole target graph", budget.Used())
		}
	}()
	_, cause := ConvertPolynomial(sm, p, tm, ^uint32(0), 0)
	if !errors.Is(cause, arena.ErrSlotLimit) || z.PendingAllocationObject() == (umemory.ObjectRef{}) {
		t.Fatal("actual coefficient payload before slot refusal", cause)
	}
	original := z.PendingCell
	polynomialImpPhysicalAuditNamed(t, tm, nil, []*unumeral.Int{&foreign}, 0)
	serviceMust(t, z.CleanupPendingAllocation())
	e = z.Copy(&foreign, &z.Two64)
	if !errors.Is(e, arena.ErrSlotLimit) || z.PendingAllocationObject() == original.Object {
		t.Fatal("actual later generation", e)
	}
	replacement := z.PendingCell
	polynomialImpPhysicalAuditNamed(t, tm, nil, []*unumeral.Int{&foreign}, 0)
	serviceMust(t, tm.CleanupConversion())
	if z.PendingCell != replacement {
		t.Fatal("scope consumed foreign pending backing")
	}
	polynomialImpPhysicalAuditNamed(t, tm, nil, []*unumeral.Int{&foreign}, 0)
}

func TestPolynomialGlobalConvertReturnedResultNativeMonomials(t *testing.T) {
	m, _, _ := polynomialEvalFixture(t, nil)
	serviceMust(t, m.beginConversion())
	s := &m.conversion
	t.Cleanup(func() { m.core.integer.Budget().FailAfter(-1); serviceMust(t, m.CleanupConversion()) })
	unit, e := m.core.monomials.Unit()
	serviceMust(t, e)
	before, used := m.core.memory.Stats(), m.core.integer.Budget().Used()
	m.core.integer.Budget().FailAfter(0)
	zero := unumeral.Int{}
	for i := 0; i < 4097; i++ {
		s.ms = append(s.ms, unit)
		serviceMust(t, s.as.Push(&zero))
	}
	if m.core.memory.Stats() != before || m.core.integer.Budget().Used() != used {
		t.Fatal("native lexical storage charged Runtime")
	}
	m.core.integer.Budget().FailAfter(-1)
	one := unumeral.NewInt(1)
	serviceMust(t, s.as.Set(0, &one))
	av, e := s.as.View()
	serviceMust(t, e)
	serviceMust(t, m.core.som.reset())
	p, e := m.core.mkTermsSource(s.as.Count(), polynomialUnivariateCoefficients{array: av, indexed: true}, monomialArrayView{values: s.ms, count: uint32(len(s.ms))})
	serviceMust(t, e)
	if !s.ownsResult || s.result != p {
		t.Fatal("actual source return expression ownership")
	}
	polynomialImpPhysicalAudit(t, m)
	serviceMust(t, m.finishConversion(nil))
	if s.as.Count() != 0 || s.ms != nil || !s.ownsResult {
		t.Fatal("source lexical destruction lost returned root")
	}
	polynomialImpPhysicalAudit(t, m)
	serviceMust(t, m.CleanupConversion())
	if s.result != (Polynomial{}) || s.active || s.orphans != nil {
		t.Fatal("owning completion retained native scope")
	}
	polynomialImpPhysicalAudit(t, m)
}

func TestPolynomialGlobalConvertRingZeroNewVersusBorrowedMonomial(t *testing.T) {
	for _, borrowed := range []bool{false, true} {
		t.Run(fmt.Sprint(borrowed), func(t *testing.T) {
			sm, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{7, [3]uint32{2, 0, 0}}})
			tm, _, _ := polynomialEvalFixture(t, nil)
			serviceMust(t, tm.core.SetZPUint64(7))
			var prior Monomial
			if borrowed {
				var e error
				prior, e = tm.core.monomials.MkVariable(0, 2)
				serviceMust(t, e)
			}
			out, e := ConvertPolynomial(sm, p, tm, ^uint32(0), 0)
			serviceMust(t, e)
			n, e := tm.core.readWord(out, 8)
			serviceMust(t, e)
			if n != 0 {
				t.Fatal("actual target SET zero size", n)
			}
			serviceMust(t, tm.core.IncRef(out))
			defer func() { serviceMust(t, tm.core.DecRef(out)) }()
			if tm.conversionHasOrphans() == borrowed {
				t.Fatal("created outcome authority distinct from borrowed RC0", borrowed)
			}
			polynomialImpPhysicalAuditValues(t, tm, []Polynomial{out})
			serviceMust(t, tm.CleanupConversion())
			if borrowed {
				if _, e := tm.core.monomials.location(prior); e != nil {
					t.Fatal("own abort swept borrowed existing zero-reference result", e)
				}
				serviceMust(t, tm.core.monomials.CleanupValue(&prior))
			}
			polynomialImpPhysicalAudit(t, tm)
		})
	}
}

func TestPolynomialGlobalConvertSharedMonomialManager(t *testing.T) {
	sm, p, control := polynomialEvalFixture(t, []polynomialEvalTerm{{4, [3]uint32{3, 0, 0}}, {-3, [3]uint32{0, 0, 0}}})
	tm, e := NewMultivariatePolynomialManager(MultivariatePolynomialManagerConfig{Core: MultivariatePolynomialConfig{Integer: sm.core.integer, Rationals: sm.core.config.Rationals, Monomials: sm.core.monomials, Checkpoint: control.checkpoint, Cooperation: control.cooperate}})
	serviceMust(t, e)
	defer func() { serviceMust(t, tm.CleanupConversion()); serviceMust(t, tm.CloseChecked()) }()
	vars, e := sm.core.monomials.NumVars()
	serviceMust(t, e)
	out, e := ConvertPolynomial(sm, p, tm, ^uint32(0), 0)
	serviceMust(t, e)
	serviceMust(t, tm.core.IncRef(out))
	defer func() { serviceMust(t, tm.core.DecRef(out)) }()
	after, e := sm.core.monomials.NumVars()
	serviceMust(t, e)
	if after != vars || cap(tm.conversion.orphans) != 0 || tm.conversion.calleeGeneration != 0 {
		t.Fatal("shared monomial path must not invoke conversion factory")
	}
	for x := int32(-2); x <= 2; x++ {
		in, v := unumeral.NewInt(x), unumeral.Int{}
		serviceMust(t, tm.UnivariateEvaluate(out, 0, &in, &v))
		got, e := tm.core.integer.Int64(&v)
		serviceMust(t, e)
		xx := int64(x)
		if got != 4*xx*xx*xx-3 {
			t.Fatal("independent shared-MM value", x, got)
		}
		serviceMust(t, tm.core.integer.Del(&v))
	}
}

func TestPolynomialGlobalConvertFactoryForeignReplacement(t *testing.T) {
	found := false
	for ordinal := int64(0); ordinal < 32; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			sm, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{3, [3]uint32{2, 0, 0}}, {5, [3]uint32{0, 0, 0}}})
			tm, _, _ := polynomialEvalFixture(t, nil)
			defer func() { tm.core.integer.Budget().FailAfter(-1); serviceMust(t, tm.CleanupConversion()) }()
			tm.core.integer.Budget().FailAfter(ordinal)
			out, e := ConvertPolynomial(sm, p, tm, ^uint32(0), 0)
			polynomialImpPhysicalAuditValues(t, tm, []Polynomial{out})
			tm.core.integer.Budget().FailAfter(-1)
			if e == nil {
				serviceMust(t, tm.core.IncRef(out))
				serviceMust(t, tm.core.DecRef(out))
				return
			}
			s := &tm.conversion
			if s.factoryGeneration == 0 || tm.core.pending.generation != s.factoryGeneration {
				return
			}
			found = true
			old := s.factoryGeneration
			// Explicit caller recovery of the original concrete callee precedes a
			// separately invoked large placement constructor, not implicit scope repair.
			serviceMust(t, tm.core.cleanupPendingCreation())
			coefficients := make([]unumeral.Int, 1024)
			monomials := make([]Monomial, 1024)
			unit, e := tm.core.monomials.Unit()
			serviceMust(t, e)
			for i := range coefficients {
				coefficients[i] = unumeral.NewInt(1)
				monomials[i] = unit
			}
			tm.core.integer.Budget().FailAfter(0)
			_, cause := tm.core.mkCell(1024, coefficients, monomials, nil, nil)
			if !numericPhysicalReservationRefusal(cause) || tm.core.pending.generation == 0 || tm.core.pending.generation == old || tm.core.pending.conversionParent != nil {
				t.Fatal("actual foreign constructor continuation", cause, tm.core.pending.generation, old)
			}
			replacement := tm.core.pending
			polynomialImpPhysicalAudit(t, tm)
			tm.core.integer.Budget().FailAfter(-1)
			serviceMust(t, tm.CleanupConversion())
			if tm.core.pending.generation != replacement.generation || tm.core.pending.raw != replacement.raw {
				t.Fatal("scope aborted foreign factory")
			}
			serviceMust(t, tm.core.cleanupPendingCreation())
		})
		if found {
			break
		}
	}
	if !found {
		t.Fatal("actual final factory refusal not exercised")
	}
}

// monomial_manager::convert -> mk_monomial(sz, pws) -> tmp_monomial::init
// grows the temporary (increase_capacity(sz * 2)), so a monomial with more
// powers than the initial capacity converts (AUDIT math_polynomial_05-F2).
func TestMonomialConvertWideMonomialGrowsScratch(t *testing.T) {
	source, target := monomialFixture(t), monomialFixture(t)
	xs := make([]uint32, 200)
	for i := range xs {
		v, e := source.MkVar()
		serviceMust(t, e)
		xs[i] = v
	}
	a, e := source.MkFromVariables(xs)
	serviceMust(t, e)
	serviceMust(t, source.IncRef(a))
	t.Cleanup(func() { serviceMust(t, source.DecRef(a)) })
	r, e := target.Convert(source, a)
	serviceMust(t, e)
	serviceMust(t, target.IncRef(r))
	t.Cleanup(func() { serviceMust(t, target.DecRef(r)) })
	info, e := target.Info(r)
	serviceMust(t, e)
	if info.Size != 200 || info.TotalDegree != 200 {
		t.Fatal("wide converted monomial", info)
	}
}

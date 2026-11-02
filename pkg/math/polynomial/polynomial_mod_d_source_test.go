package polynomial

// Source: src/math/polynomial/polynomial.h:62-69; src/math/polynomial/polynomial.cpp:2117-2139,4899-5091,6898-6927.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func polynomialDegreeMap(t *testing.T, m *MultivariatePolynomialCore) *PolynomialVariableDegrees {
	t.Helper()
	d, e := NewPolynomialVariableDegrees(m.memory)
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, d.CloseChecked()) })
	return d
}
func TestPolynomialVariableDegreesSparseDefaultsAndRejectedMaxIndex(t *testing.T) {
	m := polynomialCellFixture(t)
	d := polynomialDegreeMap(t, m)
	serviceMust(t, d.SetDegree(3, 2))
	serviceMust(t, d.SetDegree(1, 5))
	for x, want := range []uint32{0, 5, 0, 2, 0, 0} {
		actual, e := d.Degree(uint32(x))
		serviceMust(t, e)
		if actual != want {
			t.Fatal(x, actual, want)
		}
	}
	// The Go boundary rejects an index that cannot be represented by the
	// source uint32 size without changing the existing map.
	e := d.SetDegree(^uint32(0), 7)
	if !errors.Is(e, arena.ErrBounds) {
		t.Fatal("actual endangered write", e)
	}
	actual, e := d.Degree(3)
	serviceMust(t, e)
	if actual != 2 {
		t.Fatal("rejected index changed existing degree", actual)
	}
}
func TestPolynomialModDIndependentMultivariateFilter(t *testing.T) {
	m := polynomialCellFixture(t)
	d := polynomialDegreeMap(t, m)
	serviceMust(t, d.SetDegree(0, 2))
	serviceMust(t, d.SetDegree(1, 3))
	p := polynomialQueryTerms(t, m, []int32{3, 5, 7, 11, 13, 17}, [][]MonomialPower{{{0, 2}}, {{0, 1}, {1, 2}}, {{1, 3}}, {{2, 4}}, nil, {{0, 3}, {2, 1}}})
	result, e := m.ModD(p, d)
	serviceMust(t, e)
	polynomialCaller(t, m, result)
	polynomialQueryExpect(t, m, result, map[[3]uint32]int64{{1, 2, 0}: 5, {0, 0, 4}: 11, {}: 13})
	// Constant source short circuit does not read the map storage.
	serviceMust(t, d.SourceFinalize())
	result, e = m.ModD(m.unit, d)
	serviceMust(t, e)
	if result != m.unit {
		t.Fatal("constant identity", result)
	}
}
func TestPolynomialPseudoModDActualSharedBranches(t *testing.T) {
	m := polynomialCellFixture(t)
	d := polynomialDegreeMap(t, m)
	serviceMust(t, d.SetDegree(1, 2))
	p := polynomialQueryTerms(t, m, []int32{1, 1, 1}, [][]MonomialPower{{{0, 2}}, {{1, 2}}, nil})
	divisor := polynomialQueryTerms(t, m, []int32{2, 1}, [][]MonomialPower{{{0, 1}}, nil})
	q, r := polynomialPseudoOutputs(t, m)
	serviceMust(t, m.ExactPseudoDivideModD(p, divisor, 0, d, q, r))
	a, e := q.Get()
	serviceMust(t, e)
	b, e := r.Get()
	serviceMust(t, e)
	polynomialQueryExpect(t, m, a, map[[3]uint32]int64{{1, 0, 0}: 2, {}: -1})
	polynomialQueryExpect(t, m, b, map[[3]uint32]int64{{}: 5})
	// Constant divisor branch applies public mod_d after Power and multiplication.
	constant := polynomialQueryTerms(t, m, []int32{2}, [][]MonomialPower{nil})
	serviceMust(t, m.ExactPseudoDivideModD(p, constant, 0, d, q, r))
	a, e = q.Get()
	serviceMust(t, e)
	polynomialQueryExpect(t, m, a, map[[3]uint32]int64{{2, 0, 0}: 4, {}: 4})
	// Exact adjustment executes both post-Mul public mod_d sites.
	serviceMust(t, d.SetDegree(0, 2))
	cube := polynomialQueryTerms(t, m, []int32{1}, [][]MonomialPower{{{0, 3}}})
	linear := polynomialQueryTerms(t, m, []int32{2}, [][]MonomialPower{{{0, 1}}})
	serviceMust(t, m.ExactPseudoDivideModD(cube, linear, 0, d, q, r))
	a, e = q.Get()
	serviceMust(t, e)
	b, e = r.Get()
	serviceMust(t, e)
	if a != m.zero || b != m.zero {
		t.Fatal("actual quotient and remainder post-adjustment filters", a, b)
	}
}

func TestPolynomialPseudoModDLeadingPowerAdjustment(t *testing.T) {
	m := polynomialCellFixture(t)
	d := polynomialDegreeMap(t, m)
	serviceMust(t, d.SetDegree(1, 2))
	p := polynomialQueryTerms(t, m, []int32{1, 1}, [][]MonomialPower{{{0, 3}}, nil})
	divisor := polynomialQueryTerms(t, m, []int32{1, 1}, [][]MonomialPower{{{0, 1}, {1, 1}}, {{0, 1}}})
	q, r := polynomialPseudoOutputs(t, m)
	serviceMust(t, m.ExactPseudoDivideModD(p, divisor, 0, d, q, r))
	a, e := q.Get()
	serviceMust(t, e)
	b, e := r.Get()
	serviceMust(t, e)
	// lc=y+1. One iteration gives Q=x²,R=y+1. Exact adjustment lc²
	// creates y² terms in BOTH products; source mod_d removes those terms.
	polynomialQueryExpect(t, m, a, map[[3]uint32]int64{{2, 1, 0}: 2, {2, 0, 0}: 1})
	polynomialQueryExpect(t, m, b, map[[3]uint32]int64{{0, 1, 0}: 3, {}: 1})
}
func TestPolynomialSOMModDSourceLogicalAssertionAndZeroReadMask(t *testing.T) {
	m := polynomialCellFixture(t)
	a := unumeral.NewInt(1)
	unit, e := m.monomials.Unit()
	serviceMust(t, e)
	serviceMust(t, m.som.add(&a, unit, false))
	serviceMust(t, m.som.as.reset(m))
	e = m.som.modD(nil)
	if polynomialSourceDebug {
		if !errors.Is(e, ErrPolynomialAssertion) {
			t.Fatal("source operator[] logical assertion", e)
		}
	} else if !errors.Is(e, arena.ErrBounds) {
		t.Fatal("native logical bounds", e)
	}
	serviceMust(t, m.som.as.appendCoefficient(m, unumeral.Int{}))
	// Actual cold map is finalized; NUM0 must avoid both monomial and map reads.
	d := polynomialDegreeMap(t, m)
	serviceMust(t, d.SourceFinalize())
	original, e := m.som.ms.monomial(m, 0)
	serviceMust(t, e)
	serviceMust(t, m.som.ms.putMonomial(m, 0, Monomial{}))
	e = m.som.modD(d)
	serviceMust(t, m.som.ms.putMonomial(m, 0, original))
	serviceMust(t, e)
}

func TestPolynomialVariableDegreesNativeGrowthIgnoresArenaFailure(t *testing.T) {
	b := arena.NewBudget(1 << 22)
	runtime, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b})
	serviceMust(t, e)
	d, e := NewPolynomialVariableDegrees(runtime)
	serviceMust(t, e)
	before := b.Used()
	b.FailAfter(0)
	serviceMust(t, d.SetDegree(9, 7))
	b.FailAfter(-1)
	if b.Used() != before || runtime.Stats().LiveBlocks != 0 {
		t.Fatal("native degree storage charged to Runtime", before, b.Used(), runtime.Stats())
	}
	for i := uint32(0); i < 10; i++ {
		value, e := d.Degree(i)
		serviceMust(t, e)
		want := uint32(0)
		if i == 9 {
			want = 7
		}
		if value != want {
			t.Fatal(i, value, want)
		}
	}
	serviceMust(t, d.CloseChecked())
	serviceMust(t, runtime.Close())
	if b.Used() != 0 {
		t.Fatal("complete refund", b.Used())
	}
}
func TestPolynomialPseudoModDPhysicalFailurePrefixes(t *testing.T) {
	failures, success := 0, false
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		reached := false
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m := polynomialCellFixture(t)
			d := polynomialDegreeMap(t, m)
			serviceMust(t, d.SetDegree(1, 2))
			p := polynomialQueryTerms(t, m, []int32{1, 1}, [][]MonomialPower{{{0, 3}}, nil})
			divisor := polynomialQueryTerms(t, m, []int32{1, 1}, [][]MonomialPower{{{0, 1}, {1, 1}}, {{0, 1}}})
			q, r := polynomialPseudoOutputs(t, m)
			serviceMust(t, q.Assign(m.unit))
			serviceMust(t, r.Assign(m.unit))
			m.integer.Budget().FailAfter(ordinal)
			e := m.ExactPseudoDivideModD(p, divisor, 0, d, q, r)
			polynomialModDPhysical(t, m, q, r, d, []Polynomial{p, divisor})
			polynomialQueryExpect(t, m, p, map[[3]uint32]int64{{3, 0, 0}: 1, {}: 1})
			polynomialQueryExpect(t, m, divisor, map[[3]uint32]int64{{1, 1, 0}: 1, {1, 0, 0}: 1})
			if e != nil {
				if !numericPhysicalReservationRefusal(e) {
					t.Fatal("actual reached reservation refusal", ordinal, e)
				}
				failures++
			} else {
				a, x := q.Get()
				serviceMust(t, x)
				b, x := r.Get()
				serviceMust(t, x)
				polynomialQueryExpect(t, m, a, map[[3]uint32]int64{{2, 1, 0}: 2, {2, 0, 0}: 1})
				polynomialQueryExpect(t, m, b, map[[3]uint32]int64{{0, 1, 0}: 3, {}: 1})
				reached = true
			}
			m.integer.Budget().FailAfter(-1)
			if m.pseudoDivision.active {
				serviceMust(t, m.CleanupPseudoDivision())
			}
			if m.powerLocal.active {
				serviceMust(t, m.CleanupPolynomialPower())
			}
		})
		if reached {
			t.Log("first mathematical success", ordinal)
			success = true
			break
		}
	}
	if failures == 0 || !success {
		t.Fatal(failures, success)
	}
}
func TestPolynomialModDReachedDegreeAndMapReadProfiles(t *testing.T) {
	m := polynomialCellFixture(t)
	d := polynomialDegreeMap(t, m)
	// Fresh actual partial record has SIZE and VAR initialized, DEG unread.
	// This diagnostic fixture does not assert completed C++ construction.
	term := monomialReadProfileObject(t, m.monomials, 1)
	monomialReadWrite(t, m.monomials, term, 20, 0)
	survives, e := m.monomialSurvivesModD(term, d)
	serviceMust(t, e)
	if !survives {
		t.Fatal("zero bound ignores degree")
	}
	serviceMust(t, d.SetDegree(0, 1))
	_, e = m.monomialSurvivesModD(term, d)
	monomialReadMissing(t, e)
	if polynomialSourceDebug {
		budget := arena.NewBudget(1 << 20)
		region, e := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: budget}})
		serviceMust(t, e)
		debug, e := diagnostic.NewDebugContext(region, budget)
		serviceMust(t, e)
		previous, e := diagnostic.BindProcessDebugContext(debug)
		serviceMust(t, e)
		defer func() {
			_, e := diagnostic.BindProcessDebugContext(previous)
			serviceMust(t, e)
			serviceMust(t, debug.Close())
			serviceMust(t, region.Close())
		}()
		q, r := polynomialPseudoOutputs(t, m)
		e = m.ExactPseudoDivideModD(Polynomial{}, Polynomial{}, 0, nil, q, r)
		if !errors.Is(e, ErrPolynomialAssertion) {
			t.Fatal("ModD map assertion before input reads", e)
		}
	}
}

func TestPolynomialModDWarmStorageAndColdOwnerGuards(t *testing.T) {
	m := polynomialCellFixture(t)
	d := polynomialDegreeMap(t, m)
	serviceMust(t, d.SetDegree(1, 2))
	p := polynomialQueryTerms(t, m, []int32{1, 1}, [][]MonomialPower{{{0, 3}}, nil})
	divisor := polynomialQueryTerms(t, m, []int32{1, 1}, [][]MonomialPower{{{0, 1}, {1, 1}}, {{0, 1}}})
	q, r := polynomialPseudoOutputs(t, m)
	operation := func() {
		if e := m.ExactPseudoDivideModD(p, divisor, 0, d, q, r); e != nil {
			panic(e)
		}
	}
	for i := 0; i < 20; i++ {
		operation()
	}
	before := numericWarmProfileSnapshot(t, m.integer, m.integer.Budget())
	monoBefore := m.monomials.allocator.Stats()
	valuesBefore := append([]uint32(nil), d.values...)
	sizeBefore, e := d.size()
	serviceMust(t, e)
	for i := 0; i < 20; i++ {
		operation()
	}
	numericWarmRuntimeAudit(t, m.integer, m.integer.Budget(), before, monoBefore.DirectObjects != 0)
	if m.monomials.allocator.Stats() != monoBefore || len(d.values) != len(valuesBefore) || m.pseudoDivision.active {
		t.Fatal("stable actual cohort")
	}
	for i := range valuesBefore {
		if d.values[i] != valuesBefore[i] {
			t.Fatal("degree map changed", i, d.values[i], valuesBefore[i])
		}
	}
	sizeAfter, e := d.size()
	serviceMust(t, e)
	if sizeBefore != sizeAfter {
		t.Fatal(sizeBefore, sizeAfter)
	}
	polynomialModDPhysical(t, m, q, r, d, []Polynomial{p, divisor})
	copied := *d
	used := m.integer.Budget().Used()
	if !errors.Is(copied.CloseChecked(), ErrPolynomialOwner) || m.integer.Budget().Used() != used {
		t.Fatal("shallow cold-owner copy cannot free/refund real backing")
	}
	var zero PolynomialVariableDegrees
	if !errors.Is(zero.CloseChecked(), ErrPolynomialOwner) {
		t.Fatal("zero-owner close")
	}
}

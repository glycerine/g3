package polynomial

// Source: src/math/polynomial/polynomial.cpp:6436-6522; src/util/scoped_numeral.h;
// src/util/mpq.cpp.

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func polynomialEvaluationAudit(t *testing.T, m *MultivariatePolynomialManager, p Polynomial, values *polynomialEvalRationalMap, out *unumeral.Rational) {
	t.Helper()
	inputs := []*unumeral.Int{&(*out.BorrowNumerator()), &(*out.BorrowDenominator())}
	for i := range values.values {
		inputs = append(inputs, &(*values.values[i].BorrowNumerator()), &(*values.values[i].BorrowDenominator()))
	}
	polynomialImpPhysicalAuditNamed(t, m, []Polynomial{p}, inputs, 0)
}
func TestPolynomialEvaluationLiveAliasSource(t *testing.T) {
	m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{2, 0, 0}}, {1, [3]uint32{1, 0, 0}}, {1, [3]uint32{}}})
	q := m.core.config.Rationals
	v := polynomialEvalRationalMap{manager: q}
	for i := range v.values {
		v.values[i] = unumeral.NewRational()
	}
	serviceMust(t, q.SetInt32(&v.values[0], 2))
	serviceMust(t, m.EvaluateRational(p, &v, &v.values[0]))
	want := unumeral.NewRational()
	serviceMust(t, q.SetInt32(&want, 5))
	equal, e := q.Equal(&v.values[0], &want)
	serviceMust(t, e)
	if !equal {
		t.Fatal("source borrowed x_value changes with aliased output: Horner prefixes 1→2→4→5")
	}
	polynomialEvaluationAudit(t, m, p, &v, &want)
	for i := range v.values {
		serviceMust(t, q.Del(&v.values[i]))
	}
	serviceMust(t, q.Del(&want))
}
func TestPolynomialEvaluationCancellationAndCachedFrames(t *testing.T) {
	m, p, c := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{2, 1, 0}}, {2, [3]uint32{1, 0, 0}}, {7, [3]uint32{}}})
	q := m.core.config.Rationals
	v := polynomialEvalRationalMap{manager: q}
	for i := range v.values {
		v.values[i] = unumeral.NewRational()
		serviceMust(t, q.SetInt32(&v.values[i], 2))
	}
	out := unumeral.NewRational()
	serviceMust(t, m.LexSort(p))
	c.limit = c.checkpoints
	e := m.EvaluateRational(p, &v, &out)
	if !errors.Is(e, ErrUPolynomialCanceled) {
		t.Fatal("actual Horner checkpoint", e)
	}
	if m.evaluation.active || m.evaluation.depth != 0 {
		t.Fatal("completed auxiliary dtors remain active")
	}
	polynomialEvaluationAudit(t, m, p, &v, &out)
	c.limit = 100000
	serviceMust(t, m.EvaluateRational(p, &v, &out))
	count := len(m.evaluation.rationalFrames)
	for i := 0; i < 10; i++ {
		serviceMust(t, m.EvaluateRational(p, &v, &out))
		if len(m.evaluation.rationalFrames) != count {
			t.Fatal("successful source locals allocated a new cache object")
		}
		for _, f := range m.evaluation.rationalFrames {
			if f.constructed || (*f.aux.BorrowNumerator()).Cell != 0 || (*f.aux.BorrowDenominator()).Cell != 0 {
				t.Fatal("cached completed local owns a numeral")
			}
		}
	}
	polynomialEvaluationAudit(t, m, p, &v, &out)
	for i := range v.values {
		serviceMust(t, q.Del(&v.values[i]))
	}
	serviceMust(t, q.Del(&out))
}
func TestPolynomialEvaluationEveryReservationOwnedRaw(t *testing.T) {
	for _, synch := range []bool{false, true} {
		t.Run(fmt.Sprint("synch", synch), func(t *testing.T) { polynomialEvaluationEveryReservation(t, synch) })
	}
}
func polynomialEvaluationEveryReservation(t *testing.T, synch bool) {
	failures, raw := 0, 0
	success := false
	for ordinal := int64(0); ordinal < 100; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			q, _ := rationalManager(t, synch)
			m, p, _ := polynomialEvalFixtureWithRationals(t, q, []polynomialEvalTerm{{1, [3]uint32{3, 2, 0}}, {2, [3]uint32{1, 0, 0}}, {7, [3]uint32{}}})
			budget := q.Budget()
			values := polynomialEvalRationalMap{manager: q}
			for i := range values.values {
				values.values[i] = unumeral.NewRational()
			}
			serviceMust(t, q.SetString(&values.values[0], "184467440737095516170000000000000000000003"))
			serviceMust(t, q.SetFraction(&values.values[1], 13, 7))
			serviceMust(t, q.SetInt32(&values.values[2], 2))
			out := unumeral.NewRational()
			budget.FailAfter(ordinal)
			e := m.EvaluateRational(p, &values, &out)
			polynomialEvaluationAudit(t, m, p, &values, &out)
			if e == nil {
				success = true
			} else {
				failures++
				if !numericPhysicalReservationRefusal(e) {
					t.Fatal("exact reservation category", e)
				}
				if m.evaluation.groups != ([3]uint64{}) {
					raw++
				}
				if m.evaluation.active {
					if !errors.Is(m.CloseChecked(), ErrPolynomialPending) {
						t.Fatal("owning close bypassed retained lexical operation")
					}
				}
			}
			budget.FailAfter(-1)
			// The operation-specific adapter alone must drain its exact dependent roots.
			hadLex := m.lexPending
			serviceMust(t, m.CleanupEvaluation())
			if hadLex && m.lexPending {
				t.Fatal("evaluator cleanup omitted its actually reached LexSort continuation")
			}
			polynomialEvaluationAudit(t, m, p, &values, &out)
			if q.RawArithmetic.Pending() {
				t.Fatal("operation returned successful cleanup with its actual RAW group retained")
			}
			for i := range values.values {
				serviceMust(t, q.Del(&values.values[i]))
			}
			serviceMust(t, q.Del(&out))
		})
		if success {
			break
		}
	}
	if !success || failures == 0 || raw == 0 {
		t.Fatal("every-prefix campaign must reach real RAW ownership and success", failures, raw, success)
	}
}
func TestPolynomialEvaluationManagerGuardBeforeFrames(t *testing.T) {
	m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{1, 0, 0}}})
	foreign, _ := rationalManager(t, false)
	v := polynomialEvalRationalMap{manager: foreign}
	out := unumeral.NewRational()
	before := m.originalBudget.Used()
	e := m.EvaluateRational(p, &v, &out)
	if !errors.Is(e, arena.ErrConfig) || v.managers != 1 || m.originalBudget.Used() != before || len(m.evaluation.rationalFrames) != 0 {
		t.Fatal("actual VM cohort guard precedes charged frame construction", e)
	}
}

func TestPolynomialEvaluationRawReceiptForeignReplacement(t *testing.T) {
	found := false
	for ordinal := int64(0); ordinal < 100 && !found; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{3, 0, 0}}, {7, [3]uint32{}}})
			q := m.core.config.Rationals
			v := polynomialEvalRationalMap{manager: q}
			for i := range v.values {
				v.values[i] = unumeral.NewRational()
			}
			// An armed (never reached) injection keeps GCD on the Lehmer
			// path, whose warmed allocator state this sweep was calibrated on.
			q.Budget().FailAfter(1 << 40)
			serviceMust(t, q.SetString(&v.values[0], "184467440737095516170000000000000000003"))
			q.Budget().FailAfter(-1)
			out := unumeral.NewRational()
			q.Budget().FailAfter(ordinal)
			e := m.EvaluateRational(p, &v, &out)
			polynomialEvaluationAudit(t, m, p, &v, &out)
			receipt := m.evaluation.groups[2]
			q.Budget().FailAfter(-1)
			if e != nil && receipt != 0 && q.RawArithmetic.PowerActive {
				found = true
				serviceMust(t, q.CleanupRawGroup(unumeral.RationalRawPower))
				// A second actual Power call starts the SAME concrete raw group. Its
				// physical failed-copy prefix is foreign to the stopped evaluation.
				old := q.RawGeneration
				q.Budget().FailAfter(0)
				e = q.Power(&v.values[0], 3, &v.values[1])
				q.Budget().FailAfter(-1)
				if e == nil || (!numericPhysicalReservationRefusal(e) && !errors.Is(e, arena.ErrInvalidSlot)) || !q.RawArithmetic.PowerActive || q.RawArithmetic.PowerGeneration <= old {
					t.Fatal("must reach actual same-group replacement", e)
				}
				generation := q.RawArithmetic.PowerGeneration
				before := q.RawArithmetic.Power
				polynomialEvaluationAudit(t, m, p, &v, &out)
				serviceMust(t, m.CleanupEvaluation())
				if !q.RawArithmetic.PowerActive || q.RawArithmetic.PowerGeneration != generation || q.RawArithmetic.Power != before {
					t.Fatal("stale evaluation receipt cleaned foreign replacement")
				}
				polynomialEvaluationAudit(t, m, p, &v, &out)
				serviceMust(t, q.CleanupRawGroup(unumeral.RationalRawPower))
			} else {
				serviceMust(t, m.CleanupEvaluation())
			}
			for i := range v.values {
				serviceMust(t, q.Del(&v.values[i]))
			}
			serviceMust(t, q.Del(&out))
		})
	}
	if !found {
		t.Fatal("no actual stopped Power group reached")
	}
}

func TestPolynomialEvaluationWarmInlineSourceLocals(t *testing.T) {
	m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{2, 1, 0}}, {2, [3]uint32{1, 0, 0}}, {7, [3]uint32{}}})
	q := m.core.config.Rationals
	v := polynomialEvalRationalMap{manager: q}
	for i := range v.values {
		v.values[i] = unumeral.NewRational()
		serviceMust(t, q.SetInt32(&v.values[i], 2))
	}
	out := unumeral.NewRational()
	serviceMust(t, m.EvaluateRational(p, &v, &out))
	before := q.Budget().Used()
	source, e := q.Integers().RuntimeOwner().AllocationCount()
	serviceMust(t, e)
	allocations := testing.AllocsPerRun(100, func() {
		if e := m.EvaluateRational(p, &v, &out); e != nil {
			panic(e)
		}
	})
	after, e := q.Integers().RuntimeOwner().AllocationCount()
	serviceMust(t, e)
	if allocations != 0 || q.Budget().Used() != before || after != source {
		t.Fatal("inline lexical locals reuse real charged frame headers without allocations/events", allocations, before, q.Budget().Used())
	}
	polynomialEvaluationAudit(t, m, p, &v, &out)
	for i := range v.values {
		serviceMust(t, q.Del(&v.values[i]))
	}
	serviceMust(t, q.Del(&out))
}

func TestPolynomialEvaluationLexReceiptForeignReplacement(t *testing.T) {
	m, _, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{}}})
	p := polynomialLexWrappedPolynomial(t, m)
	q := m.core.config.Rationals
	values := polynomialEvalRationalMap{manager: q}
	for i := range values.values {
		values.values[i] = unumeral.NewRational()
		serviceMust(t, q.SetInt32(&values.values[i], 2))
	}
	out := unumeral.NewRational()
	t.Cleanup(func() {
		serviceMust(t, m.CleanupEvaluation())
		serviceMust(t, m.CleanupLexSort())
		for i := range values.values {
			serviceMust(t, q.Del(&values.values[i]))
		}
		serviceMust(t, q.Del(&out))
	})
	cause := m.EvaluateRational(p, &values, &out)
	expected := error(arena.ErrBounds)
	if polynomialSourceDebug {
		expected = ErrPolynomialAssertion
	}
	if cause != expected || m.evaluation.lexGeneration == 0 {
		t.Fatal("evaluation did not reach native LexSort bounds stop", cause)
	}
	old := m.evaluation.lexGeneration
	serviceMust(t, m.CleanupLexSort())
	foreign := polynomialLexWrappedPolynomial(t, m)
	cause = m.LexSort(foreign)
	if cause != expected || !m.lexPending || m.lexGeneration == old {
		t.Fatal("later distinct LexSort did not stop", cause)
	}
	generation := m.lexGeneration
	buckets := make([][]uint32, len(m.lexBuckets))
	for i, b := range m.lexBuckets {
		if b != nil {
			buckets[i] = make([]uint32, len(b))
			copy(buckets[i], b)
		}
	}
	permutation := append([]uint32(nil), m.lexPermutation...)
	polynomialEvaluationAudit(t, m, p, &values, &out)
	serviceMust(t, m.CleanupEvaluation())
	if !m.lexPending || m.lexGeneration != generation || m.lexCause != cause || !reflect.DeepEqual(m.lexBuckets, buckets) || !reflect.DeepEqual(m.lexPermutation, permutation) {
		t.Fatal("stale evaluator receipt changed later LexSort scratch")
	}
	polynomialEvaluationAudit(t, m, p, &values, &out)
	serviceMust(t, m.CleanupLexSort())
}

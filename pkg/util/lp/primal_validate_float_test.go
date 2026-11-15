package lp

import (
	"bytes"
	"errors"
	"io"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestLPFloatPrimalValidationIndependentRows(t *testing.T) {
	rng := rand.New(rand.NewSource(271358))
	for world := 0; world < 200; world++ {
		m := lpFloatPrimal(t)
		solution := make(map[string]float64)
		values := make([]float64, 5)
		for j := 0; j < 5; j++ {
			name := string(rune('a' + j))
			c := m.GetOrCreateColumnInfo(uint32(j))
			c.Name = name
			c.SetLowBound(-8)
			c.SetUpperBound(8)
			values[j] = float64(rng.Intn(25)-12) / 2
			solution[name] = values[j]
		}
		all := true
		for i := 0; i < 4; i++ {
			coeff := make([]float64, 5)
			sum := 0.0
			for j := 0; j < 5; j++ {
				coeff[j] = float64(rng.Intn(9)-4) / 2
				sum += coeff[j] * values[j]
			}
			// Dictionary order differs from the independent dense coordinate sum.
			for _, j := range rng.Perm(5) {
				m.SetRowColumnCoefficient(uint32(i), uint32(j), coeff[j])
			}
			relation := LPRelation((world + i) % 3)
			offset := float64(rng.Intn(3)-1) / 4
			right := sum + offset
			lpStaticMust(t, m.AddConstraint(relation, right, uint32(i)))
			want := false
			switch relation {
			case RelationEqual:
				want = offset == 0
			case RelationLessOrEqual:
				want = offset >= 0
			case RelationGreaterOrEqual:
				want = offset <= 0
			}
			row, err := m.GetRowValue(uint32(i), solution, nil)
			if err != nil || row != sum {
				t.Fatal(world, i, row, sum, err)
			}
			got, err := m.RowConstraintHolds(uint32(i), solution, nil)
			if err != nil || got != want {
				t.Fatal(world, i, got, want, err)
			}
			all = all && want
		}
		got, err := m.SolutionIsFeasible(solution)
		if err != nil || got != all {
			t.Fatal(world, got, all, err)
		}
	}
}

func TestLPFloatPrimalValidationToleranceAndIEEE(t *testing.T) {
	m := lpFloatPrimal(t)
	c := m.GetOrCreateColumnInfo(7)
	c.Name = "x"
	m.SetRowColumnCoefficient(4, 7, 1)
	for _, relation := range []LPRelation{RelationLessOrEqual, RelationEqual, RelationGreaterOrEqual} {
		m.Constraints[4] = FloatConstraint{0, relation}
		for _, x := range []float64{0, 1e-5, -1e-5, math.Nextafter(1e-5, math.Inf(1)), math.Nextafter(-1e-5, math.Inf(-1)), math.NaN(), math.Inf(1), math.Inf(-1)} {
			want := true
			if !math.IsNaN(x) {
				switch relation {
				case RelationEqual:
					want = math.Abs(x) <= 1e-5
				case RelationLessOrEqual:
					want = x <= 1e-5
				case RelationGreaterOrEqual:
					want = x >= -1e-5
				}
			}
			got, err := m.RowConstraintHolds(4, map[string]float64{"x": x}, nil)
			if err != nil || got != want {
				t.Fatal(relation, x, got, want, err)
			}
		}
	}
	c.SetLowBound(0)
	c.SetUpperBound(0)
	for _, x := range []float64{-1e-7, 1e-6, math.Nextafter(-1e-7, math.Inf(-1)), math.Nextafter(1e-6, math.Inf(1)), math.NaN()} {
		want := x >= -1e-7 && x <= 1e-6
		got, err := m.BoundsHold(map[string]float64{"x": x})
		if err != nil || got != want {
			t.Fatal(x, got, want, err)
		}
	}
	// Fixed flag alone is not read by source bounds_hold; unset low/upper
	// flags accept any value, including NaN, independent of FixedValue.
	c.LowSet = false
	c.UpperSet = false
	c.SetFixedValue(4)
	if ok, err := m.BoundsHold(map[string]float64{"x": math.NaN()}); err != nil || !ok {
		t.Fatal(ok, err)
	}
	c.SetLowBound(0)
	delete(m.Constraints, 4)
	if ok, err := m.SolutionIsFeasible(map[string]float64{"x": -1}); err != nil || ok {
		t.Fatal(ok, err)
	}
	if _, exists := m.Constraints[4]; exists {
		t.Fatal("bounds short circuit")
	}
}

type lpValidationWriter struct {
	buffer  bytes.Buffer
	allowed int
	err     error
}

func (w *lpValidationWriter) String() string { return w.buffer.String() }

func (w *lpValidationWriter) Write(p []byte) (int, error) {
	if w.allowed == 0 {
		return 0, w.err
	}
	w.allowed--
	return w.buffer.Write(p)
}

func TestLPFloatPrimalValidationOutputAndMutationPrefixes(t *testing.T) {
	m := lpFloatPrimal(t)
	m.GetOrCreateColumnInfo(1).Name = "a"
	m.GetOrCreateColumnInfo(2).Name = "b"
	m.SetRowColumnCoefficient(0, 1, 2)
	m.SetRowColumnCoefficient(0, 2, -3)
	solution := map[string]float64{"a": 4, "b": 1}
	var out bytes.Buffer
	value, err := m.GetRowValue(0, solution, &out)
	// get_row_value walks the row in libstdc++ order: column 2, then 1.
	if err != nil || value != 5 || out.String() != "-3(b=1) 2(a=4)  = 5\n" {
		t.Fatal(value, err, out.String())
	}
	for _, relation := range []LPRelation{RelationEqual, RelationLessOrEqual, RelationGreaterOrEqual} {
		right := 4.0
		comparison := "="
		if relation == RelationGreaterOrEqual {
			right = 6
			comparison = ">="
		}
		if relation == RelationLessOrEqual {
			comparison = "<="
		}
		m.Constraints[0] = FloatConstraint{right, relation}
		out.Reset()
		ok, err := m.RowConstraintHolds(0, solution, &out)
		if err != nil || ok || !strings.HasSuffix(out.String(), "should be "+comparison+" "+floatLPNumberText(right)+"\n") {
			t.Fatal(relation, ok, err, out.String())
		}
	}
	want := errors.New("validation output refused")
	for allowed := 0; allowed < 4; allowed++ {
		w := &lpValidationWriter{allowed: allowed, err: want}
		ok, err := m.RowConstraintHolds(0, solution, w)
		if ok || err != want {
			t.Fatal(allowed, ok, err, w.String())
		}
	}
	w := &lpValidationWriter{allowed: 1, err: want}
	value, err = m.GetRowValue(0, solution, w)
	if err != want || value != -3 || w.String() != "-3(b=1) " {
		t.Fatal(value, err, w.String())
	}
	delete(m.Constraints, 0)
	out.Reset()
	if ok, err := m.RowConstraintHolds(0, solution, &out); ok || err != ErrBounds || out.String() != "-3(b=1) 2(a=4)  = 5\n" {
		t.Fatal(ok, err, out.String())
	}
	if _, exists := m.Constraints[0]; !exists {
		t.Fatal("default constraint key not published")
	}
}

func TestLPFloatPrimalValidationMissingMetadata(t *testing.T) {
	m := lpFloatPrimal(t)
	m.GetOrCreateColumnInfo(7).Name = "x"
	var source z3exception.DefaultException
	if ok, err := m.BoundsHold(nil); ok || !errors.As(err, &source) || source.Message() != "cannot find column 7 in solution" {
		t.Fatal(ok, err)
	}
	if _, err := m.GetRowValue(99, nil, nil); !errors.As(err, &source) || source.Message() != "cannot find row 99" {
		t.Fatal(err)
	}
	m.SetRowColumnCoefficient(1, 7, 1)
	m.Constraints[1] = FloatConstraint{0, RelationEqual}
	want := ErrBounds
	if sourceDebugEnabled {
		want = ErrAssertion
	}
	if _, err := m.GetRowValue(1, nil, nil); err != want {
		t.Fatal(err)
	}
	delete(m.Columns, 7)
	if _, err := m.GetRowValue(1, nil, nil); err != want {
		t.Fatal(err)
	}
	m.Columns[7] = nil
	if ok, err := m.BoundsHold(nil); ok || err != ErrBounds {
		t.Fatal(ok, err)
	}
	if _, err := m.GetRowValue(1, nil, nil); err != ErrBounds {
		t.Fatal(err)
	}
	m.GetOrCreateColumnInfo(8).Name = "z"
	m.SetRowColumnCoefficient(2, 8, 1)
	m.Constraints[2] = FloatConstraint{0, LPRelation(99)}
	errWant := error(nil)
	if sourceDebugEnabled {
		errWant = ErrAssertion
	}
	if ok, err := m.RowConstraintHolds(2, map[string]float64{"z": 0}, nil); ok || err != errWant {
		t.Fatal(ok, err)
	}
	// row_constraints_hold walks m_A_values in libstdc++ order and reaches
	// row 2 (whose relation 99 trips the debug assertion) before row 1.
	rowsWant := error(ErrBounds)
	if sourceDebugEnabled {
		rowsWant = ErrAssertion
	}
	if ok, err := m.RowConstraintsHold(nil); ok || err != rowsWant {
		t.Fatal(ok, err)
	}
	m.AValues.Set(3, nil)
	if _, err := m.GetRowValue(3, nil, nil); err != ErrBounds {
		t.Fatal(err)
	}
	// A retained zero coefficient still reads its column name and solution.
	m.SetRowColumnCoefficient(4, 8, 0)
	if _, err := m.GetRowValue(4, nil, nil); err != want {
		t.Fatal(err)
	}
}

// Source lp_primal_simplex.hpp:289-346: term insertion precedes its sum,
// and each completed row/mismatch report ends with std::endl.
type lpValidationPrefixWriter struct {
	larConstraintPrefixWriter
	flushes, failAt int
}

func (w *lpValidationPrefixWriter) Flush() error {
	w.flushes++
	if w.flushes == w.failAt {
		return lpErrResidualWrite
	}
	return nil
}

func TestLPFloatPrimalValidationEveryReportByteAndEndl(t *testing.T) {
	m := lpFloatPrimal(t)
	m.GetOrCreateColumnInfo(1).Name = "a"
	m.GetOrCreateColumnInfo(2).Name = "b"
	m.SetRowColumnCoefficient(0, 1, 2)
	m.SetRowColumnCoefficient(0, 2, -3)
	solution := map[string]float64{"a": 4, "b": 1}
	// libstdc++ row order: column 2 (b), then column 1 (a).
	first, second := "-3(b=1) ", "2(a=4) "
	row := first + second + " = 5\n"
	for _, refused := range []error{nil, lpErrResidualWrite} {
		wantErr := refused
		if wantErr == nil {
			wantErr = io.ErrShortWrite
		}
		for n := 0; n < len(row); n++ {
			w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: n, err: refused}}
			got, err := m.GetRowValue(0, solution, w)
			want := 0.0
			if n >= len(first) {
				want = -3
			}
			if n >= len(first)+len(second) {
				want = 5
			}
			if err != wantErr || got != want || w.output.String() != row[:n] || w.flushes != 0 {
				t.Fatal("row prefix", n, got, want, err, w.output.String(), w.flushes)
			}
		}
		for _, tc := range []struct {
			relation   LPRelation
			right      float64
			diagnostic string
		}{
			{RelationEqual, 4, "should be = 4\n"},
			{RelationLessOrEqual, 4, "should be <= 4\n"},
			{RelationGreaterOrEqual, 6, "should be >= 6\n"},
			{RelationEqual, 5, ""},
		} {
			m.Constraints[0] = FloatConstraint{tc.right, tc.relation}
			report := row + tc.diagnostic
			for n := 0; n < len(report); n++ {
				w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: n, err: refused}}
				ok, err := m.RowConstraintHolds(0, solution, w)
				flushes := 0
				if n >= len(row) {
					flushes = 1
				}
				if ok || err != wantErr || w.output.String() != report[:n] || w.flushes != flushes {
					t.Fatal("constraint prefix", tc, n, ok, err, w.output.String(), w.flushes, flushes)
				}
			}
			flushes := 1
			if tc.diagnostic != "" {
				flushes = 2
			}
			for fail := 0; fail <= flushes; fail++ {
				w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: len(report)}, failAt: fail}
				ok, err := m.RowConstraintHolds(0, solution, w)
				if fail == 0 {
					if err != nil || ok != (tc.diagnostic == "") || w.flushes != flushes || w.output.String() != report {
						t.Fatal(tc, ok, err, w.flushes, w.output.String())
					}
				} else {
					prefix := report
					if fail == 1 {
						prefix = row
					}
					if ok || err != lpErrResidualWrite || w.flushes != fail || w.output.String() != prefix {
						t.Fatal(tc, fail, ok, err, w.flushes, w.output.String())
					}
				}
			}
		}
	}
	// A failed row endl precedes the source constraint operator[] insertion.
	delete(m.Constraints, 0)
	w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: len(row)}, failAt: 1}
	if ok, err := m.RowConstraintHolds(0, solution, w); ok || err != lpErrResidualWrite {
		t.Fatal(ok, err)
	}
	if _, exists := m.Constraints[0]; exists {
		t.Fatal("constraint published before row endl completed")
	}
}

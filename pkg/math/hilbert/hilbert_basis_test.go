package hilbert

import (
	"errors"
	"sync"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var testQOnce struct {
	sync.Once
	q *numeral.RationalManager
}

// testQ is the tests' rational manager (the source's global g_mpq_manager).
func testQ() *numeral.RationalManager {
	testQOnce.Do(func() {
		q, err := numeral.NewRationalManager(numeral.RationalConfig{})
		if err != nil {
			panic(err)
		}
		testQOnce.q = q
	})
	return testQOnce.q
}

func rationals(values ...int64) []numeral.Rational {
	result := make([]numeral.Rational, len(values))
	for i, value := range values {
		result[i] = numeral.NewRational()
		if err := testQ().SetInt64(&result[i], value); err != nil {
			panic(err)
		}
	}
	return result
}

func rational(value int64) *numeral.Rational {
	return &rationals(value)[0]
}

func zeroRationals(n int) []numeral.Rational {
	return rationals(make([]int64, n)...)
}

func validateBasis(t *testing.T, basis *Basis) {
	t.Helper()
	for i := uint32(0); i < basis.BasisSize(); i++ {
		solution, initial, err := basis.BasisSolution(i)
		if err != nil {
			t.Fatalf("basis solution %d: %v", i, err)
		}
		for j := range basis.inequalities {
			coefficients, bound, equal, err := basis.Inequality(uint32(j))
			if err != nil {
				t.Fatalf("inequality %d: %v", j, err)
			}
			q := testQ()
			sum, product := numeral.NewRational(), numeral.NewRational()
			for k := range solution {
				if err := q.Mul(&coefficients[k], &solution[k], &product); err != nil {
					t.Fatal(err)
				}
				if err := q.Add(&sum, &product, &sum); err != nil {
					t.Fatal(err)
				}
			}
			cmp, err := q.Compare(&sum, &bound)
			if err != nil {
				t.Fatal(err)
			}
			sign, err := q.Sign(&sum)
			if err != nil {
				t.Fatal(err)
			}
			text, _ := q.String(&sum)
			if initial {
				if equal && cmp != 0 || !equal && cmp < 0 {
					t.Fatalf("initial basis solution %d violates inequality %d: sum=%s equal=%v", i, j, text, equal)
				}
			} else if equal && sign != 0 || !equal && sign < 0 {
				t.Fatalf("homogeneous basis solution %d violates inequality %d: sum=%s equal=%v", i, j, text, equal)
			}
		}
	}
}

func TestBasisSourceExamples(t *testing.T) {
	tests := []struct {
		name string
		add  func(*testing.T, *Basis)
	}{
		{
			name: "Ajili-Contejean-example-9",
			add: func(t *testing.T, basis *Basis) {
				t.Helper()
				if err := basis.AddEQ(rationals(1, 1, -2), rational(0)); err != nil {
					t.Fatal(err)
				}
				if err := basis.AddEQ(rationals(1, 0, -1), rational(0)); err != nil {
					t.Fatal(err)
				}
				if err := basis.AddLE(rationals(0, 1, -1), rational(0)); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "affine-equalities",
			add: func(t *testing.T, basis *Basis) {
				t.Helper()
				if err := basis.AddEQ(rationals(0, 1), rational(0)); err != nil {
					t.Fatal(err)
				}
				if err := basis.AddEQ(rationals(1, -1), rational(2)); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			basis := New(testQ(), nil)
			test.add(t, basis)
			result, err := basis.Saturate()
			if err != nil || result != lbool.True {
				t.Fatalf("Saturate() = %v, %v", result, err)
			}
			if basis.BasisSize() == 0 {
				t.Fatal("expected non-empty basis")
			}
			validateBasis(t, basis)
		})
	}
}

func TestBasisSourceA553RegisteredWorkload(t *testing.T) {
	basis := New(testQ(), nil)
	for i := 0; i < 15; i++ {
		row := zeroRationals(75)
		for j := 0; j < 5; j++ {
			for k := 0; k < 15; k++ {
				if k == i {
					row[j*15+k] = *rational(1)
				}
			}
		}
		if err := basis.AddGE(row, rational(0)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 15; i++ {
		row := zeroRationals(75)
		for k := 1; k <= 5; k++ {
			for l := 1; l <= 5; l++ {
				for j := 1; j <= 3; j++ {
					if j*k <= i && (i-j)%3 == 0 {
						index := ((k-1)*5+(l-1))*3 + (j - 1)
						row[index] = *rational(1)
					}
				}
			}
		}
		if err := basis.AddGE(row, rational(0)); err != nil {
			t.Fatal(err)
		}
	}
	if len(basis.inequalities) != 30 {
		t.Fatalf("registered workload has %d inequalities, want 30", len(basis.inequalities))
	}
	for i, inequality := range basis.inequalities {
		if len(inequality.values) != 76 {
			t.Fatalf("inequality %d has %d coordinates, want 76", i, len(inequality.values))
		}
	}
	result, err := basis.Saturate()
	if err != nil || result != lbool.True {
		t.Fatalf("registered A_5_5_3 saturation = %v, %v", result, err)
	}
	validateBasis(t, basis)
}

func TestBasisIntegerCoordinatesAndCancellation(t *testing.T) {
	basis := New(testQ(), func() bool { return false })
	basis.SetIsInt(0)
	if !basis.IsInt(0) {
		t.Fatal("integer coordinate was not retained")
	}
	if err := basis.AddGE(rationals(1), rational(0)); err != nil {
		t.Fatal(err)
	}
	result, err := basis.Saturate()
	if err != nil || result != lbool.Undef {
		t.Fatalf("cancelled Saturate() = %v, %v", result, err)
	}
}

func TestBasisRefusesInvalidNumeralsAndDimensions(t *testing.T) {
	basis := New(testQ(), nil)
	if err := basis.AddGE(halfRationals(), rational(0)); !errors.Is(err, ErrOverflow) {
		t.Fatalf("fractional coefficient error = %v", err)
	}
	if err := basis.AddGE(rationals(1, 2), rational(0)); err != nil {
		t.Fatal(err)
	}
	if err := basis.AddGE(rationals(1), rational(0)); !errors.Is(err, ErrDimension) {
		t.Fatalf("dimension error = %v", err)
	}
}

func halfRationals() []numeral.Rational {
	result := []numeral.Rational{numeral.NewRational()}
	if err := testQ().SetFractionSigned(&result[0], 1, 2); err != nil {
		panic(err)
	}
	return result
}

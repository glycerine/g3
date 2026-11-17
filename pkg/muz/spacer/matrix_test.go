package spacer

// Source: src/muz/spacer/spacer_matrix.cpp normalization and reached Gaussian
// elimination assignment/subtraction order.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// spacerTestRational is rational(n, d) of the test manager's rational
// manager; small values own no cell.
func spacerTestRational(t *testing.T, q *numeral.RationalManager, n, d int64) *numeral.Rational {
	t.Helper()
	r := numeral.NewRational()
	if err := q.SetInt64(&r, n); err != nil {
		t.Fatal(err)
	}
	if d != 1 {
		den := numeral.NewRational()
		if err := q.SetInt64(&den, d); err != nil {
			t.Fatal(err)
		}
		if err := q.Div(&r, &den, &r); err != nil {
			t.Fatal(err)
		}
	}
	return &r
}

func spacerTestEqual(t *testing.T, q *numeral.RationalManager, got *numeral.Rational, n, d int64) bool {
	t.Helper()
	c, err := q.Compare(got, spacerTestRational(t, q, n, d))
	if err != nil {
		t.Fatal(err)
	}
	return c == 0
}

func TestSpacerMatrixNormalizeAndSourceEliminationOrder(t *testing.T) {
	m, _ := newSpacerTestServices(t)
	q := m.NumeralService().Manager
	normalized := newSpacerMatrix(q, 1, 2)
	defer normalized.del()
	if err := normalized.set(0, 0, spacerTestRational(t, q, 1, 2)); err != nil {
		t.Fatal(err)
	}
	if err := normalized.set(0, 1, spacerTestRational(t, q, 1, 3)); err != nil {
		t.Fatal(err)
	}
	if err := normalized.normalize(); err != nil {
		t.Fatal(err)
	}
	first, _ := normalized.get(0, 0)
	second, _ := normalized.get(0, 1)
	if !spacerTestEqual(t, q, first, 3, 1) || !spacerTestEqual(t, q, second, 2, 1) {
		t.Fatal("normalized row", first, second)
	}

	ordered := newSpacerMatrix(q, 2, 2)
	defer ordered.del()
	_ = ordered.set(0, 0, spacerTestRational(t, q, 1, 1))
	_ = ordered.set(0, 1, spacerTestRational(t, q, 2, 1))
	_ = ordered.set(1, 1, spacerTestRational(t, q, 1, 1))
	rank, err := ordered.gaussianElimination()
	if err != nil || rank != 2 {
		t.Fatal("ordered rank", rank, err)
	}
	upper, _ := ordered.get(0, 1)
	if !spacerTestEqual(t, q, upper, 2, 1) {
		t.Fatal("source subtraction unexpectedly revisited row zero", upper)
	}

	swapped := newSpacerMatrix(q, 2, 2)
	defer swapped.del()
	_ = swapped.set(0, 1, spacerTestRational(t, q, 1, 1))
	_ = swapped.set(1, 0, spacerTestRational(t, q, 2, 1))
	_ = swapped.set(1, 1, spacerTestRational(t, q, 3, 1))
	rank, err = swapped.gaussianElimination()
	if err != nil || rank != 1 {
		t.Fatal("source row-reorder assignment parity", rank, err)
	}
}

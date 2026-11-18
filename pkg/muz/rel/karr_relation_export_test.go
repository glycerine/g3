package rel

// Test-only Karr accessors: the tests state matrices and facts as
// math/big oracles; these helpers convert them to and from the relation's
// rationals.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// karrBigMatrix is a KarrMatrix read back as big.Rat values.
type karrBigMatrix struct {
	A  [][]big.Rat
	B  []big.Rat
	Eq []bool
}

func (m karrBigMatrix) Len() int { return len(m.A) }

func karrBig(q *numeral.RationalManager, r *numeral.Rational) (big.Rat, error) {
	var value big.Rat
	text, err := q.String(r)
	if err != nil {
		return value, err
	}
	if _, ok := value.SetString(text); !ok {
		return value, ErrKarrDimension
	}
	return value, nil
}

func karrRational(q *numeral.RationalManager, value *big.Rat) (numeral.Rational, error) {
	r := numeral.NewRational()
	if err := q.SetString(&r, value.RatString()); err != nil {
		return r, errors.Join(err, q.Del(&r))
	}
	return r, nil
}

func karrBigOf(q *numeral.RationalManager, m *KarrMatrix) (karrBigMatrix, error) {
	result := karrBigMatrix{A: make([][]big.Rat, len(m.A)), B: make([]big.Rat, len(m.B)), Eq: append([]bool(nil), m.Eq...)}
	for i := range m.A {
		result.A[i] = make([]big.Rat, len(m.A[i]))
		for j := range m.A[i] {
			v, err := karrBig(q, &m.A[i][j])
			if err != nil {
				return result, err
			}
			result.A[i][j] = v
		}
	}
	for i := range m.B {
		v, err := karrBig(q, &m.B[i])
		if err != nil {
			return result, err
		}
		result.B[i] = v
	}
	return result, nil
}

// Inequalities is get_ineqs read back as big.Rat values.
func (r *KarrRelation) Inequalities() (karrBigMatrix, error) {
	if err := r.getIneqs(); err != nil {
		return karrBigMatrix{}, err
	}
	return karrBigOf(r.q, &r.inequalities)
}

// Basis is get_basis read back as big.Rat values.
func (r *KarrRelation) Basis() (karrBigMatrix, error) {
	if err := r.getBasis(); err != nil {
		return karrBigMatrix{}, err
	}
	return karrBigOf(r.q, &r.basis)
}

// AddFactRat is AddFact for big.Rat values; nil and non-integers are
// skipped as add_fact skips non-integer numerals.
func (r *KarrRelation) AddFactRat(values []*big.Rat) (resultErr error) {
	numbers := make([]*numeral.Rational, len(values))
	storage := make([]numeral.Rational, len(values))
	for i := range storage {
		storage[i] = numeral.NewRational()
	}
	defer func() { resultErr = errors.Join(resultErr, delKarrRationals(r.q, storage)) }()
	for i, value := range values {
		if value == nil || !value.IsInt() {
			continue
		}
		v, err := karrRational(r.q, value)
		if err != nil {
			return err
		}
		storage[i] = v
		numbers[i] = &storage[i]
	}
	return r.AddFact(numbers)
}

// karrPushBig pushes a big.Rat row and constant onto m.
func karrPushBig(q *numeral.RationalManager, m *KarrMatrix, row []big.Rat, constant *big.Rat, equal bool) error {
	values := make([]numeral.Rational, len(row))
	for i := range row {
		v, err := karrRational(q, &row[i])
		if err != nil {
			return errors.Join(err, delKarrRationals(q, values[:i]))
		}
		values[i] = v
	}
	b, err := karrRational(q, constant)
	if err != nil {
		return errors.Join(err, delKarrRationals(q, values))
	}
	m.pushOwned(values, b, equal)
	return nil
}

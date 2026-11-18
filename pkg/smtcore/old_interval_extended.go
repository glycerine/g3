package smtcore

// Source: src/smt/old_interval.cpp:20-158 and src/smt/old_interval.h:25-64.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type smtExtendedRationalKind uint8

const (
	smtExtendedFinite smtExtendedRationalKind = iota
	smtExtendedMinusInfinity
	smtExtendedPlusInfinity
)

// smtExtendedRational is ext_numeral: a kind and a rational m_value of the
// AST manager's rational manager. An infinite value keeps m_value as the
// source leaves it (operations that make a value infinite reset it). Free
// it with del where the source's ext_numeral dies.
type smtExtendedRational struct {
	kind  smtExtendedRationalKind
	value numeral.Rational
}

// smtExtendedZero is ext_numeral(): finite zero.
func smtExtendedZero() smtExtendedRational { return smtExtendedRational{value: smtQZero()} }

// smtFiniteExtended is ext_numeral(rational const & val).
func smtFiniteExtended(q *numeral.RationalManager, value *numeral.Rational) (smtExtendedRational, error) {
	result := smtExtendedZero()
	return result, q.Copy(&result.value, value)
}

// smtInfiniteExtended is ext_numeral(bool plus_infinity).
func smtInfiniteExtended(positive bool) smtExtendedRational {
	result := smtExtendedZero()
	result.kind = smtExtendedMinusInfinity
	if positive {
		result.kind = smtExtendedPlusInfinity
	}
	return result
}

func (n *smtExtendedRational) del(q *numeral.RationalManager) error { return smtQDel(q, &n.value) }

// set is ext_numeral's operator=.
func (n *smtExtendedRational) set(q *numeral.RationalManager, other *smtExtendedRational) error {
	n.kind = other.kind
	return q.Copy(&n.value, &other.value)
}

// clone is ext_numeral's copy constructor.
func (n *smtExtendedRational) clone(q *numeral.RationalManager) (smtExtendedRational, error) {
	result := smtExtendedZero()
	return result, result.set(q, n)
}

func (n *smtExtendedRational) infinite() bool { return n.kind != smtExtendedFinite }

// sign is -1, 0 or 1 (the source's sign() is the is_neg half of it).
func (n *smtExtendedRational) sign() int {
	if n.kind == smtExtendedMinusInfinity {
		return -1
	}
	if n.kind == smtExtendedPlusInfinity {
		return 1
	}
	return n.value.Sign()
}
func (n *smtExtendedRational) zero() bool { return n.kind == smtExtendedFinite && n.value.IsZero() }

func (n *smtExtendedRational) neg(q *numeral.RationalManager) error {
	switch n.kind {
	case smtExtendedMinusInfinity:
		n.kind = smtExtendedPlusInfinity
	case smtExtendedPlusInfinity:
		n.kind = smtExtendedMinusInfinity
	default:
		return q.Neg(&n.value)
	}
	return nil
}

// add is operator+=. Adding opposite infinities is rejected.
func (n *smtExtendedRational) add(q *numeral.RationalManager, other *smtExtendedRational) error {
	if n.infinite() && other.infinite() && n.kind != other.kind {
		return ast.ErrASTDomain
	}
	if n.infinite() {
		return nil
	}
	if other.infinite() {
		n.kind = other.kind
		return q.Reset(&n.value)
	}
	return q.Add(&n.value, &other.value, &n.value)
}

// sub is operator-=. Subtracting an equal infinity is rejected.
func (n *smtExtendedRational) sub(q *numeral.RationalManager, other *smtExtendedRational) error {
	if n.infinite() && other.infinite() && n.kind == other.kind {
		return ast.ErrASTDomain
	}
	if n.infinite() {
		return nil
	}
	if other.infinite() {
		if other.kind == smtExtendedMinusInfinity {
			n.kind = smtExtendedPlusInfinity
		} else {
			n.kind = smtExtendedMinusInfinity
		}
		return q.Reset(&n.value)
	}
	return q.Sub(&n.value, &other.value, &n.value)
}

// mul is operator*=.
func (n *smtExtendedRational) mul(q *numeral.RationalManager, other *smtExtendedRational) error {
	if n.zero() {
		n.kind = smtExtendedFinite
		return nil
	}
	if other.zero() {
		n.kind = smtExtendedFinite
		return q.Reset(&n.value)
	}
	if n.infinite() || other.infinite() {
		if (n.sign() < 0) == (other.sign() < 0) {
			n.kind = smtExtendedPlusInfinity
		} else {
			n.kind = smtExtendedMinusInfinity
		}
		return q.Reset(&n.value)
	}
	return q.Mul(&n.value, &other.value, &n.value)
}

// smtExtendedProduct is operator*(n1, n2): ext_numeral(n1) *= n2. The caller
// frees the result.
func smtExtendedProduct(q *numeral.RationalManager, left, right *smtExtendedRational) (smtExtendedRational, error) {
	result, err := left.clone(q)
	if err == nil {
		err = result.mul(q, right)
	}
	return result, err
}

// inverse is inv(): m_value = rational(1) / m_value.
func (n *smtExtendedRational) inverse(q *numeral.RationalManager) error {
	if n.zero() {
		return ast.ErrASTDomain
	}
	if n.infinite() {
		n.kind = smtExtendedFinite
		return q.Reset(&n.value)
	}
	return q.Inv(&n.value, &n.value)
}

// power is expt(n): m_value = m_value.expt(n).
func (n *smtExtendedRational) power(q *numeral.RationalManager, exponent uint32) error {
	if n.kind == smtExtendedMinusInfinity && exponent%2 == 0 {
		n.kind = smtExtendedPlusInfinity
	}
	if n.infinite() {
		return nil
	}
	return q.Power(&n.value, exponent, &n.value)
}

// cmp orders by operator< and operator==.
func (n *smtExtendedRational) cmp(q *numeral.RationalManager, other *smtExtendedRational) (int, error) {
	if n.kind == other.kind {
		if n.infinite() {
			return 0, nil
		}
		return q.Compare(&n.value, &other.value)
	}
	if n.kind == smtExtendedMinusInfinity || other.kind == smtExtendedPlusInfinity {
		return -1, nil
	}
	return 1, nil
}

func (n *smtExtendedRational) equal(q *numeral.RationalManager, other *smtExtendedRational) (bool, error) {
	c, err := n.cmp(q, other)
	return c == 0, err
}

// String is display.
func (n *smtExtendedRational) String(q *numeral.RationalManager) (string, error) {
	switch n.kind {
	case smtExtendedMinusInfinity:
		return "-oo", nil
	case smtExtendedPlusInfinity:
		return "oo", nil
	default:
		return q.String(&n.value)
	}
}

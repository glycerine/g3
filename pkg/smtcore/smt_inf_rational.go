package smtcore

// Source: src/util/inf_rational.h, the inf_numeral of mi_ext used by the
// dense difference-logic instantiation theory_dense_mi.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtInfRational is inf_rational: m_first + m_second*epsilon, both rationals
// on the AST manager's mpq manager (rational's g_mpq_manager), ordered
// lexicographically. It owns its cells: copy with set, free with del.
type smtInfRational struct {
	first, second numeral.Rational
}

func newSMTInfRational() smtInfRational {
	return smtInfRational{first: smtQZero(), second: smtQZero()}
}

func (v *smtInfRational) del(q *numeral.RationalManager) error {
	err := smtQDel(q, &v.first)
	if e := smtQDel(q, &v.second); err == nil {
		err = e
	}
	return err
}

func (v *smtInfRational) set(q *numeral.RationalManager, other *smtInfRational) error {
	if err := q.Copy(&v.first, &other.first); err != nil {
		return err
	}
	return q.Copy(&v.second, &other.second)
}

// setRational is inf_rational(r): (r, 0).
func (v *smtInfRational) setRational(q *numeral.RationalManager, r *numeral.Rational) error {
	if err := q.Copy(&v.first, r); err != nil {
		return err
	}
	return q.SetInt32(&v.second, 0)
}

// add is out = a + b (operator+ / +=, out may alias a or b).
func smtInfRationalAdd(q *numeral.RationalManager, a, b, out *smtInfRational) error {
	if err := q.Add(&a.first, &b.first, &out.first); err != nil {
		return err
	}
	return q.Add(&a.second, &b.second, &out.second)
}

// sub is out = a - b (operator- / -=, out may alias a or b).
func smtInfRationalSub(q *numeral.RationalManager, a, b, out *smtInfRational) error {
	if err := q.Sub(&a.first, &b.first, &out.first); err != nil {
		return err
	}
	return q.Sub(&a.second, &b.second, &out.second)
}

func (v *smtInfRational) neg(q *numeral.RationalManager) error {
	if err := q.Neg(&v.first); err != nil {
		return err
	}
	return q.Neg(&v.second)
}

// smtInfRationalCmp is the lexicographic order of operator< / operator==.
func smtInfRationalCmp(q *numeral.RationalManager, a, b *smtInfRational) (int, error) {
	c, err := q.Compare(&a.first, &b.first)
	if err != nil || c != 0 {
		return c, err
	}
	return q.Compare(&a.second, &b.second)
}

func (v *smtInfRational) isZero() bool { return v.first.IsZero() && v.second.IsZero() }

// text is inf_rational::to_string (also operator<<).
func (v *smtInfRational) text(q *numeral.RationalManager) (string, error) {
	main, err := q.String(&v.first)
	if err != nil || v.second.IsZero() {
		return main, err
	}
	sign := " +e*"
	if v.second.Sign() < 0 {
		sign = " -e*"
	}
	// abs(m_second).to_string()
	abs := smtQZero()
	defer smtQDel(q, &abs)
	if err = q.Copy(&abs, &v.second); err != nil {
		return "", err
	}
	if abs.Sign() < 0 {
		if err = q.Neg(&abs); err != nil {
			return "", err
		}
	}
	epsilon, err := q.String(&abs)
	if err != nil {
		return "", err
	}
	return "(" + main + sign + epsilon + ")", nil
}

// smtQText is rational::to_string for a trace operand; a manager failure
// prints as the error text, as the trace has no error channel.
func smtQText(q *numeral.RationalManager, r *numeral.Rational) string {
	text, err := q.String(r)
	if err != nil {
		return fmt.Sprint(err)
	}
	return text
}

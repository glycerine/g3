package smtcore

// Source: src/smt/theory_str.cpp lower_bound/upper_bound and their retained
// theory_mi_arith get_lower/get_upper/to_expr calls in theory_arith_core.h.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// stringArithmeticBound reads the concrete mixed-arithmetic bound service.
// A candidate tableau value is never evidence of a bound. The native
// to_expr conversion refuses infinitesimals; the String caller additionally
// refuses nonintegral rational numerals, including bounds on Real terms.
//
// It is lower_bound(e, lo) / upper_bound(e, hi): out is the caller's
// rational, written by is_numeral.
func (t *SMTStringTheory) stringArithmeticBound(expression ast.ASTHandle, lower bool, out *numeral.Rational) (bool, error) {
	family, err := t.state.manager.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	node := t.state.context.enodes.get(expression)
	if port, ok := t.state.Theory(family).(*SMTTheoryArith); ok {
		// theory_str queries theory_mi_arith::get_lower/get_upper.
		if port.integerExt || node == nil {
			return false, nil
		}
		get := port.getUpper
		if lower {
			get = port.getLower
		}
		h, known, err := get(node)
		if err != nil || !known {
			return false, err
		}
		isNumeral, err := port.isNumeralQ(h, out)
		return err == nil && isNumeral && out.IsInt(), err
	}
	return false, nil
}

// discardStringRational runs a reader into a rational local whose value the
// caller does not use, then frees it.
func (t *SMTStringTheory) discardStringRational(read func(*numeral.Rational) error) error {
	r := smtQZero()
	err := read(&r)
	return smtErrJoin(err, smtQDel(t.q(), &r))
}

// withStringRational runs f on rational(n) as a temporary.
func (t *SMTStringTheory) withStringRational(n int64, f func(*numeral.Rational) error) error {
	r := smtQZero()
	err := t.q().SetInt64(&r, n)
	if err == nil {
		err = f(&r)
	}
	return smtErrJoin(err, smtQDel(t.q(), &r))
}

// stringLengths4 is four rational locals (the arrangements' x/y/m/n
// lengths); delStringLengths frees them.
func stringLengths4() [4]numeral.Rational {
	return [4]numeral.Rational{smtQZero(), smtQZero(), smtQZero(), smtQZero()}
}

func (t *SMTStringTheory) delStringLengths(values []numeral.Rational, err *error) {
	for i := range values {
		t.qdel(&values[i], err)
	}
}

// stringTextCut reads a length as a cut position into a literal of size
// limit: it must be a nonnegative integer no larger than limit.
func (t *SMTStringTheory) stringTextCut(r *numeral.Rational, limit int) (int, error) {
	q := t.q()
	if r.IsNeg() || !r.IsInt() {
		return 0, ast.ErrASTDomain
	}
	fits, err := q.IsUint64(r)
	if err != nil {
		return 0, err
	}
	if !fits {
		return 0, ast.ErrASTDomain
	}
	v, err := q.Uint64(r)
	if err != nil {
		return 0, err
	}
	if v > uint64(limit) {
		return 0, ast.ErrASTDomain
	}
	return int(v), nil
}

// withStringDifference runs f on the rational temporary a - b.
func (t *SMTStringTheory) withStringDifference(a, b *numeral.Rational, f func(*numeral.Rational) error) error {
	d := smtQZero()
	err := t.q().Sub(a, b, &d)
	if err == nil {
		err = f(&d)
	}
	return smtErrJoin(err, smtQDel(t.q(), &d))
}

package qe

// Source: src/qe/qe_arith_plugin.cpp bounds_proc::div_z (m_d = lcm(...)).

import (
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// cooperLCM is div_z's m_d = rational(1); m_d = lcm(divisor, m_d) over the
// divisors, written into out (a rational of q).
func cooperLCM(q *unumeral.RationalManager, values []*ArithmeticDivisibility, out *unumeral.Rational) error {
	if err := q.SetInt32(out, 1); err != nil {
		return err
	}
	for _, v := range values {
		if v == nil {
			return ast.ErrASTDomain
		}
		isInt, err := q.IsInt(&v.Divisor)
		if err != nil {
			return err
		}
		zero, err := q.IsZero(&v.Divisor)
		if err != nil {
			return err
		}
		if !isInt || zero {
			return ast.ErrASTDomain
		}
		if err = q.LCM(&v.Divisor, out, out); err != nil {
			return err
		}
	}
	return nil
}

// ArithmeticCooperDivisor is the lcm of all of processor's divisors, written
// into out (a rational of the manager's NumeralService().Manager).
func ArithmeticCooperDivisor(processor *ArithmeticBoundsProcessor, out *unumeral.Rational) (bool, error) {
	if processor == nil {
		return false, ast.ErrASTHandle
	}
	values := append(processor.Divisibilities(false), processor.Divisibilities(true)...)
	if len(values) == 0 {
		return false, nil
	}
	err := cooperLCM(processor.util.q(), values, out)
	return err == nil, err
}

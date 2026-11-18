package qe

// Source: src/smt/arith_eq_solver.cpp gcd_normalize, find_abs_min, gcd_test
// and solve_integer_equation; src/util/rational.h mod_hat.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// arithmeticEquationGCD folds g = gcd(abs(values[i]), g) over the nonzero
// values (from start) until g is one, as gcd_normalize and gcd_test do.
func arithmeticEquationGCD(q *unumeral.RationalManager, values []unumeral.Rational, start int, g *unumeral.Rational) (resultErr error) {
	value := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&value)) }()
	if err := q.SetInt32(g, 0); err != nil {
		return err
	}
	for i := start; i < len(values); i++ {
		if one, err := q.IsOne(g); err != nil || one {
			return err
		}
		zero, err := q.IsZero(&values[i])
		if err != nil {
			return err
		}
		if zero {
			continue
		}
		if err = errors.Join(q.Copy(&value, &values[i]), q.Abs(&value)); err != nil {
			return err
		}
		if gZero, err := q.IsZero(g); err != nil {
			return err
		} else if gZero {
			err = q.Copy(g, &value)
		} else {
			err = q.GCD(&value, g, g)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// arithmeticEquationGCDNormalize is arith_eq_solver::gcd_normalize.
func arithmeticEquationGCDNormalize(q *unumeral.RationalManager, values []unumeral.Rational) (resultErr error) {
	g := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&g)) }()
	if err := arithmeticEquationGCD(q, values, 0, &g); err != nil {
		return err
	}
	zero, err := q.IsZero(&g)
	if err != nil {
		return err
	}
	one, err := q.IsOne(&g)
	if err != nil || zero || one {
		return err
	}
	for i := range values {
		if err = q.Div(&values[i], &g, &values[i]); err != nil {
			return err
		}
	}
	return nil
}

// arithmeticEquationFindAbsMin is arith_eq_solver::find_abs_min.
func arithmeticEquationFindAbsMin(q *unumeral.RationalManager, values []unumeral.Rational) (index int, resultErr error) {
	v, w := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&v), q.Del(&w)) }()
	for i := 1; i < len(values); i++ {
		if err := errors.Join(q.Copy(&w, &values[i]), q.Abs(&w)); err != nil {
			return 0, err
		}
		vZero, err := q.IsZero(&v)
		if err != nil {
			return 0, err
		}
		take := vZero
		if !take {
			wZero, err := q.IsZero(&w)
			if err != nil {
				return 0, err
			}
			c, err := q.Compare(&w, &v)
			if err != nil {
				return 0, err
			}
			take = !wZero && c < 0
		}
		if take {
			index = i
			if err = q.Copy(&v, &w); err != nil {
				return 0, err
			}
		}
	}
	return index, nil
}

// arithmeticEquationGCDTest is arith_eq_solver::gcd_test.
func arithmeticEquationGCDTest(q *unumeral.RationalManager, values []unumeral.Rational) (_ bool, resultErr error) {
	g, r := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&g), q.Del(&r)) }()
	if err := arithmeticEquationGCD(q, values, 1, &g); err != nil {
		return false, err
	}
	if one, err := q.IsOne(&g); err != nil || one {
		return one, err
	}
	if zero, err := q.IsZero(&g); err != nil || zero {
		if err != nil {
			return false, err
		}
		return q.IsZero(&values[0])
	}
	if err := q.Div(&values[0], &g, &r); err != nil {
		return false, err
	}
	return q.IsInt(&r)
}

// arithmeticEquationModHat is mod_hat(a, b) into out.
func arithmeticEquationModHat(q *unumeral.RationalManager, a, b, out *unumeral.Rational) (resultErr error) {
	r2 := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&r2)) }()
	if err := q.Mod(a, b, out); err != nil {
		return err
	}
	if err := q.Add(out, out, &r2); err != nil {
		return err
	}
	if c, err := q.Compare(b, &r2); err != nil {
		return err
	} else if c < 0 {
		return q.Sub(out, b, out)
	}
	return nil
}

// solveArithmeticIntegerRow is arith_eq_solver::solve_integer_equation on
// values (rationals of q); satisfiable is its result.
func solveArithmeticIntegerRow(q *unumeral.RationalManager, values []unumeral.Rational) (index int, fresh, satisfiable bool, resultErr error) {
	if len(values) < 2 {
		return 0, false, false, ast.ErrASTDomain
	}
	if err := arithmeticEquationGCDNormalize(q, values); err != nil {
		return 0, false, false, err
	}
	if ok, err := arithmeticEquationGCDTest(q, values); err != nil || !ok {
		return 0, false, false, err
	}
	index, err := arithmeticEquationFindAbsMin(q, values)
	if err != nil {
		return 0, false, false, err
	}
	// numeral a = values[index]; numeral abs_a = abs(a);
	a, absA, m := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&a), q.Del(&absA), q.Del(&m)) }()
	if err = errors.Join(q.Copy(&a, &values[index]), q.Copy(&absA, &a), q.Abs(&absA)); err != nil {
		return 0, false, false, err
	}
	if zero, err := q.IsZero(&absA); err != nil || zero {
		// The equation is trivial.
		return index, false, err == nil, err
	}
	negateAll := func() error {
		for i := range values {
			if err := q.Neg(&values[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if one, err := q.IsOne(&a); err != nil {
		return 0, false, false, err
	} else if one {
		if err = negateAll(); err != nil {
			return 0, false, false, err
		}
	}
	one, err := q.IsOne(&absA)
	if err != nil {
		return 0, false, false, err
	}
	fresh = !one
	if !fresh {
		return index, false, true, nil
	}
	// numeral m = abs_a + numeral(1);
	if err = errors.Join(q.Copy(&m, &absA), q.Inc(&m)); err != nil {
		return 0, false, false, err
	}
	for i := range values {
		if err = arithmeticEquationModHat(q, &values[i], &m, &values[i]); err != nil {
			return 0, false, false, err
		}
	}
	if one, err := q.IsOne(&values[index]); err != nil {
		return 0, false, false, err
	} else if one {
		if err = negateAll(); err != nil {
			return 0, false, false, err
		}
	}
	if minusOne, err := q.IsMinusOne(&values[index]); err != nil || !minusOne {
		return 0, false, false, errors.Join(err, ast.ErrASTDomain)
	}
	if err = q.Copy(&values[index], &m); err != nil {
		return 0, false, false, err
	}
	return index, true, true, nil
}

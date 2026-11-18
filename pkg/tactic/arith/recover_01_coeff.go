package arith

// Source: recover_01_tactic.cpp find_zero_cls, find_coeff and process checks.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// recover01Coefficients validates the complete truth table before a caller
// constructs substitutions. Clause and literal handles are borrowed.
// The coefficients are rationals of the manager's NumeralService().Manager
// owned by the caller (recover01Release).
func recover01Coefficients(m *ast.ASTManager, clauses []ast.ASTHandle) (_ []ast.ASTHandle, _ []unumeral.Rational, _ bool, resultErr error) {
	if len(clauses) == 0 {
		return nil, nil, false, nil
	}
	n, e := m.GetNumArgs(clauses[len(clauses)-1])
	if e != nil {
		return nil, nil, false, e
	}
	if n < 2 || n > 31 {
		return nil, nil, false, nil
	}
	expected := uint64(1) << (n - 1)
	if uint64(len(clauses)) < expected {
		return nil, nil, false, nil
	}
	var zero ast.ASTHandle
	for _, c := range clauses {
		for i := uint32(0); i < n; i++ {
			lit, e := m.GetArg(c, i)
			if e != nil {
				return nil, nil, false, e
			}
			eq, e := arithmeticBasicApp(m, lit, ast.ASTOpEq)
			if e != nil {
				return nil, nil, false, e
			}
			if !eq {
				continue
			}
			a, e := m.GetArg(lit, 0)
			if e != nil {
				return nil, nil, false, e
			}
			b, e := m.GetArg(lit, 1)
			if e != nil {
				return nil, nil, false, e
			}
			for j := 0; j < 2; j++ {
				constant, e := arithmeticUninterpretedConstant(m, a)
				if e != nil {
					return nil, nil, false, e
				}
				isZero, e := m.ArithmeticNumeralIsZero(b)
				if e != nil {
					return nil, nil, false, e
				}
				if constant && isZero {
					zero = c
					break
				}
				a, b = b, a
			}
			if zero != (ast.ASTHandle{}) {
				break
			}
		}
		if zero != (ast.ASTHandle{}) {
			break
		}
	}
	if zero == (ast.ASTHandle{}) {
		return nil, nil, false, nil
	}
	var literals []ast.ASTHandle
	for i := uint32(0); i < n; i++ {
		lit, e := m.GetArg(zero, i)
		if e != nil {
			return nil, nil, false, e
		}
		eq, e := arithmeticBasicApp(m, lit, ast.ASTOpEq)
		if e != nil {
			return nil, nil, false, e
		}
		if !eq {
			literals = append(literals, lit)
		}
	}
	if len(literals) != int(n-1) {
		return nil, nil, false, nil
	}
	// vector<rational> values; rational value, sum;
	q := m.NumeralService().Manager
	values := make([]unumeral.Rational, int(expected))
	for i := range values {
		values[i] = unumeral.NewRational()
	}
	value, sum := unumeral.NewRational(), unumeral.NewRational()
	defer func() {
		resultErr = errors.Join(resultErr, recover01Release(q, values), q.Del(&value), q.Del(&sum))
	}()
	found := make([]bool, int(expected))
	for _, c := range clauses {
		size, e := m.GetNumArgs(c)
		if e != nil {
			return nil, nil, false, e
		}
		if size != n {
			return nil, nil, false, nil
		}
		idx := 0
		for bit, lit := range literals {
			matched := false
			for j := uint32(0); j < n; j++ {
				other, e := m.GetArg(c, j)
				if e != nil {
					return nil, nil, false, e
				}
				if other == lit {
					matched = true
					break
				}
				not, arg, e := m.IsNot(other)
				if e != nil {
					return nil, nil, false, e
				}
				if not && arg == lit {
					idx |= 1 << bit
					matched = true
					break
				}
				not, arg, e = m.IsNot(lit)
				if e != nil {
					return nil, nil, false, e
				}
				if not && arg == other {
					idx |= 1 << bit
					matched = true
					break
				}
			}
			if !matched {
				return nil, nil, false, nil
			}
		}
		hasValue := false
		for j := uint32(0); j < n && !hasValue; j++ {
			lit, e := m.GetArg(c, j)
			if e != nil {
				return nil, nil, false, e
			}
			eq, e := arithmeticBasicApp(m, lit, ast.ASTOpEq)
			if e != nil {
				return nil, nil, false, e
			}
			if !eq {
				continue
			}
			for k := uint32(0); k < 2; k++ {
				a, e := m.GetArg(lit, k)
				if e != nil {
					return nil, nil, false, e
				}
				_, ok, e := m.ArithmeticNumeralRational(a, &value)
				if e != nil {
					return nil, nil, false, e
				}
				if ok {
					hasValue = true
					break
				}
			}
		}
		if !hasValue {
			return nil, nil, false, nil
		}
		if found[idx] {
			if equal, e := q.Equal(&values[idx], &value); e != nil || !equal {
				return nil, nil, false, e
			}
		}
		found[idx] = true
		if e := q.Copy(&values[idx], &value); e != nil {
			return nil, nil, false, e
		}
	}
	for idx := range values {
		if !found[idx] {
			return nil, nil, false, nil
		}
		if e := q.SetInt32(&sum, 0); e != nil {
			return nil, nil, false, e
		}
		for bit := range literals {
			if idx&(1<<bit) != 0 {
				if e := q.Add(&sum, &values[1<<bit], &sum); e != nil {
					return nil, nil, false, e
				}
			}
		}
		if equal, e := q.Equal(&sum, &values[idx]); e != nil || !equal {
			return nil, nil, false, e
		}
	}
	coefficients := make([]unumeral.Rational, len(literals))
	for bit := range literals {
		coefficients[bit] = unumeral.NewRational()
		if e := q.Copy(&coefficients[bit], &values[1<<bit]); e != nil {
			return nil, nil, false, errors.Join(e, recover01Release(q, coefficients))
		}
	}
	return literals, coefficients, true, nil
}

// recover01Release destroys a coefficient vector.
func recover01Release(q *unumeral.RationalManager, values []unumeral.Rational) (err error) {
	for i := range values {
		err = errors.Join(err, q.Del(&values[i]))
	}
	return err
}

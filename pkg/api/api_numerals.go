package api

// Source: src/api/api_numeral.cpp:50-194,265-403. Arithmetic, bit-vector,
// finite-domain and floating-point numeral construction is implemented here.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

// errAPINumeralParse is Z3_PARSER_ERROR from Z3_mk_numeral's character
// filter.
var errAPINumeralParse = errors.New("api: numeral text contains a character outside the numeral alphabet")

// apiIsNumeralSort is is_numeral_sort: arith, bv, datalog_relation and fpa
// family sorts.
func apiIsNumeralSort(m *ast.ASTManager, sort ast.ASTHandle) (bool, error) {
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	name, err := apiRegisteredFamilyName(m, family)
	if err != nil {
		return false, err
	}
	switch name {
	case "arith", "bv", "datalog_relation", "fpa":
		return true, nil
	}
	return false, nil
}

func (c *SMTContext) Numeral(value string, sort ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.Manager()
	if err != nil {
		return result, err
	}
	// check_numeral_sort runs first: Z3_INVALID_ARG.
	if numeral, e := apiIsNumeralSort(m, sort); e != nil {
		return result, e
	} else if !numeral {
		return result, ast.ErrASTDomain
	}
	// The character filter runs for every sort; p, P and + are accepted only
	// for floating-point sorts (fpa_util::is_float, not rounding modes).
	_, _, isFloat, e := m.FloatingPointSortShape(sort)
	if e != nil {
		return result, e
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if !(('0' <= ch && ch <= '9') ||
			ch == '/' || ch == '-' ||
			ch == ' ' || ch == '\n' ||
			ch == '.' || ch == 'e' ||
			ch == 'E' ||
			(isFloat &&
				(ch == 'p' ||
					ch == 'P' ||
					ch == '+'))) {
			return result, errAPINumeralParse
		}
	}
	if yes, e := m.IsBitVectorSort(sort); e == nil && yes {
		bits, e := m.BitVectorSize(sort)
		if e != nil {
			return result, e
		}
		q := m.NumeralService()
		n, e := q.NewValue()
		if e != nil {
			return result, e
		}
		defer func() { err = errors.Join(err, q.Del(&n)) }()
		if e = q.SetString(&n, value); e != nil {
			return result, e
		}
		return m.BitVectorNumeral(&n, bits)
	} else if e != nil {
		return result, e
	}
	if _, e := m.RegisterArithmeticPlugin(); e != nil {
		return result, e
	}
	arith, e := m.ArithmeticFamilyID()
	if e != nil {
		return result, e
	}
	family, e := m.GetFamilyID(sort)
	if e != nil {
		return result, e
	}
	if family == arith {
		name, e := m.GetASTName(sort)
		if e != nil {
			return result, e
		}
		text, e := m.Symbols.String(name)
		if e != nil {
			return result, e
		}
		if text == "Int" {
			return m.ArithmeticIntegerNumeralText(value)
		}
		if text == "Real" {
			return m.ArithmeticRealNumeralText(value)
		}
		return result, ast.ErrASTDomain
	}
	if finite, e := m.IsDatalogFiniteSort(sort); e == nil && finite {
		// rational r(value); r.is_uint64(); r.get_uint64()
		var v uint64
		e = withRationals(m, 1, func(q *numeral.RationalManager, rs []numeral.Rational) error {
			if q.SetString(&rs[0], value) != nil {
				return ast.ErrASTDomain
			}
			ok, err := q.IsUint64(&rs[0])
			if err == nil && !ok {
				err = ast.ErrASTDomain
			}
			if err == nil {
				v, err = q.Uint64(&rs[0])
			}
			return err
		})
		if e != nil {
			return result, e
		}
		return m.MkDatalogNumeral(v, sort)
	} else if e != nil {
		return result, e
	}
	floatingPoint, floatingPointFamily, e := c.floatingPointManager()
	if e != nil {
		return result, e
	}
	if family == floatingPointFamily {
		return floatingPoint.MkFPANumeralString(value, sort)
	}
	return result, ast.ErrASTDomain
}
func (c *SMTContext) IntNumeral(value int64, sort ast.ASTHandle) (ast.ASTHandle, error) {
	return c.Numeral(strconv.FormatInt(value, 10), sort)
}
func (c *SMTContext) UintNumeral(value uint64, sort ast.ASTHandle) (ast.ASTHandle, error) {
	return c.Numeral(strconv.FormatUint(value, 10), sort)
}
func (c *SMTContext) NumeralString(value ast.ASTHandle) (string, error) {
	m, e := c.Manager()
	if e != nil {
		return "", e
	}
	if text, _, yes, e := m.ArithmeticNumeralText(value); e != nil || yes {
		return text, e
	}
	if text, yes, e := m.BitVectorNumeralText(value); e != nil || yes {
		return text, e
	}
	if text, yes, e := m.DatalogNumeralText(value); e != nil || yes {
		return text, e
	}
	if text, yes, e := m.FPANumeralString(value); e != nil || yes {
		return text, e
	}
	return "", errAPINumeralArgument
}

// IsNumeral is Z3_is_numeral_ast: the arith, bit-vector, floating-point and
// rounding-mode numeral recognizers, or dl_decl_util::is_numeral_ext, which
// also accepts true and false. NumeralString (Z3_get_numeral_string) covers
// all but the Boolean values.
func (c *SMTContext) IsNumeral(value ast.ASTHandle) (bool, error) {
	_, e := c.NumeralString(value)
	if errors.Is(e, errAPINumeralArgument) {
		m, err := c.Manager()
		if err != nil {
			return false, err
		}
		return m.IsTrue(value) || m.IsFalse(value), nil
	}
	return e == nil, e
}

// NumeralRational is Z3_get_numeral_rational: it writes value's rational
// into out, a rational of the AST manager's NumeralService().Manager.
func (c *SMTContext) NumeralRational(value ast.ASTHandle, out *numeral.Rational) (bool, error) {
	m, e := c.Manager()
	if e != nil {
		return false, e
	}
	var yes bool
	if _, yes, e = m.ArithmeticNumeralRational(value, out); e != nil || yes {
		return yes, e
	}
	if _, yes, e = m.BitVectorNumeralRational(value, out); e != nil || yes {
		return yes, e
	}
	v, yes, e := m.DatalogNumeralValue(value)
	if e != nil || !yes {
		return false, e
	}
	return true, m.NumeralService().Manager.SetUint64(out, v)
}

// numeralQuery runs body on Z3_get_numeral_rational's rational r.
func (c *SMTContext) numeralQuery(value ast.ASTHandle, body func(q *numeral.RationalManager, rs []numeral.Rational) error) (bool, error) {
	m, e := c.Manager()
	if e != nil {
		return false, e
	}
	var yes bool
	e = withRationals(m, 2, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		var err error
		if yes, err = c.NumeralRational(value, &rs[0]); err != nil || !yes {
			return err
		}
		return body(q, rs)
	})
	return yes, e
}
func (c *SMTContext) NumeralInt64(value ast.ASTHandle) (result int64, matched bool, err error) {
	yes, err := c.numeralQuery(value, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		ok, e := q.IsInt64(&rs[0])
		if e == nil && ok {
			result, e = q.Int64(&rs[0])
			matched = e == nil
		}
		return e
	})
	return result, yes && matched, err
}
func (c *SMTContext) NumeralUint64(value ast.ASTHandle) (result uint64, matched bool, err error) {
	yes, err := c.numeralQuery(value, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		ok, e := q.IsUint64(&rs[0])
		if e == nil && ok {
			result, e = q.Uint64(&rs[0])
			matched = e == nil
		}
		return e
	})
	return result, yes && matched, err
}

// NumeralRationalInt64 is Z3_get_numeral_rational_int64: numerator(r) and
// denominator(r) when both are int64.
func (c *SMTContext) NumeralRationalInt64(value ast.ASTHandle) (num, den int64, matched bool, err error) {
	yes, err := c.numeralQuery(value, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		if e := q.DenominatorRational(&rs[0], &rs[1]); e != nil {
			return e
		}
		if e := q.NumeratorRational(&rs[0], &rs[0]); e != nil {
			return e
		}
		okN, e := q.IsInt64(&rs[0])
		if e != nil {
			return e
		}
		okD, e := q.IsInt64(&rs[1])
		if e != nil || !okN || !okD {
			return e
		}
		if num, e = q.Int64(&rs[0]); e != nil {
			return e
		}
		den, e = q.Int64(&rs[1])
		matched = e == nil
		return e
	})
	return num, den, yes && matched, err
}
func (c *SMTContext) BitVectorNumeral(bits []bool) (ast.ASTHandle, error) {
	if len(bits) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	// rational r(0); rational p(1); r += p for each set bit; p *= 2
	var result ast.ASTHandle
	e = withRationals(m, 3, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		r, p, two := &rs[0], &rs[1], &rs[2]
		if err := errors.Join(q.SetInt32(p, 1), q.SetInt32(two, 2)); err != nil {
			return err
		}
		for _, bit := range bits {
			if bit {
				if err := q.Add(r, p, r); err != nil {
					return err
				}
			}
			if err := q.Mul(p, two, p); err != nil {
				return err
			}
		}
		var err error
		result, err = m.BitVectorNumeralFromRational(r, uint32(len(bits)))
		return err
	})
	return result, e
}

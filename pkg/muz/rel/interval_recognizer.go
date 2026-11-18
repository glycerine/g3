package rel

// Source: src/muz/rel/dl_interval_relation.cpp is_linear/is_le/is_lt/is_eq.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// intervalConstraint is is_le/is_lt/is_eq's out-parameters x, y, k (a
// rational of the AST manager's rational manager, which the caller frees)
// and is_int.
type intervalConstraint struct {
	positive, negative uint32
	constant           numeral.Rational
	integer            bool
}

// newIntervalConstraint is the k.reset(), x = y = UINT_MAX prologue.
func newIntervalConstraint() intervalConstraint {
	return intervalConstraint{positive: math.MaxUint32, negative: math.MaxUint32, constant: numeral.NewRational()}
}
func (c *intervalConstraint) hasVariable() bool {
	return c.positive != math.MaxUint32 || c.negative != math.MaxUint32
}

// intervalLinear is interval_relation_plugin::is_linear.
func intervalLinear(m *ast.ASTManager, q *numeral.RationalManager, family ast.FamilyID, h ast.ASTHandle, c *intervalConstraint, positive bool) (bool, error) {
	k, e := m.GetKind(h)
	if e != nil {
		return false, e
	}
	if k == ast.ASTVar {
		index, e := m.GetVarIndex(h)
		if e != nil {
			return false, e
		}
		if positive && c.positive == math.MaxUint32 {
			c.positive = index
			return true, nil
		}
		if !positive && c.negative == math.MaxUint32 {
			c.negative = index
			return true, nil
		}
		return false, nil
	}
	if k != ast.ASTApp {
		return false, nil
	}
	fid, e := m.GetFamilyID(h)
	if e != nil {
		return false, e
	}
	if fid != family {
		return false, nil
	}
	op, e := m.GetDeclKind(h)
	if e != nil {
		return false, e
	}
	n, e := m.GetNumArgs(h)
	if e != nil {
		return false, e
	}
	if op == 6 {
		for i := uint32(0); i < n; i++ {
			a, e := m.GetArg(h, i)
			if e != nil {
				return false, e
			}
			ok, e := intervalLinear(m, q, family, a, c, positive)
			if e != nil || !ok {
				return ok, e
			}
		}
		return true, nil
	}
	if op == 7 && n == 2 {
		a, e := m.GetArg(h, 0)
		if e != nil {
			return false, e
		}
		b, e := m.GetArg(h, 1)
		if e != nil {
			return false, e
		}
		ok, e := intervalLinear(m, q, family, a, c, positive)
		if e != nil || !ok {
			return ok, e
		}
		return intervalLinear(m, q, family, b, c, !positive)
	}
	if op == 9 && n == 2 {
		a, e := m.GetArg(h, 0)
		if e != nil {
			return false, e
		}
		b, e := m.GetArg(h, 1)
		if e != nil {
			return false, e
		}
		// rational k1; is_numeral(a->get_arg(0), k1) && k1.is_minus_one()
		k1 := numeral.NewRational()
		_, isNumeral, e := m.ArithmeticNumeralRational(a, &k1)
		minusOne := false
		if e == nil && isNumeral {
			minusOne, e = q.IsMinusOne(&k1)
		}
		if e = errors.Join(e, q.Del(&k1)); e != nil {
			return false, e
		}
		if minusOne {
			kind, e := m.GetKind(b)
			if e != nil {
				return false, e
			}
			// SET_VAR uses the incoming polarity in the source's -1*x branch.
			if kind == ast.ASTVar {
				return intervalLinear(m, q, family, b, c, positive)
			}
		}
	}
	k1 := numeral.NewRational()
	_, isNumeral, e := m.ArithmeticNumeralRational(h, &k1)
	if e == nil && isNumeral {
		if positive {
			e = q.Add(&c.constant, &k1, &c.constant)
		} else {
			e = q.Sub(&c.constant, &k1, &c.constant)
		}
	}
	if e = errors.Join(e, q.Del(&k1)); e != nil {
		return false, e
	}
	return isNumeral, nil
}

func intervalSortKind(m *ast.ASTManager, family ast.FamilyID, h ast.ASTHandle) (int32, error) {
	sort, e := m.GetSort(h)
	if e != nil {
		return -1, e
	}
	fid, e := m.GetFamilyID(sort)
	if e != nil {
		return -1, e
	}
	if fid != family {
		return -1, nil
	}
	return m.GetDeclKind(sort)
}

// intervalComparison is filter_interpreted's recognizer cascade: it
// returns 0 for is_le (non-strict), 1 for is_lt (strict real) and 2 for
// is_eq. The returned constraint's constant is always initialized; the
// caller frees it.
func intervalComparison(m *ast.ASTManager, q *numeral.RationalManager, h ast.ASTHandle) (int, intervalConstraint, bool, error) {
	family, e := m.RegisterArithmeticPlugin()
	if e != nil {
		return 0, newIntervalConstraint(), false, e
	}
	c, ok, e := intervalLE(m, q, family, h)
	if e != nil || ok {
		return 0, c, ok, e
	}
	if e = q.Del(&c.constant); e != nil {
		return 0, newIntervalConstraint(), false, e
	}
	c, ok, e = intervalLT(m, q, family, h)
	if e != nil || ok {
		return 1, c, ok, e
	}
	if e = q.Del(&c.constant); e != nil {
		return 0, newIntervalConstraint(), false, e
	}
	c, ok, e = intervalEQ(m, q, family, h)
	return 2, c, ok, e
}

func intervalParts(m *ast.ASTManager, h ast.ASTHandle) (ast.FamilyID, int32, ast.ASTHandle, ast.ASTHandle, bool, error) {
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	n, e := m.GetNumArgs(h)
	if e != nil || n != 2 {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	fid, e := m.GetFamilyID(h)
	if e != nil {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	op, e := m.GetDeclKind(h)
	if e != nil {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	a, e := m.GetArg(h, 0)
	if e != nil {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	b, e := m.GetArg(h, 1)
	return fid, op, a, b, e == nil, e
}
func intervalPair(m *ast.ASTManager, q *numeral.RationalManager, family ast.FamilyID, a, b ast.ASTHandle, leftPositive bool, c *intervalConstraint) (bool, error) {
	ok, e := intervalLinear(m, q, family, a, c, leftPositive)
	if e != nil || !ok {
		return ok, e
	}
	ok, e = intervalLinear(m, q, family, b, c, !leftPositive)
	if e != nil || !ok {
		return ok, e
	}
	return c.hasVariable(), nil
}

// intervalLE is is_le: 0 <= x - y + k.
func intervalLE(m *ast.ASTManager, q *numeral.RationalManager, family ast.FamilyID, h ast.ASTHandle) (intervalConstraint, bool, error) {
	c := newIntervalConstraint()
	fid, op, a, b, app, e := intervalParts(m, h)
	if e != nil {
		return c, false, e
	}
	one := numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}
	if app && fid == family && op >= 2 && op <= 5 {
		kind, e := intervalSortKind(m, family, a)
		if e != nil {
			return c, false, e
		}
		c.integer = kind == 1
		if op >= 4 && !c.integer {
			return c, false, nil
		}
		leftPositive := op == 3 || op == 5
		ok, e := intervalPair(m, q, family, a, b, leftPositive, &c)
		if e != nil || !ok {
			return c, ok, e
		}
		if op == 4 {
			// k -= rational::one()
			e = q.Sub(&c.constant, &one, &c.constant)
		}
		if op == 5 {
			// k += rational::one()
			e = q.Add(&c.constant, &one, &c.constant)
		}
		return c, e == nil, e
	}
	not, arg, e := m.IsNot(h)
	if e != nil || !not {
		return c, false, e
	}
	if e = q.Del(&c.constant); e != nil {
		return newIntervalConstraint(), false, e
	}
	c, ok, e := intervalLE(m, q, family, arg)
	if e != nil {
		return c, false, e
	}
	if ok && c.integer {
		// k.neg(); k -= rational::one(); std::swap(x, y)
		e = q.Neg(&c.constant)
		if e == nil {
			e = q.Sub(&c.constant, &one, &c.constant)
		}
		c.positive, c.negative = c.negative, c.positive
		return c, e == nil, e
	}
	if e = q.Del(&c.constant); e != nil {
		return newIntervalConstraint(), false, e
	}
	c, ok, e = intervalLT(m, q, family, arg)
	if e != nil || !ok {
		return c, ok, e
	}
	// is_int = false; k.neg(); std::swap(x, y)
	c.integer = false
	e = q.Neg(&c.constant)
	c.positive, c.negative = c.negative, c.positive
	return c, e == nil, e
}

// intervalLT is is_lt: 0 < x - y + k.
func intervalLT(m *ast.ASTManager, q *numeral.RationalManager, family ast.FamilyID, h ast.ASTHandle) (intervalConstraint, bool, error) {
	c := newIntervalConstraint()
	fid, op, a, b, app, e := intervalParts(m, h)
	if e != nil || !app || fid != family || (op != 4 && op != 5) {
		return c, false, e
	}
	kind, e := intervalSortKind(m, family, a)
	if e != nil || kind != 0 {
		return c, false, e
	}
	ok, e := intervalPair(m, q, family, a, b, op == 5, &c)
	return c, ok, e
}

// intervalEQ is is_eq: 0 = x - y + k.
func intervalEQ(m *ast.ASTManager, q *numeral.RationalManager, family ast.FamilyID, h ast.ASTHandle) (intervalConstraint, bool, error) {
	c := newIntervalConstraint()
	fid, op, a, b, app, e := intervalParts(m, h)
	if e != nil || !app || fid != m.BasicFamilyID() || op != ast.ASTOpEq {
		return c, false, e
	}
	ok, e := intervalPair(m, q, family, a, b, false, &c)
	return c, ok, e
}

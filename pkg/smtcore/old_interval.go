package smtcore

// Source: src/smt/old_interval.cpp and src/smt/old_interval.h retained
// rational interval operations used by nonlinear arithmetic.
//
// An smtOldInterval is old_interval: its endpoints are ext_numerals over
// rationals of the AST manager's rational manager q. The source's intervals
// are values; here each one is freed with del where the source's interval
// dies (a local at scope exit, a temporary after its full expression).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type smtOldInterval struct {
	manager              *smtArithmeticBoundDependencyManager
	q                    *numeral.RationalManager
	lower, upper         smtExtendedRational
	lowerOpen, upperOpen bool
	lowerDep, upperDep   *smtArithmeticBoundDependency
}

// newSMTOldInterval is interval(m): (-oo, oo).
func newSMTOldInterval(manager *smtArithmeticBoundDependencyManager, q *numeral.RationalManager) (*smtOldInterval, error) {
	if manager == nil || q == nil {
		return nil, ast.ErrASTDomain
	}
	return &smtOldInterval{manager: manager, q: q, lower: smtInfiniteExtended(false), upper: smtInfiniteExtended(true), lowerOpen: true, upperOpen: true}, nil
}

// newSMTOldIntervalBounds is interval(m, lower, l_open, l_dep, upper, u_open,
// u_dep); the source's SASSERT(lower <= upper) is kept as a domain check.
func newSMTOldIntervalBounds(manager *smtArithmeticBoundDependencyManager, q *numeral.RationalManager, lower *numeral.Rational, lowerOpen bool, lowerDep *smtArithmeticBoundDependency, upper *numeral.Rational, upperOpen bool, upperDep *smtArithmeticBoundDependency) (*smtOldInterval, error) {
	if manager == nil || q == nil || lower == nil || upper == nil {
		return nil, ast.ErrASTDomain
	}
	c, err := q.Compare(lower, upper)
	if err != nil {
		return nil, err
	}
	if c > 0 || c == 0 && lowerOpen && upperOpen {
		return nil, ast.ErrASTDomain
	}
	result := &smtOldInterval{manager: manager, q: q, lower: smtExtendedZero(), upper: smtExtendedZero(), lowerOpen: lowerOpen, upperOpen: upperOpen, lowerDep: lowerDep, upperDep: upperDep}
	if err = q.Copy(&result.lower.value, lower); err == nil {
		err = q.Copy(&result.upper.value, upper)
	}
	if err != nil {
		return nil, errors.Join(err, result.del())
	}
	return result, nil
}

// newSMTOldIntervalPoint is interval(m, val, l_dep, u_dep): [val, val].
func newSMTOldIntervalPoint(manager *smtArithmeticBoundDependencyManager, q *numeral.RationalManager, value *numeral.Rational, lowerDep, upperDep *smtArithmeticBoundDependency) (*smtOldInterval, error) {
	return newSMTOldIntervalBounds(manager, q, value, false, lowerDep, value, false, upperDep)
}

// newSMTOldIntervalInt is interval(m, rational(k)).
func newSMTOldIntervalInt(manager *smtArithmeticBoundDependencyManager, q *numeral.RationalManager, k int32) (*smtOldInterval, error) {
	r := smtQInt(k)
	return newSMTOldIntervalPoint(manager, q, &r, nil, nil)
}

// newSMTOldIntervalHalfBound is interval(m, val, open, lower, d).
func newSMTOldIntervalHalfBound(manager *smtArithmeticBoundDependencyManager, q *numeral.RationalManager, value *numeral.Rational, open, lower bool, dependency *smtArithmeticBoundDependency) (*smtOldInterval, error) {
	result, err := newSMTOldInterval(manager, q)
	if err != nil || value == nil {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	if lower {
		result.lowerOpen, result.lowerDep = open, dependency
		result.lower.kind = smtExtendedFinite
		err = q.Copy(&result.lower.value, value)
	} else {
		result.upperOpen, result.upperDep = open, dependency
		result.upper.kind = smtExtendedFinite
		err = q.Copy(&result.upper.value, value)
	}
	if err != nil {
		return nil, errors.Join(err, result.del())
	}
	return result, nil
}

// del is ~old_interval: it frees both endpoints. A nil interval frees
// nothing.
func (i *smtOldInterval) del() error {
	if i == nil {
		return nil
	}
	return errors.Join(i.lower.del(i.q), i.upper.del(i.q))
}

// clone is the copy constructor interval(other).
func (i *smtOldInterval) clone() (*smtOldInterval, error) {
	result := &smtOldInterval{manager: i.manager, q: i.q, lower: smtExtendedZero(), upper: smtExtendedZero()}
	if err := result.set(i); err != nil {
		return nil, errors.Join(err, result.del())
	}
	return result, nil
}

// set is operator=.
func (i *smtOldInterval) set(other *smtOldInterval) error {
	if err := i.lower.set(i.q, &other.lower); err != nil {
		return err
	}
	if err := i.upper.set(i.q, &other.upper); err != nil {
		return err
	}
	i.lowerOpen, i.upperOpen = other.lowerOpen, other.upperOpen
	i.lowerDep, i.upperDep = other.lowerDep, other.upperDep
	return nil
}

func (i *smtOldInterval) valid(other *smtOldInterval) bool {
	return i != nil && other != nil && i.manager != nil && i.manager == other.manager && i.q == other.q
}

// add is operator+=.
func (i *smtOldInterval) add(other *smtOldInterval) error {
	if !i.valid(other) {
		return ast.ErrASTDomain
	}
	if err := i.lower.add(i.q, &other.lower); err != nil {
		return err
	}
	if err := i.upper.add(i.q, &other.upper); err != nil {
		return err
	}
	i.lowerOpen = i.lowerOpen || other.lowerOpen
	i.upperOpen = i.upperOpen || other.upperOpen
	if i.lower.infinite() {
		i.lowerDep = nil
	} else {
		i.lowerDep = i.manager.Join(i.lowerDep, other.lowerDep)
	}
	if i.upper.infinite() {
		i.upperDep = nil
	} else {
		i.upperDep = i.manager.Join(i.upperDep, other.upperDep)
	}
	return nil
}

// neg swaps the endpoints (std::swap moves the rationals) and negates them.
func (i *smtOldInterval) neg() error {
	i.lower, i.upper = i.upper, i.lower
	i.lowerOpen, i.upperOpen = i.upperOpen, i.lowerOpen
	i.lowerDep, i.upperDep = i.upperDep, i.lowerDep
	if err := i.lower.neg(i.q); err != nil {
		return err
	}
	return i.upper.neg(i.q)
}

// sub is operator-=: interval tmp(other); tmp.neg(); *this += tmp.
func (i *smtOldInterval) sub(other *smtOldInterval) (err error) {
	if !i.valid(other) {
		return ast.ErrASTDomain
	}
	tmp, err := other.clone()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, tmp.del()) }()
	if err = tmp.neg(); err != nil {
		return err
	}
	return i.add(tmp)
}

func (i *smtOldInterval) containsZero() bool {
	return i != nil && (i.lower.sign() < 0 || i.lower.zero() && !i.lowerOpen) && (i.upper.sign() > 0 || i.upper.zero() && !i.upperOpen)
}
func (i *smtOldInterval) minusInfinity() bool { return i == nil || i.lower.infinite() }
func (i *smtOldInterval) plusInfinity() bool  { return i == nil || i.upper.infinite() }

// lowerValue is get_lower_value(): the endpoint's rational, borrowed.
func (i *smtOldInterval) lowerValue() (*numeral.Rational, error) {
	if i == nil || i.lower.infinite() {
		return nil, ast.ErrASTDomain
	}
	return &i.lower.value, nil
}

// upperValue is get_upper_value(): the endpoint's rational, borrowed.
func (i *smtOldInterval) upperValue() (*numeral.Rational, error) {
	if i == nil || i.upper.infinite() {
		return nil, ast.ErrASTDomain
	}
	return &i.upper.value, nil
}

// equal is operator==.
func (i *smtOldInterval) equal(other *smtOldInterval) (bool, error) {
	if i == nil || other == nil {
		return false, nil
	}
	if eq, err := i.lower.equal(i.q, &other.lower); err != nil || !eq {
		return false, err
	}
	if eq, err := i.upper.equal(i.q, &other.upper); err != nil || !eq {
		return false, err
	}
	return i.lowerOpen == other.lowerOpen && i.upperOpen == other.upperOpen, nil
}

// contains is contains(rational const & v).
func (i *smtOldInterval) contains(value *numeral.Rational) (bool, error) {
	if i == nil || value == nil {
		return false, nil
	}
	if !i.lower.infinite() {
		c, err := i.q.Compare(value, &i.lower.value)
		if err != nil || c < 0 || c == 0 && i.lowerOpen {
			return false, err
		}
	}
	if !i.upper.infinite() {
		c, err := i.q.Compare(value, &i.upper.value)
		if err != nil || c > 0 || c == 0 && i.upperOpen {
			return false, err
		}
	}
	return true, nil
}
func (i *smtOldInterval) zero() bool { return i != nil && i.lower.zero() && i.upper.zero() }
func (i *smtOldInterval) positiveOne() bool {
	return i.lower.sign() > 0 || i.lower.zero() && i.lowerOpen
}
func (i *smtOldInterval) negativeOne() bool {
	return i.upper.sign() < 0 || i.upper.zero() && i.upperOpen
}
func (i *smtOldInterval) positive() bool     { return i.lower.sign() >= 0 }
func (i *smtOldInterval) positiveZero() bool { return i.lower.zero() && !i.lowerOpen }
func (i *smtOldInterval) negative() bool     { return i.upper.sign() <= 0 }
func (i *smtOldInterval) negativeZero() bool { return i.upper.zero() && !i.upperOpen }
func (i *smtOldInterval) mixed() bool        { return i.lower.sign() < 0 && i.upper.sign() > 0 }

func (i *smtOldInterval) join3(a, b, c *smtArithmeticBoundDependency) *smtArithmeticBoundDependency {
	return i.manager.Join(i.manager.Join(a, b), c)
}
func (i *smtOldInterval) join4(a, b, c, d *smtArithmeticBoundDependency) *smtArithmeticBoundDependency {
	return i.manager.Join(i.manager.Join(a, b), i.manager.Join(c, d))
}
func (i *smtOldInterval) joinOptional(a, b, first, second *smtArithmeticBoundDependency) *smtArithmeticBoundDependency {
	if first == a || first == b || second == a || second == b || first == nil || second == nil {
		return i.manager.Join(a, b)
	}
	return i.join3(a, b, first)
}

// mul is operator*=. The products new_lower and new_upper are computed
// from the old endpoints first, then assigned (m_lower = new_lower copies)
// and freed, as the source's ext_numeral temporaries are.
func (i *smtOldInterval) mul(other *smtOldInterval) (err error) {
	if !i.valid(other) {
		return ast.ErrASTDomain
	}
	if i.zero() {
		return nil
	}
	q := i.q
	if other.zero() {
		if err = i.set(other); err != nil {
			return err
		}
		i.lowerDep = i.manager.Join(i.lowerDep, i.upperDep)
		i.upperDep = i.lowerDep
		return nil
	}
	a, b, c, d := &i.lower, &i.upper, &other.lower, &other.upper
	ao, bo, co, do := i.lowerOpen, i.upperOpen, other.lowerOpen, other.upperOpen
	ad, bd, cd, dd := i.lowerDep, i.upperDep, other.lowerDep, other.upperDep
	// Multiplying two mixed intervals needs all four endpoint products.
	var temps [4]smtExtendedRational
	used := 0
	defer func() {
		for j := 0; j < used; j++ {
			err = errors.Join(err, temps[j].del(q))
		}
	}()
	product := func(x, y *smtExtendedRational) (*smtExtendedRational, error) {
		r := &temps[used]
		used++
		var e error
		*r, e = smtExtendedProduct(q, x, y)
		return r, e
	}
	set := func(lower *smtExtendedRational, lo bool, ld func() *smtArithmeticBoundDependency, upper *smtExtendedRational, uo bool, ud func() *smtArithmeticBoundDependency) error {
		i.lowerOpen, i.upperOpen = lo, uo
		if e := i.lower.set(q, lower); e != nil {
			return e
		}
		if e := i.upper.set(q, upper); e != nil {
			return e
		}
		i.lowerDep, i.upperDep = nil, nil
		if !i.lower.infinite() {
			i.lowerDep = ld()
		}
		if !i.upper.infinite() {
			i.upperDep = ud()
		}
		return nil
	}
	pair := func(x1, y1, x2, y2 *smtExtendedRational) (*smtExtendedRational, *smtExtendedRational, error) {
		newLower, e := product(x1, y1)
		if e != nil {
			return nil, nil, e
		}
		newUpper, e := product(x2, y2)
		return newLower, newUpper, e
	}
	join := i.manager.Join
	if i.negative() {
		if other.negative() {
			newLower, newUpper, e := pair(b, d, a, c)
			if e != nil {
				return e
			}
			return set(newLower, !(i.negativeZero() || other.negativeZero()) && (bo || do), func() *smtArithmeticBoundDependency { return join(bd, dd) },
				newUpper, ao || co, func() *smtArithmeticBoundDependency { return i.joinOptional(ad, cd, bd, dd) })
		} else if other.mixed() {
			newLower, newUpper, e := pair(a, d, a, c)
			if e != nil {
				return e
			}
			return set(newLower, ao || do, func() *smtArithmeticBoundDependency { return i.join3(ad, dd, bd) },
				newUpper, ao || co, func() *smtArithmeticBoundDependency { return i.join3(ad, cd, bd) })
		}
		newLower, newUpper, e := pair(a, d, b, c)
		if e != nil {
			return e
		}
		n0 := i.negativeZero()
		return set(newLower, ao || do, func() *smtArithmeticBoundDependency { return i.joinOptional(ad, dd, bd, cd) },
			newUpper, !(n0 || other.positiveZero()) && (bo || co), func() *smtArithmeticBoundDependency { return join(bd, cd) })
	} else if i.mixed() {
		if other.negative() {
			newLower, newUpper, e := pair(b, c, a, c)
			if e != nil {
				return e
			}
			return set(newLower, bo || co, func() *smtArithmeticBoundDependency { return i.join3(bd, cd, dd) },
				newUpper, ao || co, func() *smtArithmeticBoundDependency { return i.join3(ad, cd, dd) })
		} else if other.mixed() {
			adn, bcn, e := pair(a, d, b, c)
			if e != nil {
				return e
			}
			acn, bdn, e := pair(a, c, b, d)
			if e != nil {
				return e
			}
			adOpen, bcOpen, acOpen, bdOpen := ao || do, bo || co, ao || co, bo || do
			lower, lowerOpen := adn, adOpen
			cmp, e := adn.cmp(q, bcn)
			if e != nil {
				return e
			}
			if !(cmp < 0 || cmp == 0 && !adOpen && bcOpen) {
				lower, lowerOpen = bcn, bcOpen
			}
			upper, upperOpen := acn, acOpen
			if cmp, e = acn.cmp(q, bdn); e != nil {
				return e
			}
			if !(cmp > 0 || cmp == 0 && !acOpen && bdOpen) {
				upper, upperOpen = bdn, bdOpen
			}
			deps := func() *smtArithmeticBoundDependency { return i.join4(ad, bd, cd, dd) }
			return set(lower, lowerOpen, deps, upper, upperOpen, deps)
		}
		newLower, newUpper, e := pair(a, d, b, d)
		if e != nil {
			return e
		}
		return set(newLower, ao || do, func() *smtArithmeticBoundDependency { return i.join3(ad, dd, cd) },
			newUpper, bo || do, func() *smtArithmeticBoundDependency { return i.join3(bd, dd, cd) })
	}
	if !i.positive() {
		return ast.ErrASTDomain
	}
	if other.negative() {
		newLower, newUpper, e := pair(b, c, a, d)
		if e != nil {
			return e
		}
		p0 := i.positiveZero()
		return set(newLower, bo || co, func() *smtArithmeticBoundDependency { return i.joinOptional(bd, cd, ad, dd) },
			newUpper, !(p0 || other.negativeZero()) && (ao || do), func() *smtArithmeticBoundDependency { return join(ad, dd) })
	} else if other.mixed() {
		newLower, newUpper, e := pair(b, c, b, d)
		if e != nil {
			return e
		}
		return set(newLower, bo || co, func() *smtArithmeticBoundDependency { return i.join3(bd, cd, ad) },
			newUpper, bo || do, func() *smtArithmeticBoundDependency { return i.join3(bd, dd, ad) })
	}
	newLower, newUpper, e := pair(a, c, b, d)
	if e != nil {
		return e
	}
	return set(newLower, !(i.positiveZero() || other.positiveZero()) && (ao || co), func() *smtArithmeticBoundDependency { return join(ad, cd) },
		newUpper, bo || do, func() *smtArithmeticBoundDependency { return i.joinOptional(bd, dd, ad, cd) })
}

// div is operator/=: interval tmp(other); tmp.inv(); *this *= tmp.
func (i *smtOldInterval) div(other *smtOldInterval) (err error) {
	if !i.valid(other) || other.containsZero() {
		return ast.ErrASTDomain
	}
	if i.zero() {
		dependency := other.lowerDep
		if !other.positiveOne() {
			dependency = other.upperDep
		}
		i.lowerDep = i.manager.Join(i.lowerDep, dependency)
		i.upperDep = i.manager.Join(i.upperDep, dependency)
		return nil
	}
	tmp, err := other.clone()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, tmp.del()) }()
	if err = tmp.inverse(); err != nil {
		return err
	}
	return i.mul(tmp)
}

// inverse is inv(): the new endpoints are ext_numeral copies, inverted,
// then assigned and freed.
func (i *smtOldInterval) inverse() (err error) {
	if i == nil || i.manager == nil || i.containsZero() {
		return ast.ErrASTDomain
	}
	q := i.q
	if !i.positiveOne() && !i.negativeOne() {
		return ast.ErrASTDomain
	}
	newLower, newUpper := smtExtendedZero(), smtExtendedZero()
	defer func() { err = errors.Join(err, newLower.del(q), newUpper.del(q)) }()
	if i.positiveOne() {
		if err = newLower.set(q, &i.upper); err != nil {
			return err
		}
		if err = newLower.inverse(q); err != nil {
			return err
		}
		if i.lower.zero() {
			newUpper.kind = smtExtendedPlusInfinity
		} else {
			if err = newUpper.set(q, &i.lower); err != nil {
				return err
			}
			if err = newUpper.inverse(q); err != nil {
				return err
			}
		}
		if err = i.lower.set(q, &newLower); err != nil {
			return err
		}
		if err = i.upper.set(q, &newUpper); err != nil {
			return err
		}
		i.lowerOpen, i.upperOpen = i.upperOpen, i.lowerOpen
		newUpperDep := i.lowerDep
		i.lowerDep = i.manager.Join(i.lowerDep, i.upperDep)
		i.upperDep = newUpperDep
		return nil
	}
	if err = newUpper.set(q, &i.lower); err != nil {
		return err
	}
	if err = newUpper.inverse(q); err != nil {
		return err
	}
	if i.upper.zero() {
		newLower.kind = smtExtendedMinusInfinity
	} else {
		if err = newLower.set(q, &i.upper); err != nil {
			return err
		}
		if err = newLower.inverse(q); err != nil {
			return err
		}
	}
	if err = i.lower.set(q, &newLower); err != nil {
		return err
	}
	if err = i.upper.set(q, &newUpper); err != nil {
		return err
	}
	i.lowerOpen, i.upperOpen = i.upperOpen, i.lowerOpen
	newLowerDep := i.upperDep
	i.upperDep = i.manager.Join(i.lowerDep, i.upperDep)
	i.lowerDep = newLowerDep
	return nil
}

// power is expt(n).
func (i *smtOldInterval) power(exponent uint32) error {
	if i == nil || exponent == 1 {
		return nil
	}
	q := i.q
	both := func() error {
		if err := i.lower.power(q, exponent); err != nil {
			return err
		}
		return i.upper.power(q, exponent)
	}
	if exponent%2 != 0 {
		return both()
	}
	if i.lower.sign() > 0 {
		if err := both(); err != nil {
			return err
		}
		if !i.upper.infinite() {
			i.upperDep = i.manager.Join(i.lowerDep, i.upperDep)
		} else {
			i.upperDep = nil
		}
		return nil
	}
	if i.upper.sign() < 0 {
		i.lower, i.upper = i.upper, i.lower
		i.lowerOpen, i.upperOpen = i.upperOpen, i.lowerOpen
		i.lowerDep, i.upperDep = i.upperDep, i.lowerDep
		if err := both(); err != nil {
			return err
		}
		if !i.upper.infinite() {
			i.upperDep = i.manager.Join(i.lowerDep, i.upperDep)
		} else {
			i.upperDep = nil
		}
		return nil
	}
	if err := both(); err != nil {
		return err
	}
	c, err := i.lower.cmp(q, &i.upper)
	if err != nil {
		return err
	}
	if c > 0 || c == 0 && !i.lowerOpen && i.upperOpen {
		if err = i.upper.set(q, &i.lower); err != nil {
			return err
		}
		i.upperOpen = i.lowerOpen
	}
	if !i.upper.infinite() {
		i.upperDep = i.manager.Join(i.lowerDep, i.upperDep)
	} else {
		i.upperDep = nil
	}
	// m_lower = ext_numeral(0)
	i.lower.kind = smtExtendedFinite
	if err = q.Reset(&i.lower.value); err != nil {
		return err
	}
	i.lowerOpen, i.lowerDep = false, nil
	return nil
}

// String is display.
func (i *smtOldInterval) String() string {
	if i == nil {
		return "<nil>"
	}
	left, right := "[", "]"
	if i.lowerOpen {
		left = "("
	}
	if i.upperOpen {
		right = ")"
	}
	lower, err := i.lower.String(i.q)
	if err != nil {
		lower = err.Error()
	}
	upper, err := i.upper.String(i.q)
	if err != nil {
		upper = err.Error()
	}
	return left + lower + ", " + upper + right
}

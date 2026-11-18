package smtcore

// Source: src/muz/rel/dl_interval_relation.cpp:125-190
// (interval_relation_plugin::unite, widen, meet, is_infinite, is_empty) over
// the old_interval of src/smt/old_interval.{h,cpp}.
//
// interval_relation's columns are old_intervals whose dependencies are
// always nullptr. A RelationInterval is such an old_interval: its endpoints
// are ext_numerals over rationals of the AST manager's rational manager. As
// in the source, intervals are values: free each one with Del where the
// source's interval dies; a Go copy of the struct is a move, never a copy.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// RelationIntervalManager is interval_relation_plugin's m_dep
// (v_dependency_manager) together with the rational manager its intervals'
// rationals belong to.
type RelationIntervalManager struct {
	dep smtArithmeticBoundDependencyManager
	q   *numeral.RationalManager
}

func NewRelationIntervalManager(q *numeral.RationalManager) *RelationIntervalManager {
	return &RelationIntervalManager{q: q}
}

// Rationals is the rational manager of the intervals' endpoints.
func (m *RelationIntervalManager) Rationals() *numeral.RationalManager { return m.q }

// RelationInterval is an old_interval of interval_relation.
type RelationInterval struct{ i smtOldInterval }

// Full is interval(m_dep): (-oo, oo).
func (m *RelationIntervalManager) Full() RelationInterval {
	return RelationInterval{smtOldInterval{manager: &m.dep, q: m.q, lower: smtInfiniteExtended(false), upper: smtInfiniteExtended(true), lowerOpen: true, upperOpen: true}}
}

// Point is interval(m_dep, val): [val, val].
func (m *RelationIntervalManager) Point(value *numeral.Rational) (RelationInterval, error) {
	i, err := newSMTOldIntervalPoint(&m.dep, m.q, value, nil, nil)
	if err != nil {
		return RelationInterval{}, err
	}
	return RelationInterval{*i}, nil
}

// HalfBound is interval(m_dep, val, open, lower, nullptr): [val, oo),
// (val, oo), (-oo, val] or (-oo, val).
func (m *RelationIntervalManager) HalfBound(value *numeral.Rational, open, lower bool) (RelationInterval, error) {
	i, err := newSMTOldIntervalHalfBound(&m.dep, m.q, value, open, lower, nil)
	if err != nil {
		return RelationInterval{}, err
	}
	return RelationInterval{*i}, nil
}

// extended is interval(m_dep, lower, l_open, nullptr, upper, u_open,
// nullptr) over ext_numerals; the endpoints are copied.
func (m *RelationIntervalManager) extended(lower *smtExtendedRational, lowerOpen bool, upper *smtExtendedRational, upperOpen bool) (RelationInterval, error) {
	result := RelationInterval{smtOldInterval{manager: &m.dep, q: m.q, lower: smtExtendedZero(), upper: smtExtendedZero(), lowerOpen: lowerOpen, upperOpen: upperOpen}}
	err := result.i.lower.set(m.q, lower)
	if err == nil {
		err = result.i.upper.set(m.q, upper)
	}
	if err != nil {
		return RelationInterval{}, errors.Join(err, result.i.del())
	}
	return result, nil
}

// Bounds is the ext_numeral constructor with finite endpoints given as
// rationals; a nil endpoint is the matching infinity.
func (m *RelationIntervalManager) Bounds(lower *numeral.Rational, lowerOpen bool, upper *numeral.Rational, upperOpen bool) (RelationInterval, error) {
	low, high := smtInfiniteExtended(false), smtInfiniteExtended(true)
	if lower != nil {
		low.kind, low.value = smtExtendedFinite, *lower
	}
	if upper != nil {
		high.kind, high.value = smtExtendedFinite, *upper
	}
	// low and high borrow the caller's rationals; extended copies them.
	return m.extended(&low, lowerOpen, &high, upperOpen)
}

// Del is ~old_interval.
func (m *RelationIntervalManager) Del(i *RelationInterval) error { return i.i.del() }

// Set is old_interval::operator=.
func (m *RelationIntervalManager) Set(dst, src *RelationInterval) error { return dst.i.set(&src.i) }

// Clone is the copy constructor old_interval(other).
func (m *RelationIntervalManager) Clone(src *RelationInterval) (RelationInterval, error) {
	result := m.Full()
	if err := result.i.set(&src.i); err != nil {
		return RelationInterval{}, errors.Join(err, result.i.del())
	}
	return result, nil
}

// Add is operator+=.
func (m *RelationIntervalManager) Add(dst, other *RelationInterval) error { return dst.i.add(&other.i) }

// Sub is operator-=.
func (m *RelationIntervalManager) Sub(dst, other *RelationInterval) error { return dst.i.sub(&other.i) }

func (i *RelationInterval) MinusInfinity() bool { return i.i.lower.infinite() }
func (i *RelationInterval) PlusInfinity() bool  { return i.i.upper.infinite() }
func (i *RelationInterval) LowerOpen() bool     { return i.i.lowerOpen }
func (i *RelationInterval) UpperOpen() bool     { return i.i.upperOpen }

// LowerValue is get_lower_value(): the endpoint's rational, borrowed.
func (i *RelationInterval) LowerValue() (*numeral.Rational, error) { return i.i.lowerValue() }

// UpperValue is get_upper_value(): the endpoint's rational, borrowed.
func (i *RelationInterval) UpperValue() (*numeral.Rational, error) { return i.i.upperValue() }

// Contains is contains(rational const & v).
func (i *RelationInterval) Contains(value *numeral.Rational) (bool, error) {
	return i.i.contains(value)
}

// Equal is operator==.
func (i *RelationInterval) Equal(other *RelationInterval) (bool, error) {
	return i.i.equal(&other.i)
}

// String is display.
func (i *RelationInterval) String() string { return i.i.String() }

// IsInfinite is interval_relation_plugin::is_infinite.
func (i *RelationInterval) IsInfinite() bool { return i.PlusInfinity() && i.MinusInfinity() }

// IsEmpty is interval_relation_plugin::is_empty: sup() < inf().
func (i *RelationInterval) IsEmpty() (bool, error) {
	c, err := i.i.upper.cmp(i.i.q, &i.i.lower)
	return c < 0, err
}

// Unite is interval_relation_plugin::unite. The caller frees the result.
func (m *RelationIntervalManager) Unite(src1, src2 *RelationInterval) (_ RelationInterval, resultErr error) {
	q := m.q
	lOpen, rOpen := src1.i.lowerOpen, src1.i.upperOpen
	low, err := src1.i.lower.clone(q)
	if err != nil {
		return RelationInterval{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, low.del(q)) }()
	high, err := src1.i.upper.clone(q)
	if err != nil {
		return RelationInterval{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, high.del(q)) }()
	c, err := src2.i.lower.cmp(q, &low)
	if err != nil {
		return RelationInterval{}, err
	}
	if c < 0 || c == 0 && lOpen {
		if err = low.set(q, &src2.i.lower); err != nil {
			return RelationInterval{}, err
		}
		lOpen = src2.i.lowerOpen
	}
	if c, err = src2.i.upper.cmp(q, &high); err != nil {
		return RelationInterval{}, err
	}
	if c > 0 || c == 0 && rOpen {
		if err = high.set(q, &src2.i.upper); err != nil {
			return RelationInterval{}, err
		}
		rOpen = src2.i.upperOpen
	}
	return m.extended(&low, lOpen, &high, rOpen)
}

// Widen is interval_relation_plugin::widen. The caller frees the result.
func (m *RelationIntervalManager) Widen(src1, src2 *RelationInterval) (_ RelationInterval, resultErr error) {
	q := m.q
	lOpen, rOpen := src1.i.lowerOpen, src1.i.upperOpen
	low, err := src1.i.lower.clone(q)
	if err != nil {
		return RelationInterval{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, low.del(q)) }()
	high, err := src1.i.upper.clone(q)
	if err != nil {
		return RelationInterval{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, high.del(q)) }()
	c, err := src2.i.lower.cmp(q, &low)
	if err != nil {
		return RelationInterval{}, err
	}
	if c < 0 || c == 0 && lOpen && !src2.i.lowerOpen {
		// low = ext_numeral(false)
		minus := smtInfiniteExtended(false)
		if err = low.set(q, &minus); err != nil {
			return RelationInterval{}, err
		}
		lOpen = true
	}
	if c, err = high.cmp(q, &src2.i.upper); err != nil {
		return RelationInterval{}, err
	}
	if c < 0 || c == 0 && !rOpen && src2.i.upperOpen {
		// high = ext_numeral(true)
		plus := smtInfiniteExtended(true)
		if err = high.set(q, &plus); err != nil {
			return RelationInterval{}, err
		}
		rOpen = true
	}
	return m.extended(&low, lOpen, &high, rOpen)
}

// Meet is interval_relation_plugin::meet; it reports an empty
// intersection. The caller frees the result.
func (m *RelationIntervalManager) Meet(src1, src2 *RelationInterval) (_ RelationInterval, isEmpty bool, resultErr error) {
	q := m.q
	empty, err := src1.IsEmpty()
	if err != nil {
		return RelationInterval{}, false, err
	}
	if empty || src2.IsInfinite() {
		r, err := m.Clone(src1)
		return r, false, err
	}
	if empty, err = src2.IsEmpty(); err != nil {
		return RelationInterval{}, false, err
	}
	if empty || src1.IsInfinite() {
		r, err := m.Clone(src2)
		return r, false, err
	}
	lOpen, rOpen := src1.i.lowerOpen, src1.i.upperOpen
	low, err := src1.i.lower.clone(q)
	if err != nil {
		return RelationInterval{}, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, low.del(q)) }()
	high, err := src1.i.upper.clone(q)
	if err != nil {
		return RelationInterval{}, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, high.del(q)) }()
	c, err := src2.i.lower.cmp(q, &low)
	if err != nil {
		return RelationInterval{}, false, err
	}
	if c > 0 || c == 0 && !lOpen {
		if err = low.set(q, &src2.i.lower); err != nil {
			return RelationInterval{}, false, err
		}
		lOpen = src2.i.lowerOpen
	}
	if c, err = src2.i.upper.cmp(q, &high); err != nil {
		return RelationInterval{}, false, err
	}
	if c < 0 || c == 0 && !rOpen {
		if err = high.set(q, &src2.i.upper); err != nil {
			return RelationInterval{}, false, err
		}
		rOpen = src2.i.upperOpen
	}
	if c, err = low.cmp(q, &high); err != nil {
		return RelationInterval{}, false, err
	}
	if c > 0 || c == 0 && (lOpen || rOpen) {
		return m.Full(), true, nil
	}
	r, err := m.extended(&low, lOpen, &high, rOpen)
	return r, false, err
}

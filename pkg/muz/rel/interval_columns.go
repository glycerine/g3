package rel

// Source: src/muz/rel/dl_interval_relation.cpp:417-502 and the concrete
// interval instantiation of src/muz/rel/dl_vector_relation.h.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// intervalColumns is the column and equality portion of the interval
// relation: m_elems (each column's interval is owned and freed with the
// vector), m_eqs and m_empty.
type intervalColumns struct {
	intervals *smtcore.RelationIntervalManager
	columns   []smtcore.RelationInterval
	parent    []uint32
	size      []uint32
	empty     bool
}

// newIntervalColumns is vector_relation's constructor: arity copies of
// m_default, the full interval.
func newIntervalColumns(intervals *smtcore.RelationIntervalManager, arity int, empty bool) *intervalColumns {
	r := &intervalColumns{intervals: intervals, columns: make([]smtcore.RelationInterval, arity), parent: make([]uint32, arity), size: make([]uint32, arity), empty: empty}
	for i := range r.columns {
		r.columns[i] = intervals.Full()
		r.parent[i] = uint32(i)
		r.size[i] = 1
	}
	return r
}

// del is dealloc(m_elems): every column's interval is freed.
func (r *intervalColumns) del() (result error) {
	if r == nil {
		return nil
	}
	for i := range r.columns {
		result = errors.Join(result, r.intervals.Del(&r.columns[i]))
	}
	r.columns = nil
	return result
}

func (r *intervalColumns) find(index uint32) uint32 {
	for r.parent[index] != index {
		index = r.parent[index]
	}
	return index
}

func (r *intervalColumns) merge(i, j uint32) uint32 {
	i, j = r.find(i), r.find(j)
	if i == j {
		return i
	}
	if r.size[i] > r.size[j] {
		i, j = j, i
	}
	r.parent[i] = j
	r.size[j] += r.size[i]
	return j
}

// setEmpty is set_empty: m_elems->reset() frees the intervals, resize
// refills it with m_default, and the equalities are recreated.
func (r *intervalColumns) setEmpty() (result error) {
	r.empty = true
	for i := range r.columns {
		result = errors.Join(result, r.intervals.Del(&r.columns[i]))
	}
	for i := range r.columns {
		r.parent[i] = uint32(i)
		r.size[i] = 1
		r.columns[i] = r.intervals.Full()
	}
	return result
}

// assign is (*this)[index] = value on a representative index.
func (r *intervalColumns) assign(index uint32, value *smtcore.RelationInterval) error {
	return r.intervals.Set(&r.columns[index], value)
}

// equate is vector_relation::equate: T r = mk_intersect(...) dies at the
// end of the block.
func (r *intervalColumns) equate(i, j uint32) (resultErr error) {
	if uint64(i) >= uint64(len(r.columns)) || uint64(j) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	if r.empty {
		return nil
	}
	i, j = r.find(i), r.find(j)
	if i == j {
		return nil
	}
	value, empty, err := r.intervals.Meet(&r.columns[i], &r.columns[j])
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.intervals.Del(&value)) }()
	if !empty {
		if empty, err = value.IsEmpty(); err != nil {
			return err
		}
	}
	if empty {
		r.empty = true
		return nil
	}
	// union_find attaches the smaller root to the larger root, choosing the
	// second root on equal sizes.
	return r.assign(r.merge(i, j), &value)
}

// intersect is interval_relation::mk_intersect(idx, i): the meet is a
// temporary assigned into the column.
func (r *intervalColumns) intersect(index uint32, bound *smtcore.RelationInterval) error {
	if uint64(index) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	index = r.find(index)
	value, empty, err := r.intervals.Meet(&r.columns[index], bound)
	if err != nil {
		return err
	}
	err = r.assign(index, &value)
	if err = errors.Join(err, r.intervals.Del(&value)); err != nil {
		return err
	}
	if !empty {
		if empty, err = r.columns[index].IsEmpty(); err != nil {
			return err
		}
	}
	if empty {
		return r.setEmpty()
	}
	return nil
}

// intersectWith is mk_intersect(index, interval(...)) for a temporary
// interval: intersectWith(index)(interval, err); the temporary dies after
// the call.
func (r *intervalColumns) intersectWith(index uint32) func(smtcore.RelationInterval, error) error {
	return func(bound smtcore.RelationInterval, err error) error {
		if err != nil {
			return err
		}
		return errors.Join(r.intersect(index, &bound), r.intervals.Del(&bound))
	}
}

// filterInterpreted is interval_relation::filter_interpreted; unknown
// predicates leave the relation unchanged, as in the source.
func (r *intervalColumns) filterInterpreted(m *ast.ASTManager, condition ast.ASTHandle) (resultErr error) {
	q := r.intervals.Rationals()
	kind, c, ok, err := intervalComparison(m, q, condition)
	defer func() { resultErr = errors.Join(resultErr, q.Del(&c.constant)) }()
	if err != nil {
		return err
	}
	if !ok {
		if m.IsFalse(condition) {
			return r.setEmpty()
		}
		return nil
	}
	x, y, k := c.positive, c.negative, &c.constant
	if x != math.MaxUint32 && uint64(x) >= uint64(len(r.columns)) || y != math.MaxUint32 && uint64(y) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	// -k, k + x_hi and y_lo - k are rational temporaries.
	temporary := func(f func(out *numeral.Rational) error, use func(*numeral.Rational) error) error {
		value := numeral.NewRational()
		err := f(&value)
		if err == nil {
			err = use(&value)
		}
		return errors.Join(err, q.Del(&value))
	}
	negated := func(out *numeral.Rational) error {
		if err := q.Copy(out, k); err != nil {
			return err
		}
		return q.Neg(out)
	}
	intervals := r.intervals
	if kind == 2 {
		// y = x + k
		if x == math.MaxUint32 {
			return r.intersectWith(y)(intervals.Point(k))
		}
		if y == math.MaxUint32 {
			// x = - k
			return temporary(negated, func(v *numeral.Rational) error { return r.intersectWith(x)(intervals.Point(v)) })
		}
		// interval x_i = (*this)[x]; interval y_i = (*this)[y];
		xi, err := intervals.Clone(&r.columns[r.find(x)])
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, intervals.Del(&xi)) }()
		yi, err := intervals.Clone(&r.columns[r.find(y)])
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, intervals.Del(&yi)) }()
		// x_i += interval(p.dep(), k); y_i -= interval(p.dep(), k);
		shift := func(target *smtcore.RelationInterval, add bool) error {
			point, err := intervals.Point(k)
			if err != nil {
				return err
			}
			if add {
				err = intervals.Add(target, &point)
			} else {
				err = intervals.Sub(target, &point)
			}
			return errors.Join(err, intervals.Del(&point))
		}
		if err = shift(&xi, true); err != nil {
			return err
		}
		if err = shift(&yi, false); err != nil {
			return err
		}
		if err = r.intersect(x, &yi); err != nil {
			return err
		}
		return r.intersect(y, &xi)
	}
	open := kind == 1
	if x == math.MaxUint32 {
		// y < k, y <= k
		return r.intersectWith(y)(intervals.HalfBound(k, open, false))
	}
	if y == math.MaxUint32 {
		// -k < x, -k <= x
		return temporary(negated, func(v *numeral.Rational) error { return r.intersectWith(x)(intervals.HalfBound(v, open, true)) })
	}
	// ext_numeral x_hi = (*this)[x].sup(); ext_numeral y_lo = (*this)[y].inf();
	xHigh, yLow := numeral.NewRational(), numeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&yLow), q.Del(&xHigh)) }()
	xColumn, yColumn := &r.columns[r.find(x)], &r.columns[r.find(y)]
	xHighInfinite, yLowInfinite := xColumn.PlusInfinity(), yColumn.MinusInfinity()
	if !xHighInfinite {
		value, err := xColumn.UpperValue()
		if err == nil {
			err = q.Copy(&xHigh, value)
		}
		if err != nil {
			return err
		}
	}
	if !yLowInfinite {
		value, err := yColumn.LowerValue()
		if err == nil {
			err = q.Copy(&yLow, value)
		}
		if err != nil {
			return err
		}
	}
	if !xHighInfinite {
		sum := func(out *numeral.Rational) error { return q.Add(k, &xHigh, out) }
		if err := temporary(sum, func(v *numeral.Rational) error { return r.intersectWith(y)(intervals.HalfBound(v, open, false)) }); err != nil {
			return err
		}
	}
	if !yLowInfinite {
		difference := func(out *numeral.Rational) error { return q.Sub(&yLow, k, out) }
		return temporary(difference, func(v *numeral.Rational) error { return r.intersectWith(x)(intervals.HalfBound(v, open, true)) })
	}
	return nil
}

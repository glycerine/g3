package rel

// Test-only IntervalRelation accessors: the datalog engine reaches the
// interval relation through its plugin functors and AST facts only. The
// tests state values as math/big oracles; these helpers convert them to
// and from the relation's rationals.

import (
	"errors"
	"math/big"
	"sync"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var intervalTestOnce struct {
	sync.Once
	m *smtcore.RelationIntervalManager
}

// IntervalTestManager is a relation interval manager over a private
// rational manager for tests that use no AST manager's numerals.
func IntervalTestManager() *smtcore.RelationIntervalManager {
	intervalTestOnce.Do(func() {
		q, err := numeral.NewRationalManager(numeral.RationalConfig{})
		if err != nil {
			panic(err)
		}
		intervalTestOnce.m = smtcore.NewRelationIntervalManager(q)
	})
	return intervalTestOnce.m
}

// intervalTestRational converts a big.Rat oracle into a rational of q.
func intervalTestRational(q *numeral.RationalManager, value *big.Rat) (numeral.Rational, error) {
	r := numeral.NewRational()
	if err := q.SetString(&r, value.RatString()); err != nil {
		return r, errors.Join(err, q.Del(&r))
	}
	return r, nil
}

// NewTestRelationInterval builds an interval of IntervalTestManager from
// big.Rat endpoints; nil is an infinity.
func NewTestRelationInterval(lower *big.Rat, lowerOpen bool, upper *big.Rat, upperOpen bool) smtcore.RelationInterval {
	m := IntervalTestManager()
	q := m.Rationals()
	var lo, hi *numeral.Rational
	if lower != nil {
		r, err := intervalTestRational(q, lower)
		if err != nil {
			panic(err)
		}
		defer q.Del(&r)
		lo = &r
	}
	if upper != nil {
		r, err := intervalTestRational(q, upper)
		if err != nil {
			panic(err)
		}
		defer q.Del(&r)
		hi = &r
	}
	i, err := m.Bounds(lo, lowerOpen, hi, upperOpen)
	if err != nil {
		panic(err)
	}
	return i
}

// intervalTestBig converts a rational of q into a big.Rat oracle value.
func intervalTestBig(q *numeral.RationalManager, r *numeral.Rational) (*big.Rat, error) {
	text, err := q.String(r)
	if err != nil {
		return nil, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return value, nil
}

// intervalBigBounds reads an interval's endpoints as big.Rat copies.
func intervalBigBounds(q *numeral.RationalManager, i *smtcore.RelationInterval) (lower *big.Rat, lowerOpen bool, upper *big.Rat, upperOpen bool, err error) {
	if !i.MinusInfinity() {
		v, e := i.LowerValue()
		if e != nil {
			return nil, false, nil, false, e
		}
		if lower, err = intervalTestBig(q, v); err != nil {
			return
		}
	}
	if !i.PlusInfinity() {
		v, e := i.UpperValue()
		if e != nil {
			return nil, false, nil, false, e
		}
		if upper, err = intervalTestBig(q, v); err != nil {
			return
		}
	}
	return lower, i.LowerOpen(), upper, i.UpperOpen(), nil
}

// IsFull mirrors vector_relation::is_full for interval columns.
func (r *IntervalRelation) IsFull() bool {
	if r == nil || r.values == nil {
		return false
	}
	for i := range r.values.columns {
		if !r.values.columns[r.values.find(uint32(i))].IsInfinite() {
			return false
		}
	}
	return true
}

// Bounds returns copies of the column's endpoints; nil denotes infinity.
func (r *IntervalRelation) Bounds(column uint32) (*big.Rat, bool, *big.Rat, bool, error) {
	if r == nil || r.values == nil || uint64(column) >= uint64(len(r.values.columns)) {
		return nil, false, nil, false, ast.ErrASTDomain
	}
	return intervalBigBounds(r.values.intervals.Rationals(), &r.values.columns[r.values.find(column)])
}

func (r *IntervalRelation) FilterIdentical(first, second uint32) error {
	return r.FilterIdenticalColumns([]uint32{first, second})
}

// FilterEqualRat is FilterEqual with a big.Rat oracle value.
func (r *IntervalRelation) FilterEqualRat(column uint32, value *big.Rat) error {
	if r == nil || r.values == nil || value == nil {
		return ast.ErrASTDomain
	}
	q := r.values.intervals.Rationals()
	v, err := intervalTestRational(q, value)
	if err != nil {
		return err
	}
	return errors.Join(r.FilterEqual(column, &v), q.Del(&v))
}

// AddFact joins a concrete rational tuple into the interval abstraction.
func (r *IntervalRelation) AddFact(values []*big.Rat) (resultErr error) {
	if r == nil || r.values == nil || len(values) != len(r.values.columns) {
		return ast.ErrASTDomain
	}
	q := r.values.intervals.Rationals()
	fact := newIntervalColumns(r.values.intervals, len(values), false)
	defer func() { resultErr = errors.Join(resultErr, fact.del()) }()
	for i, value := range values {
		if value == nil {
			return ast.ErrASTDomain
		}
		v, err := intervalTestRational(q, value)
		if err != nil {
			return err
		}
		point, err := r.values.intervals.Point(&v)
		if err == nil {
			err = fact.assign(uint32(i), &point)
			err = errors.Join(err, r.values.intervals.Del(&point))
		}
		if err = errors.Join(err, q.Del(&v)); err != nil {
			return err
		}
	}
	_, err := unionIntervalColumns(r.values, fact, nil, false)
	return err
}

func (r *IntervalRelation) ContainsFact(values []*big.Rat) bool {
	if r == nil || r.values == nil || len(values) != len(r.values.columns) {
		return false
	}
	q := r.values.intervals.Rationals()
	// The C++ contains_fact path reads reset column payloads even when the
	// relation's empty flag is set, so do not short-circuit on r.Empty().
	for i, value := range values {
		root := r.values.find(uint32(i))
		if value == nil || values[root] == nil || value.Cmp(values[root]) != 0 {
			return false
		}
		v, err := intervalTestRational(q, value)
		if err != nil {
			return false
		}
		contained, err := r.values.columns[root].Contains(&v)
		if err = errors.Join(err, q.Del(&v)); err != nil || !contained {
			return false
		}
	}
	return true
}

// Arity returns the concrete vector-relation column count.
func (r *IntervalRelation) Arity() (uint32, error) {
	if r == nil || r.values == nil {
		return 0, ast.ErrASTDomain
	}
	return uint32(len(r.values.columns)), nil
}

// testBounds reads an interval's endpoints as big.Rat copies; nil is an
// infinity.
func testBounds(i *smtcore.RelationInterval) (*big.Rat, bool, *big.Rat, bool) {
	lower, lowerOpen, upper, upperOpen, err := intervalBigBounds(IntervalTestManager().Rationals(), i)
	if err != nil {
		panic(err)
	}
	return lower, lowerOpen, upper, upperOpen
}

// testContains is contains(v) for a big.Rat oracle value.
func testContains(i *smtcore.RelationInterval, value *big.Rat) bool {
	q := IntervalTestManager().Rationals()
	v, err := intervalTestRational(q, value)
	if err != nil {
		panic(err)
	}
	ok, err := i.Contains(&v)
	if err = errors.Join(err, q.Del(&v)); err != nil {
		panic(err)
	}
	return ok
}

// testIntersect is mk_intersect(index, v); v is consumed.
func testIntersect(r *intervalColumns, index uint32, v smtcore.RelationInterval) error {
	return r.intersectWith(index)(v, nil)
}

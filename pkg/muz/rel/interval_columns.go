package rel

// Source: src/muz/rel/dl_interval_relation.cpp:417-502.

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// intervalColumns is the independent-column portion of the interval relation.
// Column equality and the relation plugin operations are ported separately.
type intervalColumns struct {
	columns []smtcore.RelationInterval
	empty   bool
}

func newIntervalColumns(arity int, empty bool) *intervalColumns {
	r := &intervalColumns{columns: make([]smtcore.RelationInterval, arity), empty: empty}
	for i := range r.columns {
		r.columns[i] = smtcore.NewRelationInterval(nil, true, nil, true)
	}
	return r
}

func (r *intervalColumns) intersect(index uint32, bound smtcore.RelationInterval) error {
	if uint64(index) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	var empty bool
	r.columns[index], empty = r.columns[index].Meet(bound)
	r.empty = r.empty || empty || r.columns[index].EmptyBounds()
	return nil
}

func shiftedInterval(i smtcore.RelationInterval, shift *big.Rat) smtcore.RelationInterval {
	lo, loOpen, hi, hiOpen := i.Bounds()
	if lo != nil {
		lo.Add(lo, shift)
	}
	if hi != nil {
		hi.Add(hi, shift)
	}
	return smtcore.NewRelationInterval(lo, loOpen, hi, hiOpen)
}

// filterInterpreted applies recognized linear comparisons; unknown predicates
// leave the relation unchanged, as in the source.
func (r *intervalColumns) filterInterpreted(m *ast.ASTManager, condition ast.ASTHandle) error {
	kind, c, ok, err := intervalComparison(m, condition)
	if err != nil {
		return err
	}
	if !ok {
		if m.IsFalse(condition) {
			r.empty = true
		}
		return nil
	}
	x, y, k := c.positive, c.negative, &c.constant
	if x != math.MaxUint32 && uint64(x) >= uint64(len(r.columns)) || y != math.MaxUint32 && uint64(y) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	if kind == 2 {
		if x == math.MaxUint32 {
			return r.intersect(y, smtcore.NewRelationInterval(k, false, k, false))
		}
		minusK := new(big.Rat).Neg(k)
		if y == math.MaxUint32 {
			return r.intersect(x, smtcore.NewRelationInterval(minusK, false, minusK, false))
		}
		xBound, yBound := shiftedInterval(r.columns[x], k), shiftedInterval(r.columns[y], minusK)
		if err := r.intersect(x, yBound); err != nil {
			return err
		}
		return r.intersect(y, xBound)
	}
	open := kind == 1
	if x == math.MaxUint32 {
		return r.intersect(y, smtcore.NewRelationInterval(nil, true, k, open))
	}
	if y == math.MaxUint32 {
		return r.intersect(x, smtcore.NewRelationInterval(new(big.Rat).Neg(k), open, nil, true))
	}
	_, _, xHigh, _ := r.columns[x].Bounds()
	yLow, _, _, _ := r.columns[y].Bounds()
	if xHigh != nil {
		if err := r.intersect(y, smtcore.NewRelationInterval(nil, true, new(big.Rat).Add(k, xHigh), open)); err != nil {
			return err
		}
	}
	if yLow != nil {
		return r.intersect(x, smtcore.NewRelationInterval(new(big.Rat).Sub(yLow, k), open, nil, true))
	}
	return nil
}

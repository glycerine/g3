package rel

// Source: src/muz/rel/udoc_relation.cpp udoc_plugin::filter_equal_fn.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
)

// FilterEqual keeps rows whose selected bit-vector column equals value.
func (r *UDocRelation) FilterEqual(column uint32, value uint64) (resultErr error) {
	return r.FilterEqualBig(column, new(big.Int).SetUint64(value))
}

// FilterEqualBig preserves bit-vector values wider than a machine word.
func (r *UDocRelation) FilterEqualBig(column uint32, value *big.Int) (resultErr error) {
	if err := r.check(); err != nil {
		return err
	}
	if column >= uint32(len(r.widths)) {
		return arena.ErrBounds
	}
	if value == nil || value.Sign() < 0 {
		return ErrDOC
	}
	width := r.widths[column]
	if value.BitLen() > int(width) {
		return ErrDOC
	}
	document, err := r.manager.AllocateX()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.Deallocate(document)) }()
	low, high := r.offsets[column], r.offsets[column+1]-1
	if err = r.manager.vectors.SetBigInt(document.positive, new(big.Int).Set(value), high, low); err != nil {
		return err
	}
	return r.elems.IntersectDoc(document)
}

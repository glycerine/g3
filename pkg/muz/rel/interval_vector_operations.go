package rel

// Source: the interval instantiation of src/muz/rel/dl_vector_relation.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// joinIntervalColumns follows mk_join: source column values are copied, then
// the requested cross-relation column pairs are equated.
func joinIntervalColumns(left, right *intervalColumns, leftCols, rightCols []uint32) (_ *intervalColumns, resultErr error) {
	if left == nil || right == nil || len(leftCols) != len(rightCols) {
		return nil, ast.ErrASTDomain
	}
	for i := range leftCols {
		if uint64(leftCols[i]) >= uint64(len(left.columns)) || uint64(rightCols[i]) >= uint64(len(right.columns)) {
			return nil, ast.ErrASTDomain
		}
	}
	result := newIntervalColumns(left.intervals, len(left.columns)+len(right.columns), false)
	if left.empty || right.empty {
		result.empty = true
		return result, nil
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.del())
		}
	}()
	for i := range left.columns {
		if err := result.assign(uint32(i), &left.columns[left.find(uint32(i))]); err != nil {
			return nil, err
		}
	}
	for i := range right.columns {
		if err := result.assign(uint32(len(left.columns)+i), &right.columns[right.find(uint32(i))]); err != nil {
			return nil, err
		}
	}
	for i := range leftCols {
		if err := result.equate(leftCols[i], uint32(len(left.columns))+rightCols[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// projectIntervalColumns follows mk_project; removed columns must be sorted
// and unique, as required by its native caller.
func projectIntervalColumns(source *intervalColumns, removed []uint32) (_ *intervalColumns, resultErr error) {
	if source == nil || len(removed) > len(source.columns) {
		return nil, ast.ErrASTDomain
	}
	for i, column := range removed {
		if uint64(column) >= uint64(len(source.columns)) || i > 0 && removed[i-1] >= column {
			return nil, ast.ErrASTDomain
		}
	}
	result := newIntervalColumns(source.intervals, len(source.columns)-len(removed), false)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.del())
		}
	}()
	representatives := make(map[uint32]uint32)
	removedIndex, dst := 0, uint32(0)
	for i := uint32(0); uint64(i) < uint64(len(source.columns)); i++ {
		if removedIndex < len(removed) && removed[removedIndex] == i {
			removedIndex++
			continue
		}
		root := source.find(i)
		if err := result.assign(dst, &source.columns[root]); err != nil {
			return nil, err
		}
		if first, ok := representatives[root]; ok {
			result.merge(first, dst)
		} else {
			representatives[root] = dst
		}
		dst++
	}
	return result, nil
}

// renameIntervalColumns rotates the given columns and preserves the source's
// equality classes. The destination of cycle[i] is cycle[(i+1)%len(cycle)].
func renameIntervalColumns(source *intervalColumns, cycle []uint32) (_ *intervalColumns, resultErr error) {
	if source == nil || len(cycle) == 0 {
		return nil, ast.ErrASTDomain
	}
	seen := make(map[uint32]bool, len(cycle))
	for _, column := range cycle {
		if uint64(column) >= uint64(len(source.columns)) || seen[column] {
			return nil, ast.ErrASTDomain
		}
		seen[column] = true
	}
	result := newIntervalColumns(source.intervals, len(source.columns), false)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.del())
		}
	}()
	classes := make([]uint32, len(source.columns))
	for i := range source.columns {
		classes[i] = source.find(uint32(i))
		if err := result.assign(uint32(i), &source.columns[classes[i]]); err != nil {
			return nil, err
		}
	}
	for i, column := range cycle {
		destination := cycle[(i+1)%len(cycle)]
		// The native routine reads the source vector directly here.
		if err := result.assign(destination, &source.columns[column]); err != nil {
			return nil, err
		}
		classes[destination] = source.find(column)
	}
	first := make(map[uint32]uint32)
	for i, class := range classes {
		if representative, ok := first[class]; ok {
			result.merge(representative, uint32(i))
		} else {
			first[class] = uint32(i)
		}
	}
	return result, nil
}

// unionIntervalColumns is vector_relation::mk_union without the delta: the
// target gets the source's hull or widening. The result reports the source's
// delta-change condition.
func unionIntervalColumns(target, source *intervalColumns, widen bool) (_ bool, resultErr error) {
	if target == nil || source == nil || len(target.columns) != len(source.columns) {
		return false, ast.ErrASTDomain
	}
	if source.empty {
		return false, nil
	}
	intervals := target.intervals
	if target.empty {
		// vector_relation::copy copies the values through the source's
		// representatives but initializes fresh destination equivalences.
		if err := target.setEmpty(); err != nil {
			return false, err
		}
		target.empty = false
		for i := range target.columns {
			if err := target.assign(uint32(i), &source.columns[source.find(uint32(i))]); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	// uf and elems: elems starts empty and receives each column's unite or
	// widen result.
	result := newIntervalColumns(intervals, len(target.columns), false)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.del())
		}
	}()
	type roots struct{ target, source uint32 }
	classes := make(map[roots]uint32)
	seenTarget := make(map[uint32]bool)
	changed := false
	for i := range target.columns {
		index := uint32(i)
		key := roots{target.find(index), source.find(index)}
		if first, ok := classes[key]; ok {
			result.merge(index, first)
		} else {
			classes[key] = index
			if seenTarget[key.target] {
				changed = true
			}
			seenTarget[key.target] = true
		}
	}
	for i := range target.columns {
		index := uint32(i)
		old := &target.columns[target.find(index)]
		// T t1 = mk_eq(*m_eqs, *uf, (*this)[i]); T t2 = mk_eq(..., src[i]);
		t1, err := intervals.Clone(old)
		if err != nil {
			return false, err
		}
		t2, err := intervals.Clone(&source.columns[source.find(index)])
		if err != nil {
			return false, errors.Join(err, intervals.Del(&t1))
		}
		var next = intervals.Full()
		if widen {
			next, err = intervals.Widen(&t1, &t2)
		} else {
			next, err = intervals.Unite(&t1, &t2)
		}
		// elems->push_back(mk_unite(t1, t2)): the pushed copy replaces the
		// full placeholder; t2 and t1 die at the end of the iteration.
		if err == nil {
			err = intervals.Set(&result.columns[i], &next)
			err = errors.Join(err, intervals.Del(&next))
		}
		if err = errors.Join(err, intervals.Del(&t2), intervals.Del(&t1)); err != nil {
			return false, err
		}
		equal, err := result.columns[i].Equal(old)
		if err != nil {
			return false, err
		}
		if !equal {
			changed = true
		}
	}
	// dealloc(m_elems); m_elems = elems
	err := target.del()
	target.columns, target.parent, target.size = result.columns, result.parent, result.size
	return changed, err
}

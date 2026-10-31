package rel

// Source: the interval instantiation of src/muz/rel/dl_vector_relation.h.

import "github.com/glycerine/g3/pkg/ast"

// joinIntervalColumns follows mk_join: source column values are copied, then
// the requested cross-relation column pairs are equated.
func joinIntervalColumns(left, right *intervalColumns, leftCols, rightCols []uint32) (*intervalColumns, error) {
	if left == nil || right == nil || len(leftCols) != len(rightCols) {
		return nil, ast.ErrASTDomain
	}
	for i := range leftCols {
		if uint64(leftCols[i]) >= uint64(len(left.columns)) || uint64(rightCols[i]) >= uint64(len(right.columns)) {
			return nil, ast.ErrASTDomain
		}
	}
	result := newIntervalColumns(len(left.columns)+len(right.columns), false)
	if left.empty || right.empty {
		result.empty = true
		return result, nil
	}
	for i := range left.columns {
		result.columns[i] = left.columns[left.find(uint32(i))]
	}
	for i := range right.columns {
		result.columns[len(left.columns)+i] = right.columns[right.find(uint32(i))]
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
func projectIntervalColumns(source *intervalColumns, removed []uint32) (*intervalColumns, error) {
	if source == nil || len(removed) > len(source.columns) {
		return nil, ast.ErrASTDomain
	}
	for i, column := range removed {
		if uint64(column) >= uint64(len(source.columns)) || i > 0 && removed[i-1] >= column {
			return nil, ast.ErrASTDomain
		}
	}
	result := newIntervalColumns(len(source.columns)-len(removed), false)
	representatives := make(map[uint32]uint32)
	removedIndex, dst := 0, uint32(0)
	for i := uint32(0); uint64(i) < uint64(len(source.columns)); i++ {
		if removedIndex < len(removed) && removed[removedIndex] == i {
			removedIndex++
			continue
		}
		root := source.find(i)
		result.columns[dst] = source.columns[root]
		if first, ok := representatives[root]; ok {
			result.merge(first, dst)
		} else {
			representatives[root] = dst
		}
		dst++
	}
	return result, nil
}

// unionIntervalColumns updates the target with the source's hull or widening.
// The result reports the source's delta-change condition.
func unionIntervalColumns(target, source *intervalColumns, widen bool) (bool, error) {
	if target == nil || source == nil || len(target.columns) != len(source.columns) {
		return false, ast.ErrASTDomain
	}
	if source.empty {
		return false, nil
	}
	if target.empty {
		// vector_relation::copy copies the values through the source's
		// representatives but initializes fresh destination equivalences.
		target.setEmpty()
		target.empty = false
		for i := range target.columns {
			target.columns[i] = source.columns[source.find(uint32(i))]
		}
		return true, nil
	}
	result := newIntervalColumns(len(target.columns), false)
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
		old, next := target.columns[target.find(index)], source.columns[source.find(index)]
		if widen {
			result.columns[i] = old.Widen(next)
		} else {
			result.columns[i] = old.Hull(next)
		}
		if !result.columns[i].Equal(old) {
			changed = true
		}
	}
	target.columns, target.parent, target.size = result.columns, result.parent, result.size
	return changed, nil
}

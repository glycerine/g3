package rel

// Source: src/muz/rel/dl_vector_relation.h mk_join/mk_project and
// dl_bound_relation.cpp bound_relation_helper::mk_project_t.

import "github.com/glycerine/g3/pkg/ast"

// JoinBoundRelations copies the source column bounds and equates the selected
// cross-relation columns, following the concrete vector relation join.
func JoinBoundRelations(left, right *BoundRelation, leftColumns, rightColumns []uint32) (*BoundRelation, error) {
	if left == nil || right == nil || len(leftColumns) != len(rightColumns) {
		return nil, ast.ErrASTDomain
	}
	for i := range leftColumns {
		if uint64(leftColumns[i]) >= uint64(len(left.columns)) || uint64(rightColumns[i]) >= uint64(len(right.columns)) {
			return nil, ast.ErrASTDomain
		}
	}
	result := NewBoundRelation(uint32(len(left.columns)+len(right.columns)), false)
	if left.empty || right.empty {
		result.empty = true
		return result, nil
	}
	for i := range left.columns {
		result.columns[i] = cloneBoundColumn(left.columns[left.find(uint32(i))])
	}
	for i := range right.columns {
		result.columns[len(left.columns)+i] = cloneBoundColumn(right.columns[right.find(uint32(i))])
	}
	for i := range leftColumns {
		if err := result.Equate(leftColumns[i], uint32(len(left.columns))+rightColumns[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Project removes sorted, unique columns and maps bounds to the surviving
// columns. Bounds naming removed columns are dropped.
func (r *BoundRelation) Project(removed []uint32) (*BoundRelation, error) {
	if r == nil || len(removed) > len(r.columns) {
		return nil, ast.ErrASTDomain
	}
	for i, col := range removed {
		if uint64(col) >= uint64(len(r.columns)) || i > 0 && removed[i-1] >= col {
			return nil, ast.ErrASTDomain
		}
	}
	result := NewBoundRelation(uint32(len(r.columns)-len(removed)), false)
	if r.empty {
		result.reset()
		return result, nil
	}
	kept := make([]uint32, 0, len(result.columns))
	classes := make(map[uint32]uint32)
	removeIndex := 0
	for i := range r.columns {
		index := uint32(i)
		if removeIndex < len(removed) && removed[removeIndex] == index {
			removeIndex++
			continue
		}
		dest := uint32(len(kept))
		kept = append(kept, index)
		root := r.find(index)
		result.columns[dest] = cloneBoundColumn(r.columns[root])
		if first, ok := classes[root]; ok {
			result.merge(first, dest)
		} else {
			classes[root] = dest
		}
	}
	renaming := make(map[uint32]uint32, len(kept))
	for i, old := range kept {
		renaming[old] = result.find(uint32(i))
	}
	// The source applies the projection helper through operator[] for every
	// column, including repeated representatives.
	for i := range result.columns {
		column := &result.columns[result.find(uint32(i))]
		mapped := boundColumn{lt: make(map[uint32]bool), le: make(map[uint32]bool)}
		for old := range column.lt {
			if next, ok := renaming[old]; ok {
				mapped.lt[next] = true
			}
		}
		for old := range column.le {
			if next, ok := renaming[old]; ok {
				mapped.le[next] = true
			}
		}
		*column = mapped
	}
	return result, nil
}

package rel

// Source: src/muz/rel/dl_bound_relation.cpp mk_unite/mk_widen/mk_eq and
// src/muz/rel/dl_vector_relation.h mk_union for the concrete bound type.

import "github.com/glycerine/g3/pkg/ast"

func cloneBoundColumn(c boundColumn) boundColumn {
	result := boundColumn{lt: make(map[uint32]bool), le: make(map[uint32]bool)}
	for index := range c.lt {
		result.lt[index] = true
	}
	for index := range c.le {
		result.le[index] = true
	}
	return result
}

// copyFrom is vector_relation::copy: an empty source is set_empty (fresh
// equalities); otherwise (*this)[i] = other[i] writes each source
// representative's payload through the destination's own m_eqs, so a
// destination class keeps the last column's payload. The second loop's
// merge(i, find(i)) merges a column with its own representative: the
// destination's equalities are unchanged.
func (r *BoundRelation) copyFrom(source *BoundRelation) {
	if source.empty {
		r.reset()
		return
	}
	r.empty = false
	for i := range r.columns {
		r.columns[r.find(uint32(i))] = cloneBoundColumn(source.columns[source.find(uint32(i))])
	}
}

func sameBoundColumn(a, b boundColumn) bool {
	if len(a.lt) != len(b.lt) || len(a.le) != len(b.le) {
		return false
	}
	for index := range a.lt {
		if !b.lt[index] {
			return false
		}
	}
	for index := range a.le {
		if !b.le[index] {
			return false
		}
	}
	return true
}

func (r *BoundRelation) merge(left, right uint32) {
	left, right = r.find(left), r.find(right)
	if left == right {
		return
	}
	if r.size[left] > r.size[right] {
		left, right = right, left
	}
	r.parent[left] = right
	r.size[right] += r.size[left]
}

// remap expands each old equivalence class into the joint equivalence classes.
func (r *BoundRelation) remap(c boundColumn, joint *BoundRelation) boundColumn {
	result := boundColumn{lt: make(map[uint32]bool), le: make(map[uint32]bool)}
	for index := range c.lt {
		// bound_relation::mk_eq visits only source columns [0, arity), so
		// projection's UINT_MAX sentinel is not copied into the union result.
		if uint64(index) >= uint64(len(r.columns)) {
			continue
		}
		for j := range r.columns {
			if r.find(uint32(j)) == r.find(index) {
				result.lt[joint.find(uint32(j))] = true
			}
		}
	}
	for index := range c.le {
		if uint64(index) >= uint64(len(r.columns)) {
			continue
		}
		for j := range r.columns {
			if r.find(uint32(j)) == r.find(index) {
				result.le[joint.find(uint32(j))] = true
			}
		}
	}
	return result
}

// Union keeps only bounds and equivalences shared by both inputs. The source
// widening operation uses the same concrete bound-set union.
func (r *BoundRelation) Union(source *BoundRelation) (bool, error) {
	if r == nil || source == nil || len(r.columns) != len(source.columns) {
		return false, ast.ErrASTDomain
	}
	if source.empty {
		return false, nil
	}
	if r.empty {
		r.copyFrom(source)
		return true, nil
	}
	joint := NewBoundRelation(uint32(len(r.columns)), false)
	type roots struct{ left, right uint32 }
	classes := make(map[roots]uint32)
	seenLeft := make(map[uint32]bool)
	changed := false
	for i := range r.columns {
		index := uint32(i)
		key := roots{r.find(index), source.find(index)}
		if first, ok := classes[key]; ok {
			joint.merge(index, first)
		} else {
			classes[key] = index
			if seenLeft[key.left] {
				changed = true
			}
			seenLeft[key.left] = true
		}
	}
	for i := range r.columns {
		index := uint32(i)
		old := r.columns[r.find(index)]
		left := r.remap(old, joint)
		right := source.remap(source.columns[source.find(index)], joint)
		for j := range left.lt {
			if !right.lt[j] {
				delete(left.lt, j)
			}
		}
		for j := range left.le {
			if !right.le[j] {
				delete(left.le, j)
			}
		}
		joint.columns[i] = left
		if !sameBoundColumn(left, old) {
			changed = true
		}
	}
	r.columns, r.parent, r.size = joint.columns, joint.parent, joint.size
	return changed, nil
}

func (r *BoundRelation) Widen(source *BoundRelation) (bool, error) { return r.Union(source) }

// UnionWithDelta is vector_relation::mk_union with a delta: an empty source
// is copied into delta, a union into an empty target copies the source into
// both, and otherwise delta gets a copy of the target when it changed.
func (r *BoundRelation) UnionWithDelta(source, delta *BoundRelation, widen bool) (bool, error) {
	if r == nil || source == nil || delta == nil || r == delta || source == delta || len(r.columns) != len(source.columns) || len(r.columns) != len(delta.columns) {
		return false, ast.ErrASTDomain
	}
	wasEmpty := r.empty
	changed, err := r.Union(source)
	if err != nil {
		return false, err
	}
	if source.empty || wasEmpty {
		delta.copyFrom(source)
	} else if changed {
		delta.copyFrom(r)
	}
	return changed, nil
}

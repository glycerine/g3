package rel

import "github.com/glycerine/g3/pkg/ast"

// swap follows vector_relation::swap for the concrete interval
// instantiation. The source relation manager calls it only after can_swap has
// established equal signatures; the concrete Go boundary checks equal arity.
func (r *IntervalRelation) swap(other *IntervalRelation) error {
	if r == nil || other == nil || r.values == nil || other.values == nil || len(r.values.columns) != len(other.values.columns) {
		return ast.ErrASTDomain
	}
	if r != other {
		r.values, other.values = other.values, r.values
	}
	return nil
}

// FilterByIntersection implements relation_manager's default intersection
// filter for interval vector relations: join target and source, project every
// source column, then swap the filtered relation into the target.
func (r *IntervalRelation) FilterByIntersection(source *IntervalRelation, targetColumns, sourceColumns []uint32) error {
	if r == nil || source == nil || r.values == nil || source.values == nil || len(targetColumns) != len(sourceColumns) {
		return ast.ErrASTDomain
	}
	joined, err := JoinIntervalRelations(r, source, targetColumns, sourceColumns)
	if err != nil {
		return err
	}
	targetArity := len(r.values.columns)
	removed := make([]uint32, len(source.values.columns))
	for index := range removed {
		removed[index] = uint32(targetArity + index)
	}
	filtered, err := joined.Project(removed)
	if err != nil {
		return err
	}
	return r.swap(filtered)
}

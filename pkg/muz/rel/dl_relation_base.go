package rel

// Source: src/muz/rel/dl_base.h relation_base / relation_plugin
// (tr_infrastructure<relation_traits>::base_object and plugin_object) and
// the relation_*_fn functor families.
//
// g3's execution context, relation manager and compiler hold FiniteTable
// values. A table_relation (dl_table_relation.h) is represented by its
// table itself: the FiniteTable implementations (hashtable, sparse,
// bitvector, lazy_sparse, check tables) and the table operations of
// instruction_finite_table.go are table_relation_plugin's operations
// forwarded to the table plugins. Every other relation plugin's relations
// implement RelationBase, which is also a FiniteTable so that registers,
// load/store and the relation map hold them unchanged; the instructions
// dispatch them through the relation-level relation_manager::mk_*_fn
// (dl_relation_manager_relation.go).

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// ErrRelationNotTable is returned by FiniteTable operations that only a
// table has (row enumeration) when they are asked of a relation_base.
var ErrRelationNotTable = errors.New("muz/rel: relation is not a table")

// RelationBase is relation_base for relations that are not table_relations.
// Signature() is the table signature of the column sorts when every sort is
// a bounded finite domain; Len() is get_size_estimate_rows() as the
// register-emptiness and size reports use it; Facts() is unsupported.
// Add(Fact) is add_fact of the fact whose relation elements are the table
// elements' numerals (relation_manager::table_fact_to_relation).
type RelationBase interface {
	FiniteTable
	// RelationSignature is get_signature(): the column sorts (borrowed).
	RelationSignature() []ast.ASTHandle
	// RelationPlugin is get_plugin().
	RelationPlugin() RelationPlugin
	// Empty is relation_base::empty().
	Empty() (bool, error)
	// FastEmpty is relation_base::fast_empty().
	FastEmpty() bool
	// IsPrecise is relation_base::is_precise().
	IsPrecise() bool
	// ToFormula is relation_base::to_formula(fml); the caller owns the result.
	ToFormula() (ast.ASTRef, error)
	// Display is relation_base::display(out).
	Display(out io.Writer) error
	// SizeEstimateRows and SizeEstimateBytes are get_size_estimate_rows/bytes.
	SizeEstimateRows() (uint32, error)
	SizeEstimateBytes() (uint32, error)
	// CloneRelation is clone().
	CloneRelation() (RelationBase, error)
}

// Relation operation functors (relation_join_fn, relation_transformer_fn,
// relation_mutator_fn, relation_union_fn, relation_intersection_filter_fn).
// A plugin's mk_*_fn returns nil when it does not implement the operation
// for its arguments.
type (
	RelationJoinFn               func(r1, r2 RelationBase) (RelationBase, error)
	RelationTransformerFn        func(r RelationBase) (RelationBase, error)
	RelationMutatorFn            func(r RelationBase) error
	RelationUnionFn              func(target, source, delta RelationBase) error
	RelationIntersectionFilterFn func(target, other RelationBase) error
)

// RelationPlugin is relation_plugin. Column arguments are column indexes,
// as in the C++ (cnt, cols) pairs.
type RelationPlugin interface {
	Name() string
	Manager() *RelationManager
	CanHandleSignature(sorts []ast.ASTHandle) (bool, error)
	MkEmpty(sorts []ast.ASTHandle) (RelationBase, error)
	// MkFull is mk_full(p, s); predicate may be null.
	MkFull(predicate ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error)

	MkJoinFn(t1, t2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error)
	MkProjectFn(t RelationBase, removed []int) (RelationTransformerFn, error)
	MkJoinProjectFn(t1, t2 RelationBase, cols1, cols2, removed []int) (RelationJoinFn, error)
	MkRenameFn(t RelationBase, cycle []int) (RelationTransformerFn, error)
	MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error)
	MkWidenFn(target, source, delta RelationBase) (RelationUnionFn, error)
	MkFilterIdenticalFn(t RelationBase, cols []int) (RelationMutatorFn, error)
	MkFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error)
	MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error)
	MkFilterInterpretedAndProjectFn(t RelationBase, condition ast.ASTHandle, removed []int) (RelationTransformerFn, error)
	MkSelectEqualAndProjectFn(t RelationBase, value ast.ASTHandle, col int) (RelationTransformerFn, error)
	MkFilterByIntersectionFn(t, src RelationBase, tCols, srcCols []int) (RelationIntersectionFilterFn, error)
	MkFilterByNegationFn(t, neg RelationBase, tCols, negCols []int) (RelationIntersectionFilterFn, error)
}

// RelationPluginDefaults is relation_plugin's base-class behavior: every
// operation is unsupported (returns nil). Plugins embed it and override the
// operations they implement.
type RelationPluginDefaults struct{}

func (RelationPluginDefaults) MkJoinFn(RelationBase, RelationBase, []int, []int) (RelationJoinFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkProjectFn(RelationBase, []int) (RelationTransformerFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkJoinProjectFn(RelationBase, RelationBase, []int, []int, []int) (RelationJoinFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkRenameFn(RelationBase, []int) (RelationTransformerFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkUnionFn(RelationBase, RelationBase, RelationBase) (RelationUnionFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkWidenFn(RelationBase, RelationBase, RelationBase) (RelationUnionFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkFilterIdenticalFn(RelationBase, []int) (RelationMutatorFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkFilterEqualFn(RelationBase, ast.ASTHandle, int) (RelationMutatorFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkFilterInterpretedFn(RelationBase, ast.ASTHandle) (RelationMutatorFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkFilterInterpretedAndProjectFn(RelationBase, ast.ASTHandle, []int) (RelationTransformerFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkSelectEqualAndProjectFn(RelationBase, ast.ASTHandle, int) (RelationTransformerFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkFilterByIntersectionFn(RelationBase, RelationBase, []int, []int) (RelationIntersectionFilterFn, error) {
	return nil, nil
}
func (RelationPluginDefaults) MkFilterByNegationFn(RelationBase, RelationBase, []int, []int) (RelationIntersectionFilterFn, error) {
	return nil, nil
}

// AsRelation reports whether a register value is a relation_base of a
// non-table plugin.
func AsRelation(table FiniteTable) (RelationBase, bool) {
	if table == nil {
		return nil, false
	}
	r, ok := table.(RelationBase)
	return r, ok
}

// joinSorts is the result signature of convenient_relation_join_fn.
func joinSorts(s1, s2 []ast.ASTHandle) []ast.ASTHandle {
	return append(append(make([]ast.ASTHandle, 0, len(s1)+len(s2)), s1...), s2...)
}

// projectSorts is convenient_relation_project_fn's result signature
// (relation_signature::from_project); removed is strictly increasing.
func projectSorts(s []ast.ASTHandle, removed []int) []ast.ASTHandle {
	result := make([]ast.ASTHandle, 0, len(s))
	next := 0
	for i, sort := range s {
		if next < len(removed) && removed[next] == i {
			next++
			continue
		}
		result = append(result, sort)
	}
	return result
}

// renameSorts is relation_signature::from_rename: permutate_by_cycle.
func renameSorts(s []ast.ASTHandle, cycle []int) []ast.ASTHandle {
	result := append([]ast.ASTHandle(nil), s...)
	if len(cycle) < 2 {
		return result
	}
	first := result[cycle[0]]
	for i := 0; i+1 < len(cycle); i++ {
		result[cycle[i]] = result[cycle[i+1]]
	}
	result[cycle[len(cycle)-1]] = first
	return result
}

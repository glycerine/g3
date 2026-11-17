package rel

// Source: src/muz/rel/dl_lazy_table.cpp lazy_table_filter_interpreted and the
// filter-interpreted/project fusion in lazy_table_project::force.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

type lazySparseInterpretedFilter struct {
	owner         *lazySparseConditionOwner
	manager       *RelationManager
	rewriter      *astrewriter.TheoryRewriter
	relationSorts []ast.ASTHandle
	columns       map[uint32]int
	sorts         map[uint32]ast.ASTHandle
}

func (f *lazySparseInterpretedFilter) conditions() ([]ast.ASTHandle, error) {
	if f == nil || f.owner == nil || f.owner.manager == nil || len(f.owner.refs) == 0 || f.rewriter == nil {
		return nil, ErrLazySparse
	}
	if f.manager == nil || f.manager.open() != nil || f.rewriter.Manager() != f.owner.manager {
		return nil, ErrLazySparse
	}
	result := make([]ast.ASTHandle, len(f.owner.refs))
	for index := range f.owner.refs {
		handle, err := f.owner.manager.RefGet(f.owner.refs[index])
		if err != nil {
			return nil, err
		}
		result[index] = handle
	}
	return result, nil

}

func (f *lazySparseInterpretedFilter) condition() (ast.ASTHandle, error) {
	conditions, err := f.conditions()
	if err != nil || len(conditions) != 1 {
		return ast.ASTHandle{}, ErrLazySparse
	}
	return conditions[0], nil
}

func validateLazySparseCondition(manager *RelationManager, signature Signature, relationSorts []ast.ASTHandle, rewriter *astrewriter.TheoryRewriter, condition ast.ASTHandle) error {
	if manager == nil || manager.open() != nil || rewriter == nil || rewriter.Manager() != manager.manager {
		return ErrLazySparse
	}
	expected, err := tableRelationSignature(manager, relationSorts)
	if err != nil {
		return err
	}
	if !expected.equal(signature) {
		return ErrLazySparse
	}
	kind, err := manager.manager.GetKind(condition)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return fmt.Errorf("%w: interpreted condition must be an application", ErrLazySparse)
	}
	boolean, err := manager.manager.MkBoolSort()
	if err != nil {
		return err
	}
	sort, err := manager.manager.GetSort(condition)
	if err != nil {
		return err
	}
	if sort != boolean {
		return fmt.Errorf("%w: interpreted condition is not Boolean", ErrLazySparse)
	}
	used, err := ast.NewASTUsedVars(manager.manager)
	if err != nil {
		return err
	}
	if err = used.Collect(condition); err != nil {
		return err
	}
	if used.MaxFoundVarIndexPlusOne() > uint32(len(relationSorts)) {
		return ErrLazySparse
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		variableSort := used.Contains(variable)
		if variableSort != (ast.ASTHandle{}) && variableSort != relationSorts[variable] {
			return fmt.Errorf("%w: interpreted variable sort mismatch", ErrLazySparse)
		}
	}
	return nil
}

// FilterInterpreted appends a deferred source condition and owns its AST
// reference until the final node owner closes successfully.
func (t *LazySparseTable) FilterInterpreted(manager *RelationManager, relationSorts []ast.ASTHandle, rewriter *astrewriter.TheoryRewriter, condition ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := validateLazySparseCondition(manager, t.root.signature, relationSorts, rewriter, condition); err != nil {
		return err
	}
	owner := &lazySparseConditionOwner{engine: t.engine, manager: manager.manager}
	t.engine.conditions = append(t.engine.conditions, owner)
	reference, err := manager.manager.NewRef(condition)
	if err != nil {
		_, cleanupErr := owner.release()
		return errors.Join(err, cleanupErr)
	}
	owner.refs = append(owner.refs, reference)
	payload := &lazySparseInterpretedFilter{
		owner: owner, manager: manager, rewriter: rewriter, relationSorts: append([]ast.ASTHandle(nil), relationSorts...),
	}
	if err = t.replaceLazySparseRoot(&lazySparseNode{
		kind: LazyTableFilterInterpreted, signature: Signature{domains: t.root.signature.Domains()}, functional: t.root.functional,
		left: t.root, interpreted: payload,
	}); err != nil {
		_, cleanupErr := owner.release()
		return errors.Join(err, cleanupErr)
	}
	return nil
}

func evalLazySparseInterpreted(engine *LazySparseEngine, node *lazySparseNode) (FiniteTable, error) {
	result, err := engine.transferLazySparseSource(node, &node.left)
	if err != nil {
		return nil, err
	}
	err = lazyVerbose("filter_interpreted", func() error {
		return filterInterpretedForced(result, node.interpreted)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// filterInterpretedForced is rm().mk_filter_interpreted_fn of the forced
// table: the sparse plugin's filter, or the table relation's row filter of
// a hash table (the favourite table of a nullary signature or a default
// join's result).
func filterInterpretedForced(table FiniteTable, f *lazySparseInterpretedFilter) error {
	switch concrete := table.(type) {
	case *SparseTable:
		if f.columns != nil || f.sorts != nil {
			conditions, err := f.conditions()
			if err != nil {
				return err
			}
			return filterSparseInterpretedBound(concrete, f.manager, f.rewriter, conditions, f.columns, f.sorts)
		}
		condition, err := f.condition()
		if err != nil {
			return err
		}
		return concrete.FilterInterpreted(f.manager, f.relationSorts, f.rewriter, condition)
	case *HashTable:
		sorts := f.relationSorts
		if concrete.signature.Len() == 0 {
			sorts = nil
		}
		relation := &TableRelation{manager: f.manager, sorts: sorts, table: concrete}
		conditions, err := f.conditions()
		if err != nil {
			return err
		}
		columns, varSorts := f.columns, f.sorts
		if columns == nil && varSorts == nil {
			columns, varSorts = map[uint32]int{}, map[uint32]ast.ASTHandle{}
		}
		return relation.FilterInterpreted(f.rewriter, conditions, columns, varSorts)
	}
	return ErrLazySparse
}

func evalLazySparseInterpretedProject(engine *LazySparseEngine, filter *lazySparseNode, removed []int) (FiniteTable, error) {
	// Evaluate the filter node itself: mutating force transfers the table and
	// clears its child, while retryForce keeps an incomplete result retryable.
	source, err := engine.eval(filter)
	if err != nil {
		return nil, err
	}
	var result FiniteTable
	err = lazyVerbose("filter_interpreted_project", func() (x error) {
		result, x = filterInterpretedProjectForced(engine.manager, source, filter, removed)
		return x
	})
	return result, err
}

// filterInterpretedProjectForced is the filter_interpreted_and_project of
// lazy_table_project::force on the forced filter node's table: the sparse
// plugin's, else default_table_filter_interpreted_and_project_fn (a clone,
// its filter, then mk_project_fn).
func filterInterpretedProjectForced(m *RelationManager, table FiniteTable, filter *lazySparseNode, removed []int) (FiniteTable, error) {
	source, sparse := table.(*SparseTable)
	if sparse && filter.interpreted.columns == nil && filter.interpreted.sorts == nil {
		signature, err := source.Signature()
		if err != nil {
			return nil, err
		}
		if projected, err := projectedSignature(signature, removed); err == nil && projected.Len() != 0 {
			condition, conditionErr := filter.interpreted.condition()
			if conditionErr != nil {
				return nil, conditionErr
			}
			return source.FilterInterpretedAndProject(filter.interpreted.manager, filter.interpreted.relationSorts, filter.interpreted.rewriter, condition, removed)
		}
	}
	clone, err := table.CloneFinite()
	if err != nil {
		return nil, err
	}
	if err = filterInterpretedForced(clone, filter.interpreted); err != nil {
		return nil, errors.Join(err, releaseUnallocated(clone))
	}
	projected, projectErr := finiteTableProjectIn(m, clone, removed)
	// default_table_filter_interpreted_and_project_fn never deallocates
	// its clone t2 (no recycle line).
	closeErr := releaseUnallocated(clone)
	if projectErr != nil || closeErr != nil {
		if projected != nil {
			closeErr = errors.Join(closeErr, projected.Close())
		}
		return nil, errors.Join(projectErr, closeErr)
	}
	return projected, nil
}

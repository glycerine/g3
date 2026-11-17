package rel

import "errors"

// lazySparseInstructionResult constructs compiler temporaries in the same
// engine as an operand. Explicit lazy-sparse predicates are non-nullary, while
// compiler projections may use the internal Hash-backed nullary node.
func lazySparseInstructionResult(source *LazySparseTable, signature Signature, functional int) (*LazySparseTable, error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	if signature.Len() != 0 {
		return source.engine.New(signature, functional)
	}
	return source.engine.adopt(&lazySparseNode{
		kind: LazyTableBase, signature: Signature{domains: signature.Domains()}, nullary: NewHashTable(signature),
	}), nil
}

// lazySparseInstructionJoinProject is relation_manager::mk_join_project_fn
// on lazy tables: the lazy plugin has no join_project, so it is
// default_table_join_project_fn: the lazy join, then mk_project_fn, or
// mk_project_with_reduce_fn (default_table_project_with_reduce_fn, whose
// reducer is unreachable) when table_signature::from_join_project keeps
// functional columns.
func lazySparseInstructionJoinProject(first, second *LazySparseTable, left, right, removed []int) (_ *LazySparseTable, resultErr error) {
	if err := first.open(); err != nil {
		return nil, err
	}
	if err := second.open(); err != nil {
		return nil, err
	}
	functional := 0
	if first.root.functional != 0 || second.root.functional != 0 {
		var err error
		functional, err = sparseJoinProjectFunctionalCountSignatures(first.root.signature, first.root.functional, second.root.signature, second.root.functional, left, right, removed)
		if err != nil {
			return nil, err
		}
	}
	joined, err := first.Join(second, left, right)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, joined.Close()) }()
	if functional == 0 {
		return joined.Project(removed)
	}
	projected, err := fprProjectWithReduceDefault(joined, removed, func(_, _ []Element) {})
	if err != nil {
		return nil, err
	}
	lazy, ok := projected.(*LazySparseTable)
	if !ok {
		return nil, errors.Join(ErrLazySparse, projected.Close())
	}
	return lazy, nil
}

func lazySparseInstructionSelectProject(source *LazySparseTable, value Element, column int) (_ *LazySparseTable, resultErr error) {
	if err := source.open(); err != nil || column < 0 || column >= source.root.signature.Len() || uint64(value) >= source.root.signature.domains[column] {
		return nil, ErrLazySparse
	}
	filtered := source.engine.adopt(&lazySparseNode{
		kind: LazyTableFilterEqual, signature: Signature{domains: source.root.signature.Domains()}, functional: source.root.functional,
		left: source.root, value: value, column: column,
	})
	defer func() { resultErr = errors.Join(resultErr, filtered.Close()) }()
	return filtered.Project([]int{column})
}

package rel

// FilterByNegation appends the source lazy_table_filter_by_negation node
// without forcing either packed sparse operand.
func (t *LazySparseTable) FilterByNegation(source *LazySparseTable, targetColumns, sourceColumns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || source.engine != t.engine {
		return ErrLazySparse
	}
	if err := validateColumns(t.root.signature, source.root.signature, targetColumns, sourceColumns); err != nil {
		return err
	}
	return t.replaceLazySparseRoot(&lazySparseNode{
		kind: LazyTableFilterByNegation, signature: Signature{domains: t.root.signature.Domains()}, functional: t.root.functional,
		left: t.root, right: source.root, columns1: append([]int(nil), targetColumns...), columns2: append([]int(nil), sourceColumns...),
	})
}

func evalLazySparseNegation(engine *LazySparseEngine, node *lazySparseNode) (FiniteTable, error) {
	target, err := engine.transferLazySparseSource(node, &node.left)
	if err != nil {
		return nil, err
	}
	// lazy_table_filter_by_negation::force: a join source's operands are
	// evaluated and verbose_action("filter_by_negation_join") runs whether
	// or not rm().mk_filter_by_negated_join_fn has a function (the sparse
	// plugin's negated join, for sparse tables without functional columns
	// whose plan applies).
	if node.right.kind == LazyTableJoin {
		first, firstErr := engine.eval(node.right.left)
		if firstErr != nil {
			return nil, firstErr
		}
		second, secondErr := engine.eval(node.right.right)
		if secondErr != nil {
			return nil, secondErr
		}
		applied := false
		err = lazyVerbose("filter_by_negation_join", func() error {
			t, ok1 := target.(*SparseTable)
			a, ok2 := first.(*SparseTable)
			b, ok3 := second.(*SparseTable)
			if !ok1 || !ok2 || !ok3 || node.right.left.functional != 0 || node.right.right.functional != 0 {
				return nil
			}
			plan, planErr := NewSparseNegatedJoinPlan(t, a, b, node.columns1, node.columns2, node.right.columns1, node.right.columns2)
			if planErr != nil {
				return nil
			}
			if _, planErr = plan.Apply(t, a, b); planErr != nil {
				return planErr
			}
			applied = true
			return nil
		})
		if err != nil {
			return nil, err
		}
		if applied {
			return target, nil
		}
	}
	negative, err := engine.eval(node.right)
	if err != nil {
		return nil, err
	}
	if err = lazyVerbose("filter_by_negation", func() error {
		return finiteTableFilterNegation(target, negative, node.columns1, node.columns2)
	}); err != nil {
		return nil, err
	}
	return target, nil
}

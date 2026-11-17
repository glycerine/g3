package rel

func lazySparseJoinMetadata(left, right *lazySparseNode) (Signature, int, error) {
	leftFirst := left.signature.Len() - left.functional
	rightFirst := right.signature.Len() - right.functional
	domains := make([]uint64, 0, left.signature.Len()+right.signature.Len())
	domains = append(domains, left.signature.domains[:leftFirst]...)
	domains = append(domains, right.signature.domains[:rightFirst]...)
	domains = append(domains, left.signature.domains[leftFirst:]...)
	domains = append(domains, right.signature.domains[rightFirst:]...)
	signature, err := NewSignature(domains)
	return signature, left.functional + right.functional, err
}

func lazySparseProjectMetadata(source *lazySparseNode, removed []int) (Signature, int, error) {
	signature, err := projectedSignature(source.signature, removed)
	if err != nil {
		return Signature{}, 0, err
	}
	functional := source.functional
	if len(removed) != 0 && removed[0] < source.signature.Len()-source.functional {
		functional = 0
	} else {
		functional -= len(removed)
	}
	return signature, functional, nil
}

func (t *LazySparseTable) Join(other *LazySparseTable, leftColumns, rightColumns []int) (*LazySparseTable, error) {
	if err := t.open(); err != nil || other == nil || other.open() != nil || t.engine != other.engine {
		return nil, ErrLazySparse
	}
	if err := validateColumns(t.root.signature, other.root.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	signature, functional, err := lazySparseJoinMetadata(t.root, other.root)
	if err != nil {
		return nil, err
	}
	return t.engine.adopt(&lazySparseNode{kind: LazyTableJoin, signature: signature, functional: functional, left: t.root, right: other.root, columns1: append([]int(nil), leftColumns...), columns2: append([]int(nil), rightColumns...)}), nil
}

func (t *LazySparseTable) Project(removed []int) (*LazySparseTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	signature, functional, err := lazySparseProjectMetadata(t.root, removed)
	if err != nil {
		return nil, err
	}
	return t.engine.adopt(&lazySparseNode{kind: LazyTableProject, signature: signature, functional: functional, left: t.root, columns1: append([]int(nil), removed...)}), nil
}

// eval is lazy_table_ref::eval: the node's m_table, forced on first use.
// Each force runs the relation manager's operation on the forced operand
// tables, whatever plugin made them (rm().mk_join_fn and so on).
func (e *LazySparseEngine) eval(node *lazySparseNode) (FiniteTable, error) {
	if err := e.open(); err != nil || node == nil {
		return nil, ErrLazySparse
	}
	if node.table != nil && !node.retryForce {
		return node.table, nil
	}
	e.stats.Forces++
	var result FiniteTable
	var err error
	switch node.kind {
	case LazyTableJoin:
		// lazy_table_join::force
		left, leftErr := e.eval(node.left)
		if leftErr != nil {
			return nil, leftErr
		}
		right, rightErr := e.eval(node.right)
		if rightErr != nil {
			return nil, rightErr
		}
		e.stats.Joins++
		err = lazyVerbose("join", func() (x error) {
			result, x = finiteTableJoin(left, right, node.columns1, node.columns2)
			return x
		})
	case LazyTableProject:
		result, err = e.forceProject(node)
	case LazyTableRename:
		source, sourceErr := e.eval(node.left)
		if sourceErr != nil {
			return nil, sourceErr
		}
		err = lazyVerbose("rename", func() (x error) {
			result, x = finiteTableRename(source, node.columns1)
			return x
		})
	case LazyTableFilterEqual:
		result, err = evalLazySparseEqual(e, node)
		if err == nil {
			e.stats.EqualFilters++
		}
	case LazyTableFilterIdentical:
		result, err = evalLazySparseIdentical(e, node)
		if err == nil {
			e.stats.IdenticalFilters++
		}
	case LazyTableFilterInterpreted:
		result, err = evalLazySparseInterpreted(e, node)
		if err == nil {
			e.stats.InterpretedFilters++
		}
	case LazyTableFilterByNegation:
		result, err = evalLazySparseNegation(e, node)
		if err == nil {
			e.stats.NegationFilters++
		}
	default:
		return nil, ErrLazySparse
	}
	if err != nil {
		return nil, err
	}
	node.table = result
	node.retryForce = false
	return result, nil
}

// forceProject is lazy_table_project::force: a join source is joined and
// projected by rm().mk_join_project_fn, a filter_interpreted source by
// mk_filter_interpreted_and_project_fn, a filter_equal source by
// mk_select_equal_and_project_fn, any other by mk_project_fn of the
// forced source.
func (e *LazySparseEngine) forceProject(node *lazySparseNode) (result FiniteTable, err error) {
	src := node.left
	if src == nil {
		return nil, ErrLazySparse
	}
	switch {
	case src.kind == LazyTableJoin:
		left, err := e.eval(src.left)
		if err != nil {
			return nil, err
		}
		right, err := e.eval(src.right)
		if err != nil {
			return nil, err
		}
		e.stats.JoinProjects++
		err = lazyVerbose("join_project", func() (x error) {
			result, x = finiteTableJoinProjectIn(e.manager, left, right, src.columns1, src.columns2, node.columns1)
			return x
		})
		return result, err
	case src.kind == LazyTableFilterInterpreted:
		result, err = evalLazySparseInterpretedProject(e, src, node.columns1)
		if err == nil {
			e.stats.InterpretedProjects++
		}
		return result, err
	case src.kind == LazyTableFilterEqual && len(node.columns1) == 1 && node.columns1[0] == src.column:
		result, err = evalLazySparseEqualProject(e, src)
		if err == nil {
			e.stats.SelectEqualProject++
		}
		return result, err
	}
	source, err := e.eval(src)
	if err != nil {
		return nil, err
	}
	e.stats.Projects++
	err = lazyVerbose("project", func() (x error) {
		result, x = finiteTableProjectIn(e.manager, source, node.columns1)
		return x
	})
	return result, err
}

func (t *LazySparseTable) eval() (FiniteTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return t.engine.eval(t.root)
}

func (t *LazySparseTable) finiteEval() (FiniteTable, error) {
	return t.eval()
}

// emptyTable is lazy_table_plugin::mk_empty's table: the sparse plugin's,
// or for a signature it cannot handle (the empty one) the manager's
// mk_empty_table (FORK FIX #15).
func (e *LazySparseEngine) emptyTable(signature Signature, functional int) (FiniteTable, error) {
	if signature.Len() != 0 {
		return e.sparse.MkEmpty(signature, functional)
	}
	if e.manager != nil {
		return e.manager.newFavouriteFiniteTable(signature)
	}
	return NewHashTable(signature), nil
}

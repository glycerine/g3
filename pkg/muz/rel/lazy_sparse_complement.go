package rel

// ComplementFunctional mirrors lazy_table::complement: force the deferred
// source, invoke the concrete table complement with its functional suffix,
// and return a new lazy base owner in the same engine.
func (t *LazySparseTable) ComplementFunctional(functionalSuffix Fact) (*LazySparseTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	table, err := t.eval()
	if err != nil {
		return nil, err
	}
	var complement FiniteTable
	switch concrete := table.(type) {
	case *SparseTable:
		complement, err = concrete.ComplementFunctional(functionalSuffix)
	case *HashTable:
		if len(functionalSuffix) != 0 {
			return nil, ErrLazySparse
		}
		complement, err = concrete.Complement()
	default:
		return nil, ErrLazySparse
	}
	if err != nil {
		return nil, err
	}
	return t.engine.adoptTable(t.root, complement)
}

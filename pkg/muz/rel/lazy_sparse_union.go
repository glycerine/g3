package rel

func (t *LazySparseTable) UnionFrom(source, delta *LazySparseTable) error {
	if err := t.open(); err != nil || source == nil || source.open() != nil || t.engine != source.engine {
		return ErrLazySparse
	}
	if delta != nil && (delta.open() != nil || t.engine != delta.engine) {
		return ErrLazySparse
	}
	if !t.root.signature.equal(source.root.signature) || t.root.functional != source.root.functional ||
		delta != nil && (!t.root.signature.equal(delta.root.signature) || t.root.functional != delta.root.functional) {
		return ErrLazySparse
	}
	// Source lazy union forces source, target, then optional delta.
	sourceTable, err := source.finiteEval()
	if err != nil {
		return err
	}
	targetTable, err := t.finiteEval()
	if err != nil {
		return err
	}
	var deltaTable FiniteTable
	if delta != nil {
		deltaTable, err = delta.finiteEval()
		if err != nil {
			return err
		}
	}
	// union_fn: verbose_action("union"), then rm().mk_union_fn on the
	// forced tables (the sparse plugin's union, or default_table_union_fn
	// when they are of different plugins).
	return lazyVerbose("union", func() error { return finiteTableUnion(targetTable, sourceTable, deltaTable) })
}

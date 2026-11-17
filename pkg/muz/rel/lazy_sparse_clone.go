package rel

import "errors"

// Clone mirrors lazy_table::clone: force the current operation root, clone its
// concrete owner, and wrap the clone as an independently owned lazy base node.
func (t *LazySparseTable) Clone() (*LazySparseTable, error) {
	table, err := t.finiteEval()
	if err != nil {
		return nil, err
	}
	var clone FiniteTable
	if err = lazyVerbose("clone", func() (x error) {
		clone, x = table.CloneFinite()
		return x
	}); err != nil {
		return nil, err
	}
	return t.engine.adoptTable(t.root, clone)
}

// adoptTable is alloc(lazy_table, alloc(lazy_table_base, plugin, table)) of
// a table with like's signature.
func (e *LazySparseEngine) adoptTable(like *lazySparseNode, table FiniteTable) (*LazySparseTable, error) {
	if err := e.open(); err != nil {
		return nil, errors.Join(err, table.Close())
	}
	return e.adopt(&lazySparseNode{kind: LazyTableBase, signature: Signature{domains: like.signature.Domains()}, functional: like.functional, table: table}), nil
}

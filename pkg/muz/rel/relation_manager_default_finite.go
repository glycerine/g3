package rel

import "github.com/glycerine/g3/pkg/ast"

// SetCheckedFiniteFactory borrows a factory for compiler temporaries created
// during one checked query scope. Nil restores ordinary favourite selection.
func (m *RelationManager) SetCheckedFiniteFactory(factory *FiniteCheckedFactory) error {
	if err := m.open(); err != nil {
		return err
	}
	m.checkedFinite = factory
	return nil
}

// SetLazySparseFiniteFactory borrows the context-owned factory used when
// lazy_sparse is the favourite table. The manager closes its table owners
// first; the context closes the factory afterwards.
func (m *RelationManager) SetLazySparseFiniteFactory(factory *LazySparseFiniteFactory) error {
	if err := m.open(); err != nil {
		return err
	}
	if factory != nil {
		if err := factory.open(); err != nil {
			return err
		}
	}
	m.lazyFinite = factory
	return nil
}

// SetDefaultFiniteTable selects the source favourite table plugin used for
// future finite relation and compiler-temporary construction. Existing
// predicate owners are not migrated; explicit predicate selections therefore
// keep precedence over this default.
func (m *RelationManager) SetDefaultFiniteTable(name string) error {
	if err := m.open(); err != nil {
		return err
	}
	switch name {
	case "hashtable", "sparse":
		m.defaultFinite = name
		return nil
	case "lazy_sparse":
		if m.lazyFinite == nil || m.lazyFinite.open() != nil {
			return ErrTable
		}
		m.defaultFinite = name
		return nil
	default:
		return ErrTable
	}
}

// SetRequestedFiniteKind records rel_context::set_predicate_representation's
// requested table plugin for predicate (kind is tr_sparse, tr_hashtable or
// tr_lazy_sparse).
func (m *RelationManager) SetRequestedFiniteKind(predicate ast.ASTHandle, kind string) error {
	if err := m.open(); err != nil {
		return err
	}
	if m.requestedFinite == nil {
		m.requestedFinite = make(map[ast.ASTHandle]string)
	}
	m.requestedFinite[predicate] = kind
	return nil
}

// NewFiniteTableForPredicate is relation_manager::mk_empty_relation(s, pred):
// the predicate's requested plugin when it can handle the signature,
// otherwise the appropriate default plugin.
func (m *RelationManager) NewFiniteTableForPredicate(predicate ast.ASTHandle, signature Signature) (FiniteTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if m.checkedFinite == nil {
		switch m.requestedFinite[predicate] {
		case "tr_hashtable":
			return NewHashTable(signature), nil
		case "tr_sparse":
			if signature.Len() != 0 {
				return NewSparseTable(signature, 0)
			}
		case "tr_lazy_sparse":
			if m.lazyFinite != nil && m.lazyFinite.CanHandleSignature(signature) {
				return m.lazyFinite.New(signature)
			}
		}
	}
	return m.NewDefaultFiniteTable(signature)
}

// NewDefaultFiniteTable mirrors get_appropriate_plugin: the configured sparse
// plugin is preferred when it handles the signature, while the empty
// signature falls through to the always-applicable Hash implementation.
func (m *RelationManager) NewDefaultFiniteTable(signature Signature) (FiniteTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if m.checkedFinite != nil {
		if m.checkedFinite.CanHandleSignature(signature) {
			return m.checkedFinite.New(signature)
		}
		// The source checked favourite rejects a signature if either wrapped
		// plugin rejects it, then registered-plugin selection falls through to
		// the first capable implementation. Hash is that fallback for nullary.
		return NewHashTable(signature), nil
	}
	switch m.defaultFinite {
	case "sparse":
		if signature.Len() != 0 {
			return NewSparseTable(signature, 0)
		}
	case "lazy_sparse":
		if m.lazyFinite == nil {
			return nil, ErrTable
		}
		if m.lazyFinite.CanHandleSignature(signature) {
			return m.lazyFinite.New(signature)
		}
		// Nullary: z3 4.7.1 segfaults creating a 0-column sparse table via
		// lazy_table_plugin::mk_empty (AUDIT.md muz_rel_04-F2); the fixed
		// fork falls back to the manager's plugin choice, Hash, as here.
	}
	return NewHashTable(signature), nil
}

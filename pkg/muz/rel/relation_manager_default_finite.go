package rel

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

// NewDefaultFiniteTable mirrors get_appropriate_plugin: the configured sparse
// plugin is preferred when it handles the signature, while the empty
// signature falls through to the always-applicable Hash implementation.
func (m *RelationManager) NewDefaultFiniteTable(signature Signature) (FiniteTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if m.checkedFinite != nil {
		return m.checkedFinite.New(signature)
	}
	if signature.Len() != 0 {
		switch m.defaultFinite {
		case "sparse":
			return NewSparseTable(signature, 0)
		case "lazy_sparse":
			if m.lazyFinite == nil {
				return nil, ErrTable
			}
			return m.lazyFinite.New(signature)
		}
	}
	return NewHashTable(signature), nil
}

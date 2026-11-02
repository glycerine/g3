package rel

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
	if m.defaultFinite == "sparse" && signature.Len() != 0 {
		return NewSparseTable(signature, 0)
	}
	return NewHashTable(signature), nil
}

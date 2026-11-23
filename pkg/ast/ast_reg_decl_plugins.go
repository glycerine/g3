package ast

// Source: src/ast/reg_decl_plugins.cpp:30-55; src/ast/reg_decl_plugins.h:25.

// RegisterDeclarationPlugins installs the original context's declaration
// plugins in source order. Existing registrations keep their owners and IDs.
// Constructor errors stop the sequence and leave the reached prefix owned by m.
func (m *ASTManager) RegisterDeclarationPlugins() error {
	if err := m.lock(); err != nil {
		return err
	}
	ready := m.declarationPluginsReady && m.Symbols == m.families.symbols
	m.mu.Unlock()
	if ready {
		// Plugin bindings live until manager destruction. The borrowed symbol
		// owner can close independently, including after a successful call.
		return m.Symbols.CheckOpen()
	}
	for _, entry := range [...]struct {
		name      string
		construct func(*ASTManager) (FamilyID, error)
	}{
		{"arith", (*ASTManager).RegisterArithmeticPlugin},
		{"bv", (*ASTManager).RegisterBitVectorPlugin},
		{"array", (*ASTManager).RegisterArrayPlugin},
		{"datatype", (*ASTManager).RegisterDatatypePlugin},
		{"datalog_relation", (*ASTManager).RegisterDatalogPlugin},
		{"seq", (*ASTManager).RegisterSequencePlugin},
		{"fpa", (*ASTManager).RegisterFloatingPointPlugin},
		{"pb", (*ASTManager).RegisterPseudoBooleanPlugin},
	} {
		present, err := m.declarationPluginPresent(entry.name)
		if err != nil {
			return err
		}
		if !present {
			if _, err = entry.construct(m); err != nil {
				return err
			}
		}
	}
	m.declarationPluginsReady = true
	return nil
}

// Source get_plugin(mk_family_id(symbol(name))) creates the family before
// allocating the plugin. Keep that order even when its constructor refuses.
func (m *ASTManager) declarationPluginPresent(name string) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	s, err := m.Symbols.Intern(name)
	if err != nil {
		return false, err
	}
	id, err := m.families.MkID(s)
	if err != nil {
		return false, err
	}
	return m.pluginAt(id).present, nil
}

package ast

// Source: src/ast/arith_decl_plugin.cpp:641-645;
// src/ast/bv_decl_plugin.cpp:850-855.

// ArithmeticFamilyID follows arith_util's constructor. It creates only the
// family identity; the utility's declaration-plugin pointer is initially null.
func (m *ASTManager) ArithmeticFamilyID() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	// Family identities are never unregistered, so the first lookup is final.
	if m.utilFamilies.arithValid {
		return m.utilFamilies.arith, nil
	}
	name, e := m.Symbols.Intern("arith")
	if e != nil {
		return NullFamilyID, e
	}
	id, e := m.families.MkID(name)
	if e == nil {
		m.utilFamilies.arith, m.utilFamilies.arithValid = id, true
	}
	return id, e
}

// astUtilityFamilies caches the families the arith_util and bv_util
// constructors look up by name.
type astUtilityFamilies struct {
	arith, bv, format                FamilyID
	arithValid, bvValid, formatValid bool
}

// BitVectorUtilityFamilyID follows bv_util's reached family lookups and DEBUG
// plugin assertion, independently of installing a declaration plugin.
func (m *ASTManager) BitVectorUtilityFamilyID() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	if m.utilFamilies.bvValid && (!ASTSourceDebug || m.pluginAt(m.utilFamilies.bv).present) {
		return m.utilFamilies.bv, nil
	}
	name, e := m.Symbols.Intern("bv")
	if e != nil {
		return NullFamilyID, e
	}
	family, e := m.families.MkID(name)
	if e != nil {
		return NullFamilyID, e
	}
	if ASTSourceDebug && !m.pluginAt(family).present {
		return NullFamilyID, ErrASTDomain
	}
	// Source repeats mk_family_id while obtaining the borrowed plugin pointer.
	name, e = m.Symbols.Intern("bv")
	if e != nil {
		return NullFamilyID, e
	}
	family, e = m.families.MkID(name)
	if e == nil {
		m.utilFamilies.bv, m.utilFamilies.bvValid = family, true
	}
	return family, e
}

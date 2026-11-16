package rel

// Source: dl_relation_manager.cpp get_relation / store_relation /
// reset_relations. The source creates a predicate's relation lazily, the
// first time get_relation (or store_relation) asks for it, so a relation the
// run never touches is never allocated, stored in m_relations or
// deallocated. Go's datalog engine keeps a table for every declared
// predicate from the start; until the source would have created it, such a
// table is a placeholder: it stays out of the m_relations layout, and is
// released without sparse_table_plugin::recycle's verbose line.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

// ReserveFiniteRelation returns predicate's table, creating an empty
// placeholder when the manager holds none. A placeholder becomes the
// source's relation at the first GetFiniteRelation, StoreFiniteRelation or
// MaterializeFiniteRelation.
func (m *RelationManager) ReserveFiniteRelation(predicate ast.ASTHandle) (FiniteTable, error) {
	table, found, err := m.TryGetFiniteRelation(predicate)
	if err != nil || found {
		return table, err
	}
	_, signature, err := m.PredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	table, err = m.NewFiniteTableForPredicate(predicate, signature)
	if err != nil {
		return nil, err
	}
	if err = m.manager.IncRef(predicate); err != nil {
		return nil, errors.Join(err, table.Close())
	}
	if hash, ok := table.(*HashTable); ok {
		m.relations[predicate] = hash
	} else {
		m.finite[predicate] = table
	}
	if m.placeholders == nil {
		m.placeholders = make(map[ast.ASTHandle]bool)
	}
	m.placeholders[predicate] = true
	return table, nil
}

// MaterializeFiniteRelation is relation_manager::get_relation's creation for
// a predicate whose table is still a placeholder: from here on it is the
// source's relation, in m_relations' insertion order.
func (m *RelationManager) MaterializeFiniteRelation(predicate ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	if !m.placeholders[predicate] {
		return nil
	}
	delete(m.placeholders, predicate)
	return m.noteRelation(predicate)
}

// ReplaceFiniteRelation installs table for predicate. Replacing a placeholder
// (a requested representation chosen before the source creates the relation)
// keeps the new table a placeholder and releases the old one silently;
// otherwise it is StoreFiniteRelation.
func (m *RelationManager) ReplaceFiniteRelation(predicate ast.ASTHandle, table FiniteTable) error {
	if err := m.open(); err != nil {
		return err
	}
	if !m.placeholders[predicate] {
		return m.StoreFiniteRelation(predicate, table)
	}
	if table == nil {
		return ErrTable
	}
	err := m.checkStoredSignature(predicate, table)
	if err != nil {
		return err
	}
	var prior FiniteTable = m.finite[predicate]
	if prior == nil {
		if hash := m.relations[predicate]; hash != nil {
			prior = hash
		}
	}
	if prior == table {
		return nil
	}
	if prior != nil {
		if err = releasePlaceholderTable(prior); err != nil {
			return err
		}
	}
	delete(m.relations, predicate)
	delete(m.finite, predicate)
	if hash, ok := table.(*HashTable); ok {
		m.relations[predicate] = hash
	} else {
		m.finite[predicate] = table
	}
	return nil
}

// releasePlaceholderTable frees a table the source never allocated.
func releasePlaceholderTable(table FiniteTable) error {
	if sparse, ok := table.(*SparseTable); ok {
		return sparse.release()
	}
	return table.Close()
}

// retirePlaceholder releases a placeholder table and the manager's predicate
// reference without any source-visible deallocation.
func (m *RelationManager) retirePlaceholder(predicate ast.ASTHandle) (result error) {
	release := func(table FiniteTable) bool {
		if err := releasePlaceholderTable(table); err != nil {
			result = errors.Join(result, err)
			return false
		}
		_, err := m.manager.DecRefConsumed(predicate)
		result = errors.Join(result, err)
		return true
	}
	if table, ok := m.finite[predicate]; ok && release(table) {
		delete(m.finite, predicate)
	}
	if table, ok := m.relations[predicate]; ok && release(table) {
		delete(m.relations, predicate)
	}
	if !m.holdsRelation(predicate) {
		delete(m.placeholders, predicate)
	}
	return result
}

// retirePlaceholders releases every placeholder for which keep is false.
func (m *RelationManager) retirePlaceholders(keep map[ast.ASTHandle]bool) (result error) {
	for _, predicate := range m.placeholderOrder() {
		if keep != nil && keep[predicate] {
			continue
		}
		result = errors.Join(result, m.retirePlaceholder(predicate))
	}
	return result
}

// placeholderOrder lists the placeholders in a deterministic order; their
// release prints nothing, so any fixed order serves.
func (m *RelationManager) placeholderOrder() []ast.ASTHandle {
	result := make([]ast.ASTHandle, 0, len(m.placeholders))
	for predicate := range m.placeholders {
		result = append(result, predicate)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := finitePredicateKey(result[i]), finitePredicateKey(result[j])
		if a.origin != b.origin {
			return a.origin < b.origin
		}
		if a.slot != b.slot {
			return a.slot < b.slot
		}
		return a.generation < b.generation
	})
	return result
}

// IsPlaceholderRelation reports whether predicate's table is a placeholder
// the source has not created yet.
func (m *RelationManager) IsPlaceholderRelation(predicate ast.ASTHandle) bool {
	return m != nil && m.placeholders[predicate]
}

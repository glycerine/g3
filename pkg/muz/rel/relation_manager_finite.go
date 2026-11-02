package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// TryGetFiniteRelation borrows the actual selected finite representation.
func (m *RelationManager) TryGetFiniteRelation(predicate ast.ASTHandle) (FiniteTable, bool, error) {
	if err := m.open(); err != nil {
		return nil, false, err
	}
	var table FiniteTable = m.finite[predicate]
	if table == nil {
		if hash := m.relations[predicate]; hash != nil {
			table = hash
		}
	}
	if table == nil {
		return nil, false, nil
	}
	if _, err := table.Signature(); err != nil {
		return nil, false, err
	}
	return table, true, nil
}

func (m *RelationManager) GetFiniteRelation(predicate ast.ASTHandle) (FiniteTable, error) {
	table, found, err := m.TryGetFiniteRelation(predicate)
	if err != nil || found {
		return table, err
	}
	_, signature, err := m.PredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	table, err = m.NewDefaultFiniteTable(signature)
	if err != nil {
		return nil, err
	}
	if err = m.StoreFiniteRelation(predicate, table); err != nil {
		return nil, errors.Join(err, table.Close())
	}
	return table, nil
}

// StoreFiniteRelation consumes table only on success. Replacement closes the
// former concrete owner while preserving the manager's predicate reference.
func (m *RelationManager) StoreFiniteRelation(predicate ast.ASTHandle, table FiniteTable) error {
	if err := m.open(); err != nil {
		return err
	}
	if table == nil {
		return ErrTable
	}
	_, expected, err := m.PredicateSignature(predicate)
	if err != nil {
		return err
	}
	actual, err := table.Signature()
	if err != nil {
		return err
	}
	if !expected.equal(actual) {
		return ErrTable
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
		if err = prior.Close(); err != nil {
			return err
		}
	} else if err = m.manager.IncRef(predicate); err != nil {
		return err
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

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
		// A relation whose signature no table encodes (a udoc relation
		// of a wide bit-vector) is still the predicate's relation.
		if _, relation := AsRelation(table); !relation || !errors.Is(err, ErrRelationNotTable) {
			return nil, false, err
		}
	}
	return table, true, nil
}

func (m *RelationManager) GetFiniteRelation(predicate ast.ASTHandle) (FiniteTable, error) {
	table, found, err := m.TryGetFiniteRelation(predicate)
	if err != nil {
		return nil, err
	}
	if found {
		return table, m.MaterializeFiniteRelation(predicate)
	}
	if relation, ok, cause := m.relationForPredicate(predicate); cause != nil {
		return nil, cause
	} else if ok {
		if err = m.StoreFiniteRelation(predicate, relation); err != nil {
			return nil, errors.Join(err, relation.Close())
		}
		return relation, nil
	}
	_, signature, err := m.PredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	table, err = m.newFavouriteFiniteTable(signature)
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
		return m.MaterializeFiniteRelation(predicate)
	}
	if prior != nil && m.placeholders[predicate] {
		// The source's store_relation creates the relation here.
		if err = releasePlaceholderTable(prior); err != nil {
			return err
		}
		delete(m.placeholders, predicate)
	} else if prior != nil {
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
	return m.noteRelation(predicate)
}

// checkStoredSignature is store_relation's SASSERT that the relation has
// the predicate's signature: the column sorts of a relation_base, the
// table signature of a table_relation's table.
func (m *RelationManager) checkStoredSignature(predicate ast.ASTHandle, table FiniteTable) error {
	if relation, ok := AsRelation(table); ok {
		sorts, err := m.predicateSorts(predicate)
		if err != nil {
			return err
		}
		actual := relation.RelationSignature()
		if len(actual) != len(sorts) {
			return ErrTable
		}
		for i := range sorts {
			if sorts[i] != actual[i] {
				return ErrTable
			}
		}
		return nil
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
	return nil
}

package rel

// Source: src/muz/rel/rel_context.cpp rel_context::has_facts. The source
// queries the predicate's actual relation_base regardless of its concrete
// table or relation representation.

import "github.com/glycerine/g3/pkg/ast"

// HasFacts reports whether an already materialized predicate relation is
// nonempty without creating storage or changing its selected representation.
func (m *RelationManager) HasFacts(predicate ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if m.placeholders[predicate] {
		// try_get_relation: get_relation has not created it.
		return false, nil
	}
	var table FiniteTable = m.finite[predicate]
	if table == nil {
		if hash := m.relations[predicate]; hash != nil {
			table = hash
		}
	}
	if table == nil {
		return false, nil
	}
	if r, ok := AsRelation(table); ok {
		// relation_base::empty()
		empty, err := r.Empty()
		return !empty, err
	}
	rows, err := table.Len()
	return rows != 0, err
}

// CollectNonEmptyPredicates is relation_manager::collect_non_empty_predicates:
// the predicates of m_relations, in its table order, whose relation is not
// fast_empty().
func (m *RelationManager) CollectNonEmptyPredicates() ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	var result []ast.ASTHandle
	for _, entry := range m.layout.Entries() {
		predicate := entry.Key
		if m.placeholders[predicate] {
			continue
		}
		nonEmpty := false
		{
			var table FiniteTable = m.finite[predicate]
			if table == nil {
				if hash := m.relations[predicate]; hash != nil {
					table = hash
				}
			}
			if table == nil {
				continue
			}
			if r, ok := AsRelation(table); ok {
				nonEmpty = !r.FastEmpty()
			} else {
				rows, err := table.Len()
				if err != nil {
					return nil, err
				}
				nonEmpty = rows != 0
			}
		}
		if nonEmpty {
			result = append(result, predicate)
		}
	}
	return result, nil
}

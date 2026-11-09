package rel

// Source: dl_relation_manager.cpp restrict_predicates (126-143), used by
// dl_context::scoped_query to remove query-local predicate relations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// RestrictPredicates retires relations outside keep and intersects saturation
// marks. Owners whose cleanup refuses remain attached for an explicit retry.
func (m *RelationManager) RestrictPredicates(keep map[ast.ASTHandle]bool) (result error) {
	if err := m.open(); err != nil {
		return err
	}
	// restrict_predicates: collect from m_relations in table order, then
	// deallocate, remove and dec_ref each.
	var toRemove []ast.ASTHandle
	for _, entry := range m.layout.Entries() {
		if !keep[entry.Key] {
			toRemove = append(toRemove, entry.Key)
		}
	}
	for _, predicate := range toRemove {
		// m_relations.remove(pred) needs the live predicate's hash.
		if err := m.layout.Erase(predicate); err != nil {
			result = errors.Join(result, err)
			continue
		}
		result = errors.Join(result, m.retireRelation(predicate))
		if m.holdsRelation(predicate) {
			result = errors.Join(result, m.noteRelation(predicate))
		}
	}
	for predicate := range m.saturated {
		if !keep[predicate] {
			delete(m.saturated, predicate)
		}
	}
	return result
}
